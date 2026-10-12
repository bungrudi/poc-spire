#!/usr/bin/env python3
"""Install the pinned VXLAN/iptables CNI only into the owned baseline cluster."""
import hashlib
import json
from pathlib import Path

import yaml
import kubernetes_demo as runtime


def render():
    pins = json.loads((runtime.ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
    raw = (runtime.ROOT / '.cache/kubernetes-baseline/artifacts/calico.yaml').read_bytes()
    if 'sha256:' + hashlib.sha256(raw).hexdigest() != pins['upstream_manifests']['calico']['sha256']:
        raise RuntimeError('Calico source manifest checksum mismatch')
    images = {item['reference']: item['immutable_reference'] for item in pins['images']}
    objects = list(yaml.safe_load_all(raw))
    for obj in objects:
        if obj.get('kind') == 'ConfigMap' and obj['metadata']['name'] == 'calico-config':
            obj['data']['calico_backend'] = 'vxlan'
        if obj.get('kind') not in ('DaemonSet', 'Deployment'):
            continue
        template = obj['spec']['template']
        template['metadata'].setdefault('annotations', {})['sidecar.istio.io/inject'] = 'false'
        spec = template['spec']
        if obj['metadata']['name'] == 'calico-node':
            spec['initContainers'] = [c for c in spec['initContainers'] if c['name'] != 'ebpf-bootstrap']
        for container in spec.get('initContainers', []) + spec.get('containers', []):
            container['image'] = images[container['image']]
            container['resources'] = {'requests': {'cpu': '25m', 'memory': '32Mi'},
                                      'limits': {'cpu': '500m', 'memory': '128Mi'}}
            if container['name'] == 'calico-node':
                container['resources'] = {'requests': {'cpu': '100m', 'memory': '128Mi'},
                                          'limits': {'cpu': '750m', 'memory': '384Mi'}}
                env = container['env']
                updates = {'CALICO_IPV4POOL_IPIP': 'Never', 'CALICO_IPV4POOL_VXLAN': 'Always',
                           'CLUSTER_TYPE': 'k8s', 'CALICO_IPV4POOL_CIDR': runtime.POD_CIDR,
                           'FELIX_BPFENABLED': 'false', 'FELIX_IPTABLESBACKEND': 'Auto'}
                for key, value in updates.items():
                    env[:] = [item for item in env if item['name'] != key]
                    env.append({'name': key, 'value': value})
                for probe, bird in [('readinessProbe', '--bird-ready'), ('livenessProbe', '--bird-live')]:
                    container[probe]['exec']['command'].remove(bird)
    return yaml.safe_dump_all(objects, sort_keys=False)


def main():
    value = runtime.descriptor()
    body = render()
    work = Path(value['work'])
    (work / 'calico.yaml').write_text(body)
    runtime.kube(value, 'apply', '--server-side', '-f', '-', data=body, timeout=120)
    runtime.kube(value, '-n', 'kube-system', 'rollout', 'status', 'daemonset/calico-node', '--timeout=180s', timeout=190)
    runtime.kube(value, '-n', 'kube-system', 'rollout', 'status', 'deployment/calico-kube-controllers', '--timeout=180s', timeout=190)
    runtime.kube(value, 'wait', '--for=condition=Ready', 'node/' + value['node'], '--timeout=120s', timeout=130)
    node = json.loads(runtime.kube(value, 'get', 'node', value['node'], '-o', 'json'))
    result = {'calico_ready': True, 'rendered_sha256': hashlib.sha256(body.encode()).hexdigest(),
              'node_uid': node['metadata']['uid'], 'capacity': node['status']['capacity'],
              'allocatable': node['status']['allocatable'],
              'container_runtime': node['status']['nodeInfo']['containerRuntimeVersion'],
              'network_policy_negative': 'NOT RUN'}
    runtime.private_json(work / 'network-result.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
