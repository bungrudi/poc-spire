#!/usr/bin/env python3
"""Approve only owned node serving CSRs and prove verified kubelet HTTPS."""
import base64
from contextlib import closing
import hashlib
import http.client
import json
import socket
import ssl
import sys
import time
from pathlib import Path

from cryptography import x509
from cryptography.x509.oid import NameOID
from cryptography.hazmat.primitives.asymmetric import ec, rsa
import yaml
import kubernetes_demo as runtime


def validate_csr(item, node, address):
    spec = item['spec']
    if spec['signerName'] != 'kubernetes.io/kubelet-serving':
        raise ValueError('unexpected CSR signer')
    if spec['username'] != 'system:node:' + node:
        raise ValueError('unexpected CSR requester')
    if set(spec['groups']) != {'system:nodes', 'system:authenticated'}:
        raise ValueError('unexpected CSR requester groups')
    csr = x509.load_pem_x509_csr(base64.b64decode(spec['request'], validate=True))
    usages = {'digital signature', 'server auth'}
    if isinstance(csr.public_key(), rsa.RSAPublicKey):
        usages.add('key encipherment')
    elif not isinstance(csr.public_key(), ec.EllipticCurvePublicKey):
        raise ValueError('unsupported serving key')
    if set(spec['usages']) != usages:
        raise ValueError('unexpected CSR usage')
    if not csr.is_signature_valid:
        raise ValueError('invalid CSR signature')
    expected = x509.Name([x509.NameAttribute(NameOID.ORGANIZATION_NAME, 'system:nodes'),
                         x509.NameAttribute(NameOID.COMMON_NAME, 'system:node:' + node)])
    if csr.subject != expected:
        raise ValueError('unexpected CSR subject')
    san = csr.extensions.get_extension_for_class(x509.SubjectAlternativeName).value
    if len(san) != 2 or any(not isinstance(name, (x509.DNSName, x509.IPAddress)) for name in san):
        raise ValueError('unexpected SAN type/cardinality')
    if san.get_values_for_type(x509.DNSName) != [node] or [str(v) for v in san.get_values_for_type(x509.IPAddress)] != [address]:
        raise ValueError('unexpected CSR SANs')
    if len(csr.extensions) != 1:
        raise ValueError('unexpected CSR extension')


def request(address, node, context, token):
    connection = socket.create_connection((address, 10250), timeout=4)
    wrapped = context.wrap_socket(connection, server_hostname=node)
    protocol = wrapped.version()
    with closing(http.client.HTTPConnection(address, 10250, timeout=4)) as client:
        client.sock = wrapped
        headers = {'Authorization': 'Bearer ' + token} if token else {}
        client.request('GET', '/pods', headers=headers)
        response = client.getresponse()
        data = response.read(2 * 1024 * 1024 + 1)
        if len(data) > 2 * 1024 * 1024:
            raise ValueError('bounded kubelet response exceeded')
        return response.status, protocol


def main():
    value = runtime.descriptor()
    node = value['node']
    work = Path(value['work'])
    info = runtime.docker_json('inspect', node)[0]
    address = info['NetworkSettings']['Networks'][value['network']]['IPAddress']
    ca = work / 'kubelet-ca.pem'
    ca.write_text(runtime.execute([runtime.DOCKER, 'exec', node, 'cat', '/etc/kubernetes/pki/ca.crt']))
    all_csrs = json.loads(runtime.kube(value, 'get', 'csr', '-o', 'json'))['items']
    approved = []
    validated = []
    for item in all_csrs:
        if item['spec']['signerName'] != 'kubernetes.io/kubelet-serving':
            continue
        validate_csr(item, node, address)
        validated.append(item['metadata']['name'])
        if not item.get('status', {}).get('conditions'):
            runtime.kube(value, 'certificate', 'approve', item['metadata']['name'])
            approved.append(item['metadata']['name'])
    namespace = 'spire-' + value['run']
    objects = [
        {'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': namespace, 'labels': {'istio-injection': 'disabled'}}},
        {'apiVersion': 'v1', 'kind': 'ServiceAccount', 'metadata': {'name': 'spire-agent', 'namespace': namespace}, 'automountServiceAccountToken': False},
        {'apiVersion': 'v1', 'kind': 'ServiceAccount', 'metadata': {'name': 'kubelet-denied', 'namespace': namespace}, 'automountServiceAccountToken': False},
        {'apiVersion': 'rbac.authorization.k8s.io/v1', 'kind': 'ClusterRole', 'metadata': {'name': value['cluster'] + '-kubelet'},
         'rules': [{'apiGroups': [''], 'resources': ['nodes/pods'], 'resourceNames': [node], 'verbs': ['get']}]},
        {'apiVersion': 'rbac.authorization.k8s.io/v1', 'kind': 'ClusterRoleBinding', 'metadata': {'name': value['cluster'] + '-kubelet'},
         'subjects': [{'kind': 'ServiceAccount', 'name': 'spire-agent', 'namespace': namespace}],
         'roleRef': {'apiGroup': 'rbac.authorization.k8s.io', 'kind': 'ClusterRole', 'name': value['cluster'] + '-kubelet'}}]
    runtime.kube(value, 'apply', '-f', '-', data=yaml.safe_dump_all(objects), timeout=60)
    token = runtime.kube(value, '-n', namespace, 'create', 'token', 'spire-agent', '--duration=10m').strip()
    context = ssl.create_default_context(cafile=str(ca))
    # Certificate reload is asynchronous; only retry verification while its own CSR is approved.
    for attempt in range(20):
        try:
            positive, protocol = request(address, node, context, token)
            break
        except ssl.SSLCertVerificationError:
            if attempt == 19:
                raise
            time.sleep(1)
    if positive != 200:
        raise RuntimeError('scoped kubelet /pods authorization did not succeed')
    denied_token = runtime.kube(value, '-n', namespace, 'create', 'token', 'kubelet-denied', '--duration=10m').strip()
    unauthorized, _ = request(address, node, context, denied_token)
    if unauthorized != 403:
        raise RuntimeError('valid unauthorized kubelet token was not denied')
    wrong_token, _ = request(address, node, context, 'invalid-probe-token')
    anonymous, _ = request(address, node, context, '')
    if wrong_token != 401 or anonymous != 401:
        raise RuntimeError('kubelet unauthenticated requests not denied')
    for label, ctx, hostname in [('wrong_ca', ssl.create_default_context(), node),
                                 ('wrong_hostname', context, 'unrelated.invalid')]:
        try:
            request(address, hostname, ctx, token)
        except ssl.SSLCertVerificationError:
            continue
        raise RuntimeError(label + ' unexpectedly succeeded')
    result = {'owned_serving_csrs_validated': len(validated), 'own_serving_csrs_approved': len(approved), 'validated_signer_subject_sans': True,
              'verified_kubelet_status': positive, 'kubelet_tls_protocol': protocol,
              'minimal_rbac': 'get nodes/pods for exact owned node', 'wrong_token_status': wrong_token,
              'anonymous_status': anonymous, 'valid_unauthorized_status': unauthorized, 'wrong_ca_denied': True, 'wrong_hostname_denied': True,
              'ca_sha256': hashlib.sha256(ca.read_bytes()).hexdigest(),
              'psat_pid_attestation': 'NOT RUN'}
    runtime.private_json(work / 'kubelet-result.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
