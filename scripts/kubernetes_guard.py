#!/usr/bin/env python3
"""Owned setup-runner observation and monotonic quarantine; apps never receive API authority."""
import argparse
import contextlib
import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

import kubernetes_demo as runtime

SCRIPT = Path(__file__).resolve()


def get(value, namespace, kind, name=None):
    args = ['-n', namespace, 'get', kind]
    if name:
        args += [name, '--ignore-not-found']
    raw = runtime.kube(value, *args, '-o', 'json', timeout=8)
    return json.loads(raw) if raw.strip() else None


def process_stamp(pid):
    try:
        raw = Path('/proc') / str(pid)
        fields = (raw / 'stat').read_text().rsplit(')', 1)[1].split()
        if fields[0] in ('Z', 'X'):
            return None
        return {'start': fields[19], 'cmdline': (raw / 'cmdline').read_bytes().split(b'\0')[:-1]}
    except (OSError, IndexError):
        return None


def command(value):
    return [sys.executable, '-B', str(SCRIPT), 'monitor', '--run', value['run']]


def alive(value, record):
    stamp = process_stamp(record['pid'])
    return stamp is not None and stamp['start'] == record['start'] and stamp['cmdline'] == [x.encode() for x in command(value)]


def stop(value):
    marker = Path(value['work']) / 'guard-process.json'
    if not marker.exists():
        return
    record = json.loads(marker.read_text())
    if record['run'] != value['run']:
        raise RuntimeError('refuse foreign observer')
    if alive(value, record):
        os.kill(record['pid'], signal.SIGTERM)
        for _ in range(50):
            if not alive(value, record):
                break
            time.sleep(.1)
        else:
            raise RuntimeError('owned observer did not stop; ownership retained')
    marker.unlink()


def state_loss_termination(value, role, pod, status, expected_image):
    if role not in ('kafka', 'spire-server') or not expected_image or status.get('imageID') != expected_image:
        return False
    message = 'poc-spire:destructive-startup-state:' + role + ':' + value['run'] + ':' + pod['metadata']['uid']
    for state in (status.get('state', {}), status.get('lastState', {})):
        terminated = state.get('terminated', {})
        if terminated.get('exitCode') == 78 and terminated.get('message') == message:
            return True
    return False


def requesting_image(role, pod):
    container = 'server' if role == 'spire-server' else 'caller' if role.startswith('caller-') else role
    return next((s.get('imageID') for s in pod.get('status', {}).get('containerStatuses', []) if s['name'] == container), None)


def probe(value, role, pod, stream, running=False, expected_image=None):
    ns = ('spire-' if role == 'spire-server' else 'poc-') + value['run']
    container = 'server' if role == 'spire-server' else 'caller' if role.startswith('caller-') else 'kafka'
    status = next((s for s in pod.get('status', {}).get('containerStatuses', []) if s['name'] == container), {})
    if state_loss_termination(value, role, pod, status, expected_image):
        return {'valid': False}
    if not status.get('state', {}).get('running'):
        return None  # A stopped/restarting container is not evidence of destructive state loss.
    helper = '/guard-bin/kubernetes-init' if role == 'spire-server' else '/helpers/kubernetes-init' if role == 'kafka' else '/kubernetes-init'
    directory = '/run/spire/data' if role == 'spire-server' else '/var/lib/kafka/data' if role == 'kafka' else '/state/private'
    args = [runtime.TOOLS / 'kubectl', '--kubeconfig', Path(value['work']) / 'kubeconfig', '--context', 'kind-' + value['cluster'],
            '-n', ns, 'exec', pod['metadata']['name'], '-c', container, '--', helper, '-verify-state',
            '-role', role, '-state', directory, '-run', value['run'], '-stream', stream]
    if value.get('deployment_mode') == 'shared':
        from shared_cluster import checked_target
        target = checked_target(value['config'])
        args = [*target.base, *args[5:]]
    if role == 'kafka' and running:
        args.append('-require-topic')
    try:
        result = subprocess.run([str(a) for a in args], capture_output=True, timeout=5)
    except subprocess.TimeoutExpired:
        return None
    if len(result.stdout) > 256:
        return None
    try:
        data = json.loads(result.stdout)
    except ValueError:
        return None
    if result.returncode == 20 and data == {'valid': False}:
        return data
    if result.returncode == 0 and data.get('valid') is True and len(data.get('incarnation', '')) == 64:
        return data
    return None  # exec/API/runtime failure must not be classified as destructive state loss.


def validate_pods(expected, items):
    selected = {role: [p for p in items if p['metadata'].get('labels', {}).get('poc-role') == role]
                for role in expected['pods']}
    for role, matches in selected.items():
        if len(matches) != 1 or matches[0]['metadata']['uid'] != expected['pods'][role]:
            return 'original_application_pod_lost_or_replaced', {}
        refs = matches[0]['metadata'].get('ownerReferences', [])
        if len(refs) != 1 or refs[0]['kind'] != 'ReplicaSet' or refs[0]['uid'] != expected['owners'][role]:
            return 'original_application_owner_changed', {}
    return None, {role: matches[0] for role, matches in selected.items()}


def wait_for_observer(value, work, child):
    # A first full observation over the verified SSM EKS route can exceed the
    # local Kind startup window. Never treat elapsed time as successful health.
    for _ in range(240 if value.get('deployment_mode') == 'shared' else 40):
        health = work / 'guard-health.json'
        if health.exists():
            state = json.loads(health.read_text())
            if state.get('status') == 'observing':
                return
            if state.get('status') in ('invalid', 'quarantine-pending'):
                raise RuntimeError('run invalidated during observer startup')
        if child.poll() is not None:
            raise RuntimeError('owned observer exited')
        time.sleep(.25)
    raise RuntimeError('owned observer readiness unavailable')


def initial(value):
    work = Path(value['work'])
    if (work / 'guard-ownership.json').exists() or (work / 'guard-process.json').exists():
        raise RuntimeError('observer already initialized; refuse adoption or baseline replacement')
    owned = json.loads((work / 'application-ownership.json').read_text())
    issuer = json.loads((work / 'issuer-bootstrap-started.json').read_text())
    ns, sns = 'poc-' + value['run'], 'spire-' + value['run']
    original = get(value, sns, 'pod', 'spire-server')
    if not original or original['metadata']['uid'] != issuer['server_pod_uid']:
        raise RuntimeError('original issuer lost before observer initialization')
    reason, pods = validate_pods(owned, get(value, ns, 'pods')['items'])
    if reason:
        raise RuntimeError('original app ownership lost before observer initialization')
    baseline = {'run': value['run'], **owned, 'server_pod_uid': issuer['server_pod_uid'],
                'spire_namespace_uid': get(value, sns, 'namespace', sns)['metadata']['uid'],
                'guard_uids': {n: get(value, n, 'configmap', 'run-guard')['metadata']['uid'] for n in (ns, sns)},
                'bundle_uids': {n: get(value,n,'configmap','spire-bundle')['metadata']['uid'] for n in (ns,sns)}, 'states': {}, 'image_ids': {}}
    for role in ('caller-trusted', 'caller-unidentified', 'kafka', 'spire-server'):
        for _ in range(30):
            p = get(value, sns, 'pod', 'spire-server') if role == 'spire-server' else get(value, ns, 'pod', pods[role]['metadata']['name'])
            result = probe(value, role, p, owned['stream']) if p else {'valid': False}
            if result is not None:
                break
            time.sleep(1)
        if not result or not result.get('valid'):
            raise RuntimeError('initialized state not proved before observer startup')
        baseline['states'][role] = result['incarnation']
        image = requesting_image(role, p)
        if not image or '@sha256:' not in image:
            raise RuntimeError('immutable initialized requesting-container image unavailable')
        baseline['image_ids'][role] = image
    runtime.private_json(work / 'guard-ownership.json', baseline)
    child = subprocess.Popen(command(value), cwd=runtime.ROOT, stdin=subprocess.DEVNULL,
                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
    stamp = process_stamp(child.pid)
    if stamp is None:
        raise RuntimeError('owned observer failed to start')
    runtime.private_json(work / 'guard-process.json', {'run': value['run'], 'pid': child.pid, 'start': stamp['start']})
    wait_for_observer(value, work, child)


def promote(value):
    work = Path(value['work'])
    baseline = json.loads((work / 'guard-ownership.json').read_text())
    process = json.loads((work / 'guard-process.json').read_text())
    if (work / 'run-invalid.json').exists() or not alive(value, process):
        raise RuntimeError('invalid or unobserved run cannot be promoted')
    cm = get(value, 'poc-' + value['run'], 'configmap', 'run-guard')
    if cm['metadata']['uid'] != baseline['guard_uids']['poc-' + value['run']]:
        raise RuntimeError('original guard lost before promotion')
    snapshot = json.loads(cm['data']['run.json'])
    if snapshot['run'] != value['run'] or snapshot['phase'] != 'bootstrap':
        raise RuntimeError('run promotion refused')
    snapshot['phase'] = 'running'
    cm['data']['run.json'] = json.dumps(snapshot)
    runtime.kube(value, 'replace', '-f', '-', data=json.dumps(cm), timeout=8)


def invalidate(value, baseline, reason):
    work = Path(value['work']); marker = work / 'run-invalid.json'
    if not marker.exists():
        runtime.private_json(marker, {'run': value['run'], 'reason': reason, 'detected_at': time.time()})
    # Once recorded, retry quarantine; never promote the run even if lost resources reappear.
    for ns, uid in baseline['guard_uids'].items():
        cm = get(value, ns, 'configmap', 'run-guard')
        if not cm or cm['metadata']['uid'] != uid:
            raise RuntimeError('original guard absent; refuse foreign guard adoption')
        snapshot = json.loads(cm['data']['run.json'])
        if snapshot['run'] != value['run']:
            raise RuntimeError('foreign run guard')
        if snapshot['phase'] != 'invalid':
            snapshot['phase'] = 'invalid'
            cm['data']['run.json'] = json.dumps(snapshot)
            # Exact resourceVersion/UID preconditions: no write over a replaced or concurrently changed CM.
            runtime.kube(value, 'replace', '-f', '-', data=json.dumps(cm), timeout=8)
    # Updating only an owned Pod annotation prompts kubelet to refresh its projected guard.
    for ns in ('poc-' + value['run'], 'spire-' + value['run']):
        for pod in get(value, ns, 'pods')['items']:
            expected = baseline['server_pod_uid'] if ns.startswith('spire-') and pod['metadata']['name'] == 'spire-server' else baseline['pods'].get(pod['metadata'].get('labels', {}).get('poc-role'))
            if expected != pod['metadata']['uid']:
                continue
            patch = {'metadata': {'uid': expected, 'resourceVersion': pod['metadata']['resourceVersion'],
                                 'annotations': {'poc-spire.guard/invalid': value['run']}}}
            runtime.kube(value, '-n', ns, 'patch', 'pod', pod['metadata']['name'], '--type=merge', '-p', json.dumps(patch), timeout=8)


def observe(value, baseline):
    ns, sns = 'poc-' + value['run'], 'spire-' + value['run']
    read=get
    shared=value.get('deployment_mode')=='shared'
    if shared:
        from shared_cluster import parallel_reads
        queries=[(ns,'namespace',ns),(sns,'namespace',sns),(ns,'pods',None),(sns,'pod','spire-server')]
        queries.extend((n,'configmap','run-guard') for n in baseline['guard_uids'])
        queries.extend((n,'configmap','spire-bundle') for n in baseline['bundle_uids'])
        snapshots=dict(zip(queries,parallel_reads(lambda q:get(value,*q),queries,deferred=True)))
        read=lambda value,namespace,kind,name=None:snapshots[(namespace,kind,name)].result()
    for name, uid in ((ns, baseline['namespace_uid']), (sns, baseline['spire_namespace_uid'])):
        namespace = read(value, name, 'namespace', name)
        if not namespace or namespace['metadata']['uid'] != uid:
            return 'original_namespace_lost_or_replaced'
    reason, pods = validate_pods(baseline, read(value, ns, 'pods')['items'])
    if reason:
        return reason
    server = read(value, sns, 'pod', 'spire-server')
    if not server or server['metadata']['uid'] != baseline['server_pod_uid']:
        return 'original_issuer_pod_lost_or_replaced'
    snapshot = None
    for namespace, uid in baseline['guard_uids'].items():
        cm = read(value, namespace, 'configmap', 'run-guard')
        if not cm or cm['metadata']['uid'] != uid:
            return 'original_guard_lost_or_replaced'
        try:
            current = json.loads(cm['data']['run.json'])
        except (ValueError, KeyError):
            return 'original_guard_corrupt'
        if current.get('run') != value['run'] or current.get('phase') not in ('bootstrap', 'running', 'invalid'):
            return 'original_guard_corrupt'
        if current['phase'] == 'invalid':
            return 'run_already_invalid'
        if namespace == ns:
            snapshot = current
    for namespace, uid in baseline['bundle_uids'].items():
        bundle = read(value, namespace, 'configmap', 'spire-bundle')
        if not bundle or bundle['metadata']['uid'] != uid:
            return 'original_public_trust_projection_lost_or_replaced'
    def check_state(role):
        p = server if role == 'spire-server' else pods[role]
        actual_image = requesting_image(role, p)
        expected_image = baseline['image_ids'][role]
        if actual_image and actual_image != expected_image:
            return 'original_' + role + '_requesting_image_changed'
        result = probe(value, role, p, baseline['stream'], snapshot['phase'] == 'running', expected_image)
        if result is None:
            return None
        if not result['valid'] or result['incarnation'] != baseline['states'][role]:
            return 'initialized_' + role + '_state_lost_corrupt_or_replaced'
        return None
    results=parallel_reads(check_state,baseline['states']) if shared else map(check_state,baseline['states'])
    for reason in results:
        if reason:return reason
    return None


def observation_scope(value):
    if (value and value.get('deployment_mode')=='shared') or (value is None and os.environ.get('POC_SPIRE_SHARED_CONFIG')):
        from shared_cluster import checked_scope
        return checked_scope()
    return contextlib.nullcontext()


def monitor(value, expected_run=None):
    baseline=None
    while True:
        with observation_scope(value):
            current=runtime.descriptor()
            if value is None:
                value=current
                if value['run']!=expected_run:raise RuntimeError('refuse foreign observer run')
            elif current!=value:return
            if baseline is None:
                work=Path(value['work']);baseline=json.loads((work/'guard-ownership.json').read_text())
                if baseline['run']!=value['run']:raise RuntimeError('foreign observer baseline')
            try:
                marker = work / 'run-invalid.json'
                reason = json.loads(marker.read_text())['reason'] if marker.exists() else observe(value, baseline)
                if reason:
                    invalidate(value, baseline, reason)
                runtime.private_json(work / 'guard-health.json', {'run': value['run'], 'status': 'invalid' if reason else 'observing', 'at': time.time()})
            except (RuntimeError, OSError, ValueError, KeyError, subprocess.TimeoutExpired):
                runtime.private_json(work / 'guard-health.json', {'run': value['run'], 'status': 'quarantine-pending' if (work / 'run-invalid.json').exists() else 'observation-unavailable', 'at': time.time()})
        time.sleep(2)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('start', 'monitor', 'stop'))
    parser.add_argument('--run', required=True)
    args = parser.parse_args()
    if args.action=='monitor' and os.environ.get('POC_SPIRE_SHARED_CONFIG'):
        monitor(None,expected_run=args.run)
    else:
        value=runtime.descriptor()
        if value['run']!=args.run:raise RuntimeError('refuse foreign observer run')
        {'start': initial, 'monitor': monitor, 'stop': stop}[args.action](value)
