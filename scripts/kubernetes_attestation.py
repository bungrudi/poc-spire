#!/usr/bin/env python3
"""Run the early actual caller executable/Kubernetes gate; final adapter/migration proof remains separate."""
import argparse
import json
import time
from pathlib import Path

import yaml
import kubernetes_demo as runtime
from attestation import spiffe_ids


def admin(value, *args):
    return runtime.kube(value, '-n', 'spire-' + value['run'], 'exec', 'spire-server', '-c', 'server', '--',
                        '/opt/spire/bin/spire-server', *args)


def pod(value, image, name, sa, role, command, adapter=False):
    security = {'runAsUser': 1000, 'runAsGroup': 1000, 'allowPrivilegeEscalation': False,
                'readOnlyRootFilesystem': True, 'capabilities': {'drop': ['ALL']},
                'seccompProfile': {'type': 'RuntimeDefault'}}
    application = {'name': 'caller' if role != 'money' else 'money', 'image': image,
                   'imagePullPolicy': 'Never', 'command': command, 'securityContext': security,
                   'resources': {'requests': {'cpu': '25m', 'memory': '16Mi'}, 'limits': {'cpu': '500m', 'memory': '96Mi'}},
                   'volumeMounts': [{'name': 'workload', 'mountPath': '/workload', 'readOnly': True}]}
    containers = [application]
    volumes = [{'name': 'workload', 'csi': {'driver': 'csi.spiffe.io', 'readOnly': True}}]
    if adapter:
        application['volumeMounts'].append({'name': 'control', 'mountPath': '/control'})
        containers.append({'name': 'adapter', 'image': image, 'imagePullPolicy': 'Never',
                           'command': ['/security-client', '-idle'],
                           'securityContext': {**security, 'runAsUser': 10002},
                           'resources': {'requests': {'cpu': '25m', 'memory': '16Mi'}, 'limits': {'cpu': '500m', 'memory': '64Mi'}},
                           'volumeMounts': [{'name': 'workload', 'mountPath': '/workload', 'readOnly': True},
                                            {'name': 'control', 'mountPath': '/control'}]})
        volumes.append({'name': 'control', 'emptyDir': {'sizeLimit': '1Mi'}})
    return {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name, 'namespace': 'poc-' + value['run'],
             'labels': {'poc-run': value['run'], 'poc-role': role},
             'annotations': {'sidecar.istio.io/nativeSidecar': 'false'}},
            'spec': {'serviceAccountName': sa, 'automountServiceAccountToken': False,
                     'securityContext': {'fsGroup': 1000, 'seccompProfile': {'type': 'RuntimeDefault'}},
                     'containers': containers, 'volumes': volumes}}


def observed(value, name, container):
    for attempt in range(120):
        item = json.loads(runtime.kube(value, '-n', 'poc-' + value['run'], 'get', 'pod', name, '-o', 'json'))
        status = next((c for c in item.get('status', {}).get('containerStatuses', []) if c['name'] == container and c.get('imageID')), None)
        if status:
            if '@sha256:' not in status['imageID']:
                raise RuntimeError('actual immutable requesting-container ImageID missing')
            return item, status['imageID']
        time.sleep(1)
    raise RuntimeError('actual container metadata unavailable')


def register(value, parent, sid, item, container, image_id, binary_hash, uid):
    selectors = ['unix:uid:' + str(uid), 'unix:sha256:' + binary_hash,
                 'k8s:ns:' + item['metadata']['namespace'], 'k8s:sa:' + item['spec']['serviceAccountName'],
                 'k8s:container-name:' + container, 'k8s:container-image:' + image_id,
                 'k8s:pod-uid:' + item['metadata']['uid'],
                 'k8s:pod-label:poc-run:' + value['run'], 'k8s:pod-label:poc-role:' + item['metadata']['labels']['poc-role']]
    owners=item['metadata'].get('ownerReferences',[])
    if owners:
        if len(owners)!=1 or not owners[0].get('controller') or owners[0]['kind']!='ReplicaSet':raise RuntimeError('unexpected requesting-Pod owner')
        selectors.append('k8s:pod-owner-uid:'+owners[0]['kind']+':'+owners[0]['uid'])
    entries = json.loads(admin(value, 'entry', 'show', '-socketPath', '/run/spire/admin/api.sock', '-output', 'json')).get('entries', [])
    matching = [entry for entry in entries if sid in spiffe_ids(entry.get('spiffe_id', {}))]
    if matching:
        if len(matching) != 1:
            raise RuntimeError('duplicate grant rejected')
        entry = matching[0]
        actual = sorted(s['type'] + ':' + s['value'] for s in entry['selectors'])
        if actual != sorted(selectors) or set(spiffe_ids(entry.get('parent_id', {}))) != {parent} or entry.get('x509_svid_ttl') != 90:
            raise RuntimeError('broader/different parent, selector or TTL grant rejected')
        return
    args = ['entry', 'create', '-socketPath', '/run/spire/admin/api.sock', '-parentID', parent,
            '-spiffeID', sid, '-x509SVIDTTL', '90', '-output', 'json']
    for selector in selectors:
        args += ['-selector', selector]
    admin(value, *args)


def main(resume=False):
    value = runtime.descriptor()
    work = Path(value['work'])
    marker = work / 'early-attestation-started.json'
    if marker.exists() and not resume:
        raise RuntimeError('early gate already started; refuse silent grant/workload replacement')
    if resume:
        if not marker.exists() or (work / 'early-attestation-result.json').exists():
            raise RuntimeError('only unfinished gate may resume')
    else:
        runtime.private_json(marker, {'run': value['run'], 'purpose': 'development actual-caller gate'})
    image = json.loads((work / 'feasibility-image.json').read_text())
    ns = 'poc-' + value['run']
    objects = [{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': ns, 'labels': {'istio-injection': 'enabled'}}}]
    for sa in ('money', 'caller-trusted', 'caller-unidentified', 'probe-wrong'):
        objects.append({'apiVersion': 'v1', 'kind': 'ServiceAccount', 'metadata': {'name': sa, 'namespace': ns}, 'automountServiceAccountToken': False})
    objects += [pod(value, image['immutable_reference'], 'money', 'money', 'money', ['/money', '-listen', ':8443']),
                pod(value, image['immutable_reference'], 'trusted', 'caller-trusted', 'trusted', ['/trusted-caller', '-operator-uid', '10002'], True),
                pod(value, image['immutable_reference'], 'wrong-deployment', 'probe-wrong', 'wrong-deployment', ['/security-client', '-idle'])]
    objects.append({'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': 'money', 'namespace': ns},
                    'spec': {'selector': {'poc-role': 'money'}, 'ports': [{'name': 'tcp-money', 'port': 8443, 'targetPort': 8443}]}})
    # Temporary gate policies allow the wrong-deployment reachability control explicitly.
    objects += [{'apiVersion': 'security.istio.io/v1', 'kind': 'PeerAuthentication', 'metadata': {'name': 'strict', 'namespace': ns}, 'spec': {'mtls': {'mode': 'STRICT'}}},
                {'apiVersion': 'networking.istio.io/v1', 'kind': 'DestinationRule', 'metadata': {'name': 'money', 'namespace': ns},
                 'spec': {'host': 'money.' + ns + '.svc.cluster.local', 'trafficPolicy': {'tls': {'mode': 'ISTIO_MUTUAL'}}}}]
    if not resume:
        runtime.apply_csi_workloads(value, '-', data=yaml.safe_dump_all(objects), timeout=120)
        pods = json.loads(runtime.kube(value, '-n', ns, 'get', 'pods', '-o', 'json'))['items']
        runtime.private_json(marker, {'run': value['run'], 'pods': {p['metadata']['name']: p['metadata']['uid'] for p in pods}})
    else:
        old = json.loads(marker.read_text())
        if 'pods' not in old:
            raise RuntimeError('missing immutable initial Pod ownership; explicit gate audit required')
        pods = json.loads(runtime.kube(value, '-n', ns, 'get', 'pods', '-o', 'json'))['items']
        if old['pods'] != {p['metadata']['name']: p['metadata']['uid'] for p in pods}:
            raise RuntimeError('gate Pod replacement; explicit fresh run required')
    (work / 'apps-mounted').write_text('early gate CSI workloads\n')
    agents = json.loads(admin(value, 'agent', 'list', '-socketPath', '/run/spire/admin/api.sock', '-output', 'json'))['agents']
    node = json.loads(runtime.kube(value, 'get', 'node', value['node'], '-o', 'json'))
    expected_parent = 'spiffe://poc.internal/spire/agent/k8s_psat/' + value['cluster'] + '/' + node['metadata']['uid']
    parents = [sid for agent in agents for sid in spiffe_ids(agent.get('id', {}))]
    if parents != [expected_parent]:
        raise RuntimeError('actual PSAT parent/node UID mismatch')
    for name, container, sid, binary, uid in [
        ('money', 'money', 'spiffe://poc.internal/money', 'money', 1000),
        ('trusted', 'caller', 'spiffe://poc.internal/caller/trusted', 'trusted-caller', 1000),
        ('trusted', 'adapter', 'spiffe://poc.internal/control/caller/trusted', 'security-client', 10002)]:
        item, image_id = observed(value, name, container)
        register(value, expected_parent, sid, item, container, image_id, image['binary_sha256'][binary], uid)
    runtime.kube(value, '-n', ns, 'wait', '--for=condition=Ready', 'pod/money', 'pod/trusted', 'pod/wrong-deployment', '--timeout=180s', timeout=190)
    control = json.loads(runtime.kube(value, '-n', ns, 'exec', 'trusted', '-c', 'adapter', '--', '/control-client', '-socket', '/control/api.sock', '-id', 'k8s-early-trusted'))
    if control.get('money_http_status') != 200:
        raise RuntimeError('actual attested trusted caller did not complete direct Money mTLS')
    denied = []
    for pod_name, container in [('trusted', 'caller'), ('wrong-deployment', 'caller')]:
        raw = runtime.kube(value, '-n', ns, 'exec', pod_name, '-c', container, '--', '/attestation-ineligible', '-target', 'money:8443', '-expect-denied')
        data = json.loads(raw)
        if data.get('result') != 'no_matching_identity' or data.get('grpc_code') != 'PermissionDenied' or not data.get('socket_reachable') or not data.get('network_reachable'):
            raise RuntimeError('reachable explicit executable/deployment denial missing')
        denied.append(data)
    adapter = json.loads(runtime.kube(value, '-n', ns, 'exec', 'trusted', '-c', 'adapter', '--', '/security-client', '-status', '403', '-id', 'k8s-early-control-denied'))
    if adapter.get('money_http_status') != 403 or not adapter.get('server_verified'):
        raise RuntimeError('control-only current SVID/Money policy proof missing')
    result = {'actual_trusted_caller_mtls': control, 'uid10002_control_identity_money403': adapter,
              'denials': denied, 'parent': expected_parent, 'hold_application_until_proxy_starts': True,
              'final_actual_adapter_binary_and_full_migration': 'NOT RUN',
              'note': 'early development gate; wrong-deployment approved-binary and network-policy/layer evidence still required'}
    runtime.private_json(work / 'early-attestation-result.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--resume', action='store_true')
    main(parser.parse_args().resume)
