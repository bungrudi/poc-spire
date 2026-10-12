#!/usr/bin/env python3
"""Install the pinned sidecar mesh/CNI with its independent default Istiod CA."""
import hashlib
import json
import os
import subprocess
from pathlib import Path

import yaml
import kubernetes_demo as runtime


def operator():
    pins = json.loads((runtime.ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
    images = {i['reference']: i['immutable_reference'] for i in pins['images']}
    log_format = json.dumps({'start': '%START_TIME%', 'connection': '%CONNECTION_ID%',
                             'downstream_remote':'%DOWNSTREAM_REMOTE_ADDRESS%','downstream_local':'%DOWNSTREAM_LOCAL_ADDRESS%',
                             'upstream':'%UPSTREAM_HOST%','upstream_local':'%UPSTREAM_LOCAL_ADDRESS%',
                             'downstream_tls': '%DOWNSTREAM_TLS_VERSION%', 'upstream_tls': '%UPSTREAM_TLS_VERSION%',
                             'downstream_peer': '%DOWNSTREAM_PEER_URI_SAN%', 'upstream_peer': '%UPSTREAM_PEER_URI_SAN%',
                             'flags': '%RESPONSE_FLAGS%'}, separators=(',', ':')) + '\n'
    return {'apiVersion': 'install.istio.io/v1alpha1', 'kind': 'IstioOperator',
            'metadata': {'name': 'poc-baseline', 'namespace': 'istio-system'},
            'spec': {'profile': 'minimal', 'tag': '1.31.1',
                     'components': {'pilot': {'enabled': True, 'k8s': {'replicaCount': 1,
                                      'resources': {'requests': {'cpu': '100m', 'memory': '64Mi'},
                                                    'limits': {'cpu': '750m', 'memory': '192Mi'}}}},
                                    'cni': {'enabled': True, 'k8s': {
                                        'resources': {'requests': {'cpu': '25m', 'memory': '32Mi'},
                                                      'limits': {'cpu': '500m', 'memory': '96Mi'}}}}},
                     'meshConfig': {'meshMTLS': {'minProtocolVersion': 'TLSV1_3'},
                                    'accessLogFile': '/dev/stdout', 'accessLogFormat': log_format,
                                    'defaultConfig': {'holdApplicationUntilProxyStarts': True}},
                     'values': {'global': {'proxy': {'image': images['istio/proxyv2:1.31.1'],
                                                    'resources': {'requests': {'cpu': '25m', 'memory': '32Mi'},
                                                                  'limits': {'cpu': '500m', 'memory': '96Mi'}}}},
                                'pilot': {'image': images['istio/pilot:1.31.1'],
                                          'cni': {'enabled': True}, 'autoscaleEnabled': False},
                                'cni': {'image': images['istio/install-cni:1.31.1'],
                                        'chained': True, 'cniBinDir': '/opt/cni/bin',
                                        'cniConfDir': '/etc/cni/net.d', 'excludeNamespaces': ['istio-system', 'kube-system']}}}}


def main():
    value = runtime.descriptor()
    work = Path(value['work'])
    istio_root = runtime.TOOLS / 'istio-1.31.1'
    source = work / 'istio-operator.yaml'
    source.write_text(yaml.safe_dump(operator(), sort_keys=False))
    env = os.environ.copy()
    env['KUBECONFIG'] = str(work / 'kubeconfig')
    command = [str(istio_root / 'bin/istioctl'), 'install', '--kubeconfig', env['KUBECONFIG'],
               '--context', 'kind-' + value['cluster'], '-f', str(source), '-d', str(istio_root / 'manifests'),
               '--skip-confirmation', '--verify']
    result = subprocess.run(command, text=True, capture_output=True, env=env, timeout=300)
    diagnostics = runtime.RUNTIME / 'diagnostics'
    diagnostics.mkdir(mode=0o700, exist_ok=True)
    (diagnostics / (value['run'] + '-istio.txt')).write_text((result.stdout + result.stderr)[-65536:])
    if result.returncode:
        raise RuntimeError('Istio installation failed; bounded owned private diagnostics retained')
    runtime.kube(value, '-n', 'istio-system', 'rollout', 'status', 'deployment/istiod', '--timeout=180s', timeout=190)
    runtime.kube(value, '-n', 'istio-system', 'rollout', 'status', 'daemonset/istio-cni-node', '--timeout=180s', timeout=190)
    pods = json.loads(runtime.kube(value, '-n', 'istio-system', 'get', 'pods', '-o', 'json'))['items']
    observed = [{'pod': p['metadata']['name'], 'images': {c['name']: c['imageID'] for c in p['status'].get('containerStatuses', [])}}
                for p in pods]
    result = {'istio_version': '1.31.1', 'mesh_minimum_tls': 'TLSV1_3', 'default_istiod_ca': True,
              'sidecar_mode': True, 'chained_cni': True, 'operator_sha256': hashlib.sha256(source.read_bytes()).hexdigest(),
              'observed_system_images': observed, 'proxy_sds_and_job_probe': 'NOT RUN'}
    runtime.private_json(work / 'mesh-result.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
