#!/usr/bin/env python3
"""Bootstrap guarded SPIRE and CSI only in the existing owned disposable cluster."""
import hashlib
import json
from pathlib import Path

import yaml
import kubernetes_demo as runtime


def rendered(value, filename):
    pins = json.loads((runtime.ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
    images = {i['reference']: i['immutable_reference'] for i in pins['images']}
    bootstrap = json.loads((Path(value['work']) / 'bootstrap-image.json').read_text())['tag']
    substitutions = {'SPIRE_NS': 'spire-' + value['run'], 'CLUSTER': value['cluster'], 'RUN': value['run'],
                     'NODE': value['node'], 'BOOTSTRAP_IMAGE': bootstrap,
                     'SPIRE_SERVER_IMAGE': images['ghcr.io/spiffe/spire-server:1.14.0'],
                     'SPIRE_AGENT_IMAGE': images['ghcr.io/spiffe/spire-agent:1.14.0']}
    body = (runtime.ROOT / 'infra/kubernetes-baseline' / filename).read_text()
    for name, replacement in substitutions.items():
        body = body.replace('@' + name + '@', replacement)
    if '@SPIRE_' in body or '@RUN@' in body:
        raise RuntimeError('unresolved SPIRE template')
    return body


def csi(value):
    pins = json.loads((runtime.ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
    images = {i['reference']: i['immutable_reference'] for i in pins['images']}
    raw = (runtime.ROOT / '.cache/kubernetes-baseline/artifacts/spiffe-csi-example.yaml').read_bytes()
    if 'sha256:' + hashlib.sha256(raw).hexdigest() != pins['upstream_manifests']['spiffe-csi-example']['sha256']:
        raise RuntimeError('CSI example checksum mismatch')
    objects = list(yaml.safe_load_all(raw))
    for obj in objects:
        if 'namespace' in obj['metadata']:
            obj['metadata']['namespace'] = 'spire-' + value['run']
        if obj['kind'] != 'DaemonSet':
            continue
        template = obj['spec']['template']
        template['metadata']['namespace'] = 'spire-' + value['run']
        template['metadata'].setdefault('annotations', {})['sidecar.istio.io/inject'] = 'false'
        spec = template['spec']
        spec['automountServiceAccountToken'] = False
        for container in spec['containers']:
            reference = ('ghcr.io/spiffe/spiffe-csi-driver:0.2.13' if container['name'] == 'spiffe-csi-driver'
                         else 'registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.6.0')
            container['image'] = images[reference]
            container['resources'] = {'requests': {'cpu': '10m', 'memory': '16Mi'}, 'limits': {'cpu': '250m', 'memory': '64Mi'}}
        for volume in spec['volumes']:
            if volume['name'] == 'spire-agent-socket-dir':
                volume['hostPath']['path'] = '/run/poc-spire/' + value['run'] + '/agent-sockets'
    return yaml.safe_dump_all(objects, sort_keys=False)


def ensure_fresh_issuer(value):
    marker = Path(value['work']) / 'issuer-bootstrap-started.json'
    if marker.exists() or marker.is_symlink():
        raise RuntimeError('issuer initialization already started; explicit fresh run required')
    return marker


def main():
    value = runtime.descriptor()
    work = Path(value['work'])
    namespace = 'spire-' + value['run']
    marker = ensure_fresh_issuer(value)
    server = rendered(value, 'spire-server.yaml.tmpl')
    existing = json.loads(runtime.kube(value, '-n', namespace, 'get', 'pods', '-o', 'json'))['items']
    if any(p['metadata']['name'] == 'spire-server' for p in existing):
        raise RuntimeError('refuse issuer adoption/recreation; use owned explicit fresh run')
    runtime.private_json(marker, {'run': value['run'], 'namespace': namespace, 'phase': 'initialization-started'})
    runtime.kube(value, 'apply', '-f', '-', data=server, timeout=90)
    pod = json.loads(runtime.kube(value, '-n', namespace, 'get', 'pod', 'spire-server', '-o', 'json'))
    guard = {'run': value['run'], 'phase': 'bootstrap', 'pods': {'spire-server': pod['metadata']['uid']}}
    runtime.kube(value, '-n', namespace, 'patch', 'configmap', 'run-guard', '--type=merge', '-p', json.dumps({'data': {'run.json': json.dumps(guard)}}))
    runtime.kube(value, '-n', namespace, 'wait', '--for=condition=Ready', 'pod/spire-server', '--timeout=180s', timeout=190)
    bundle = runtime.kube(value, '-n', namespace, 'exec', 'spire-server', '-c', 'server', '--',
                          '/opt/spire/bin/spire-server', 'bundle', 'show', '-socketPath', '/run/spire/admin/api.sock')
    if '-----BEGIN CERTIFICATE-----' not in bundle or 'PRIVATE KEY' in bundle:
        raise RuntimeError('authentic public bundle unavailable')
    # Initial bounded private administration; recurring restricted publisher is a later gate.
    cm = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'spire-bundle', 'namespace': namespace}, 'data': {'bundle.pem': bundle}}
    runtime.kube(value, 'apply', '-f', '-', data=yaml.safe_dump(cm))
    runtime.private_json(marker, {'run': value['run'], 'namespace': namespace, 'phase': 'initialized', 'server_pod_uid': pod['metadata']['uid']})
    guard['phase'] = 'running'
    runtime.kube(value, '-n', namespace, 'patch', 'configmap', 'run-guard', '--type=merge', '-p', json.dumps({'data': {'run.json': json.dumps(guard)}}))
    agent = rendered(value, 'spire-agent.yaml.tmpl')
    runtime.kube(value, 'apply', '-f', '-', data=agent, timeout=90)
    runtime.kube(value, 'apply', '-f', '-', data=csi(value), timeout=90)
    runtime.kube(value, '-n', namespace, 'rollout', 'status', 'daemonset/spire-agent', '--timeout=180s', timeout=190)
    runtime.kube(value, '-n', namespace, 'rollout', 'status', 'daemonset/spiffe-csi-driver', '--timeout=180s', timeout=190)
    result = {'spire': '1.14.0', 'server_pod_uid': pod['metadata']['uid'], 'psat_node_attestation': True,
              'verified_kubelet': True, 'agent_extra_capability': 'SYS_PTRACE', 'csi': '0.2.13',
              'real_executable_and_socket_gate': 'NOT RUN', 'restricted_recurring_publisher': 'NOT RUN'}
    runtime.private_json(work / 'spire-result.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
