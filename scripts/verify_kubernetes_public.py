#!/usr/bin/env python3
"""Verify the named public Kubernetes demo; retain no cookie/CSRF/key material."""
import http.client
import json
import ssl
import sys
import time
import uuid
from pathlib import Path

import kubernetes_demo as runtime
from kubernetes_public import owned_public, ORIGIN
from kafka_demo import KafkaStack


class Browser:
    attempt = KafkaStack.attempt

    def __init__(self, stream):
        self.stream = stream
        self.url = ORIGIN
        self.cookie = None
        self.csrf = None
        self.context = ssl.create_default_context()

    def request(self, method, path, body=None, headers=None):
        connection = http.client.HTTPSConnection('spire.adianto.dev', context=self.context, timeout=10)
        values = dict(headers or {})
        if self.cookie:
            values['Cookie'] = self.cookie
        if body is not None and not isinstance(body, (str, bytes)):
            body = json.dumps(body)
            values['Content-Type'] = 'application/json'
        if method == 'POST':
            values.setdefault('Origin', self.url)
            values.setdefault('X-CSRF-Token', self.csrf or '')
        try:
            connection.request(method, path, body, values)
            response = connection.getresponse()
            data = response.read(65537)
            if len(data) > 65536:
                raise RuntimeError('bounded UI response exceeded')
            if response.getheader('Set-Cookie'):
                self.cookie = response.getheader('Set-Cookie').split(';', 1)[0]
            return response.status, data
        finally:
            connection.close()

    def json(self, method, path, body=None, headers=None):
        code, data = self.request(method, path, body, headers)
        return code, json.loads(data)


def main():
    value, _ = owned_public()
    work = Path(value['work'])
    ownership = json.loads((work / 'application-ownership.json').read_text())
    browser = Browser(ownership['stream'])
    code, page = browser.request('GET', '/')
    if code != 200 or b'data-ready="true"' not in page or b'Kafka' not in page:
        raise RuntimeError('public Kubernetes five-scenario UI not ready')
    code, session = browser.json('GET', '/api/session')
    if code != 200 or 'assumed' not in session['auth_profile']:
        raise RuntimeError('expected assumed-operator profile unavailable')
    browser.csrf = session['csrf_token']
    code, initial = browser.json('GET', '/api/balances')
    assert code == 200 and initial['server_verified']
    baseline_file = work / 'public-validation-initial.json'
    if baseline_file.exists():
        initial = json.loads(baseline_file.read_text())
    else:
        runtime.private_json(baseline_file, initial)
    result = {'source_image_revision': json.loads((work / 'normal-image.json').read_text())['source_revision'],
              'run': value['run'], 'cluster': value['cluster'], 'origin': ORIGIN,
              'initial_balances': initial, 'scenarios': [], 'browser_denials': []}
    for scenario, expected in [('trusted', 'transfer_succeeded'), ('unidentified', 'tls_rejected'),
                               ('kafka-trusted', 'transfer_succeeded'), ('kafka-unidentified', 'money_rejected'),
                               ('kafka-tampered', 'money_rejected')]:
        # Each new logical case has its own persisted key; never replace it on retry.
        key_path = work / ('public-key-' + scenario + '.json')
        key = json.loads(key_path.read_text())['browser_key'] if key_path.exists() else uuid.uuid4().hex
        if not key_path.exists():runtime.private_json(key_path, {'browser_key': key})
        evidence, _ = browser.attempt(scenario, key)
        details = evidence.get('caller_evidence', evidence.get('kafka_evidence', {}))
        assert details['outcome'] == expected, scenario
        result['scenarios'].append({'scenario': scenario, 'status': evidence['status'], 'evidence': details})
        print('Public scenario PASS:', scenario, flush=True)
    code, final = browser.json('GET', '/api/balances')
    assert code == 200 and final['server_verified'] and final['version'] == initial['version'] + 2
    assert final['balances']['demo-a'] == initial['balances']['demo-a'] - 200
    assert final['balances']['demo-b'] == initial['balances']['demo-b'] + 200
    result['final_balances'] = final
    body = {'caller': 'trusted', 'browser_key': uuid.uuid4().hex}
    for name, headers in [('missing-CSRF', {'X-CSRF-Token': ''}),
                          ('wrong-Origin', {'Origin': 'https://wrong.invalid'}),
                          ('wrong-Host', {'Host': 'wrong.invalid'})]:
        code, _ = browser.request('POST', '/api/attempts', body, headers)
        if name == 'wrong-Host':
            result['public_wrong_Host_status'] = code
            assert code >= 400, (name, code)
            # An unmatched edge virtual host can fail routing. Prove the Go
            # boundary independently over the authenticated owned TLS endpoint.
            import socket
            direct = http.client.HTTPConnection('spire.adianto.dev', timeout=8)
            direct.sock = browser.context.wrap_socket(socket.create_connection(('127.0.0.1',18089),timeout=8), server_hostname='spire.adianto.dev')
            values={'Host':'wrong.invalid','Cookie':browser.cookie,'Origin':ORIGIN,'X-CSRF-Token':browser.csrf,'Content-Type':'application/json'}
            direct.request('POST','/api/attempts',json.dumps(body),values)
            response=direct.getresponse();response.read(65537);code=response.status;direct.close()
            result['Host_proof_mode']='Go UI through verified loopback hostname/chain; public edge routing failure is recorded separately'
        assert code == 403, (name, code)
        result['browser_denials'].append({'case': name, 'status': code})
    code, unchanged = browser.json('GET', '/api/balances')
    assert code == 200 and unchanged == final
    namespace = 'poc-' + value['run']
    pods = json.loads(runtime.kube(value, '-n', namespace, 'get', 'pods', '-l', 'poc-role=ui', '-o', 'json'))['items']
    assert len(pods) == 1 and pods[0]['metadata']['uid'] == ownership['pods']['ui']
    image = json.loads((work / 'normal-image.json').read_text())
    status = next(c for c in pods[0]['status']['containerStatuses'] if c['name'] == 'ui')
    container_image = next(c['image'] for c in pods[0]['spec']['containers'] if c['name']=='ui')
    assert container_image == image['immutable_reference']
    observed_digest = status['imageID'].rsplit('@',1)[-1]
    assert observed_digest == image['index_digest']
    index = json.loads(runtime.execute(['docker','exec',value['node'],'ctr','-n','k8s.io','content','get',observed_digest]))
    assert [x['digest'] for x in index['manifests']] == [image['platform_manifest_digest']]
    container = next(c for c in pods[0]['spec']['containers'] if c['name'] == 'ui')
    assert container['args'][container['args'].index('-origin') + 1] == ORIGIN
    result['actual_UI'] = {'pod_uid': pods[0]['metadata']['uid'], 'image_id': status['imageID'],
                           'normal_binary_sha256': image['binary_sha256']['ui'], 'requested_platform_image': container_image,
                           'actual_status_index_digest': observed_digest, 'index_platform_conjunction_verified': True, 'exact_public_origin': True,
                           'namespace': namespace, 'all_containers_ready': all(c['ready'] for c in pods[0]['status']['containerStatuses'])}
    runtime.private_json(work / 'public-acceptance.json', result)
    print('Public HTTPS defense/ledger/UI provenance PASS:', final['version'])


if __name__ == '__main__':
    main()
