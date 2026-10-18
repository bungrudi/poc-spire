"""SPIRE SDS wiring and exact observed proxy grants; no registration controller."""
import json
from pathlib import Path

import kubernetes_demo as runtime

TRUST_DOMAIN = 'poc.internal'
SOCKET_FILE = 'api.sock'


def configure(template):
    """Override Istio's injected emptyDir with CSI for ordinary/native sidecars."""
    annotations = template['metadata'].setdefault('annotations', {})
    config = json.loads(annotations.get('proxy.istio.io/config', '{}'))
    config.setdefault('proxyMetadata', {})['WORKLOAD_IDENTITY_SOCKET_FILE'] = SOCKET_FILE
    annotations['proxy.istio.io/config'] = json.dumps(config, separators=(',', ':'))
    volumes = template['spec'].setdefault('volumes', [])
    if any(v['name'] == 'workload-socket' for v in volumes):
        raise RuntimeError('refuse conflicting proxy credential volume')
    # Istio 1.31.1 already mounts this volume at /var/run/secrets/workload-spiffe-uds.
    volumes.append({'name': 'workload-socket', 'csi': {'driver': 'csi.spiffe.io', 'readOnly': True}})


def register_proxy(value, parent, name):
    from kubernetes_attestation import observed, ensure_entry
    item, image_id = observed(value, name, 'istio-proxy')
    spec = item['spec']
    proxies = [c for c in spec.get('containers', []) + spec.get('initContainers', []) if c['name'] == 'istio-proxy']
    pins = json.loads((runtime.ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
    pinned = next(i for i in pins['images'] if i['reference'] == 'istio/proxyv2:1.31.1')
    proxy = proxies[0] if len(proxies) == 1 else {}
    if proxy.get('image') != pinned['immutable_reference'] or proxy.get('securityContext', {}).get('runAsUser') != 1337:
        raise RuntimeError('unexpected proxy image or UID')
    if not any(image_id.endswith(d) for d in (pinned['index_digest'], pinned['platform_manifest_digest'], pinned['config_digest'])):
        raise RuntimeError('observed proxy image differs from pin')
    if spec.get('nodeName') != value['node'] or item['metadata']['labels'].get('poc-run') != value['run']:
        raise RuntimeError('proxy node/run differs')
    selectors = ['unix:uid:1337', 'k8s:ns:' + item['metadata']['namespace'],
                 'k8s:sa:' + spec['serviceAccountName'], 'k8s:container-name:istio-proxy',
                 'k8s:container-image:' + image_id, 'k8s:pod-uid:' + item['metadata']['uid'],
                 'k8s:pod-label:poc-run:' + value['run'],
                 'k8s:pod-label:poc-role:' + item['metadata']['labels']['poc-role']]
    sid = 'spiffe://' + TRUST_DOMAIN + '/ns/' + item['metadata']['namespace'] + '/sa/' + spec['serviceAccountName']
    ensure_entry(value, parent, sid, selectors)
    return item


def require_shared_mesh(target):
    """Read platform state; never rewrite a shared Istio control plane."""
    import yaml
    raw = target.kube('-n', 'istio-system', 'get', 'configmap', 'istio', '-o', 'json')
    mesh = yaml.safe_load(json.loads(raw)['data']['mesh'])
    if mesh.get('trustDomain') != TRUST_DOMAIN:
        raise RuntimeError('shared mesh must use SPIRE trust domain poc.internal; platform configuration required')
    # Per-Pod CSI+explicit custom socket enforces SPIRE issuance, without changing
    # global proxy metadata or disabling the platform CA for unrelated workloads.
