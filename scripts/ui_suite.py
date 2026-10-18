#!/usr/bin/env python3
"""Real-SPIRE browser-to-caller suite on loopback with ephemeral browser TLS."""
import json
import os
from pathlib import Path
import shutil
import socket
import tempfile
import time
import uuid

from attestation import ROOT, run, wait_for
from remote_demo import RemoteStack


def main():
    os.umask(0o077)
    fixture = Path(tempfile.mkdtemp(prefix='spire-ui-tls-'))
    stack = None
    report = {'phase': 'assumed-operator-HTTPS-demo', 'status': 'failed', 'tests': [],
              'public_deployment': False, 'human_authentication': 'explicitly omitted by owner'}
    try:
        run(['openssl', 'req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256',
             '-nodes', '-keyout', str(fixture/'key.pem'), '-out', str(fixture/'cert.pem'),
             '-days', '1', '-subj', '/CN=demo.test', '-addext', 'subjectAltName=DNS:demo.test'])
        (fixture/'browser.pem').write_bytes((fixture/'cert.pem').read_bytes()+(fixture/'key.pem').read_bytes())
        (fixture/'browser.pem').chmod(0o600)
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        stack = RemoteStack(fixture, port, 'https://demo.test', fixture/'cert.pem')
        print('Building isolated HTTPS UI and real-SPIRE caller stack', flush=True)
        stack.start()
        cookie, csrf = '', ''

        def request(method, path, body=None, extra=None, authenticated=True):
            headers = {}
            if authenticated and cookie:
                headers['Cookie'] = cookie
            if method == 'POST':
                headers.update({'Origin': stack.url, 'X-CSRF-Token': csrf, 'Content-Type': 'application/json'})
            headers.update(extra or {})
            encoded = json.dumps(body).encode() if body is not None else None
            status, response_headers, data = stack.request(method, path, encoded, headers)
            try:
                value = json.loads(data)
            except json.JSONDecodeError:
                value = None
            return status, response_headers, data, value

        def session():
            nonlocal cookie, csrf
            status, headers, page, _ = request('GET', '/', authenticated=False)
            if status != 200 or b'operator identity is assumed, not authenticated' not in page:
                raise RuntimeError('Assumed identity was not disclosed')
            raw_cookie = headers.get('Set-Cookie', '')
            if 'Secure' not in raw_cookie or 'HttpOnly' not in raw_cookie or 'SameSite=Lax' not in raw_cookie:
                raise RuntimeError('Unsafe demo session cookie')
            cookie = raw_cookie.split(';')[0]
            status, _, _, value = request('GET', '/api/session')
            if status != 200 or value.get('role') != 'operator' or 'assumed' not in value.get('auth_profile', ''):
                raise RuntimeError('Unexpected assumed session')
            csrf = value['csrf_token']

        def balances():
            status, _, _, value = request('GET', '/api/balances')
            if status != 200 or not value.get('server_verified'):
                raise RuntimeError('UI cannot read verified Money balances')
            return value

        def attempt(caller, key):
            status, _, _, value = request('POST', '/api/attempts', {'caller': caller, 'browser_key': key})
            if status != 202:
                raise RuntimeError('Browser attempt rejected')
            deadline = time.monotonic()+16
            while value['status'] == 'pending' and time.monotonic() < deadline:
                time.sleep(0.25)
                status, _, _, value = request('GET', '/api/attempts/'+value['attempt_id'])
                if status != 200:
                    raise RuntimeError('Attempt polling rejected')
            if value['status'] != 'done':
                raise RuntimeError('Caller result was not a completed measured outcome')
            return value

        body = {'caller': 'trusted', 'browser_key': uuid.uuid4().hex}
        if request('POST', '/api/attempts', body, authenticated=False)[0] != 401:
            raise RuntimeError('Direct API request bypassed CSRF session')
        if request('POST', '/api/attempts', body, {'X-Role': 'operator', 'X-User': 'fake'}, False)[0] != 401:
            raise RuntimeError('Fabricated human headers accepted')
        session()
        initial = balances()
        for headers in [{'X-CSRF-Token': ''}, {'Origin': 'https://foreign.test'},
                        {'Host': 'foreign.test', 'X-Forwarded-Host': 'demo.test'}, {'Sec-Fetch-Site': 'cross-site'}]:
            if request('POST', '/api/attempts', body, headers)[0] != 403:
                raise RuntimeError('CSRF/origin/Host boundary failed')
        if balances() != initial:
            raise RuntimeError('Rejected browser request changed ledger')
        report['tests'].append({'case': 'assumption-disclosure-session-CSRF-origin-Host', 'passed': True})
        key = uuid.uuid4().hex
        trusted = attempt('trusted', key)
        evidence = trusted['caller_evidence']
        after = balances()
        if evidence['outcome'] != 'transfer_succeeded' or evidence['money_http_status'] != 200 or not evidence['server_verified']:
            raise RuntimeError('Trusted caller did not transfer over verified TLS')
        if evidence['money_result']['verified_caller_id'] != 'spiffe://poc.internal/caller/trusted':
            raise RuntimeError('Money did not verify exact trusted identity')
        if after['version'] != initial['version']+1 or after['balances']['demo-a'] != initial['balances']['demo-a']-100 or after['balances']['demo-b'] != initial['balances']['demo-b']+100:
            raise RuntimeError('Expected fixed atomic transfer not observed')
        if attempt('trusted', key)['attempt_id'] != trusted['attempt_id'] or balances() != after:
            raise RuntimeError('Repeated browser key transferred twice')
        report['tests'].append({'case': 'real-trusted-transfer-browser-repeat', 'passed': True})
        run(stack.compose+['restart', 'ui'], env=stack.env)
        def restarted():
            try:
                return stack.request('GET', '/healthz')[0] == 200
            except OSError:
                return False
        wait_for(restarted, 'Restarted UI', seconds=20)
        session()
        retry = attempt('trusted', key)
        if retry['attempt_id'] != trusted['attempt_id'] or not retry['caller_evidence']['money_result']['replay'] or balances() != after:
            raise RuntimeError('UI restart/session renewal changed stable request ID')
        report['tests'].append({'case': 'UI-restart-new-session-same-ID-no-duplicate', 'passed': True})
        unidentified = attempt('unidentified', uuid.uuid4().hex)['caller_evidence']
        if unidentified['outcome'] != 'signature_rejected' or unidentified['identity_eligibility'] != 'no_matching_identity' or unidentified.get('money_http_status') != 401 or not unidentified['server_verified'] or balances() != after:
            raise RuntimeError('Unidentified path did not prove no-SVID signature rejection without mutation')
        report['tests'].append({'case': 'real-unidentified-no-SVID-signature-rejection', 'passed': True})
        blocked = run(stack.compose+['exec', '-T', 'untrusted-caller', '/control-client', '-socket', '/trusted-control/api.sock', '-id', 'isolation-test'], env=stack.env, check=False)
        if blocked.returncode == 0 or json.loads(blocked.stdout).get('outcome') != 'control_unreachable':
            raise RuntimeError('Unidentified caller reached trusted control volume')
        report['tests'].append({'case': 'cross-caller-control-isolation', 'passed': True})
        report['runtime'] = {}
        for service in ['money', 'trusted-caller', 'untrusted-caller', 'ui']:
            cid = run(stack.compose+['ps', '-q', service], env=stack.env).stdout.strip()
            info = json.loads(run(['docker', 'inspect', cid]).stdout)[0]
            mounts = [m['Destination'] for m in info['Mounts']]
            bindings = info['HostConfig']['PortBindings'] or {}
            if info['Config']['User'] != '1000:1000' or not info['HostConfig']['ReadonlyRootfs'] or info['HostConfig']['CapDrop'] != ['ALL']:
                raise RuntimeError('Application container hardening failed')
            if any('docker.sock' in m or 'server' in m for m in mounts):
                raise RuntimeError('Application received infrastructure/admin mount')
            if service != 'ui' and bindings:
                raise RuntimeError('Backend/caller port published')
            if any(v['HostIp'] != '127.0.0.1' for values in bindings.values() for v in values):
                raise RuntimeError('Browser listener escaped loopback')
            report['runtime'][service] = {'uid': 1000, 'mounts': mounts, 'published_loopback_only': True}
        if not stack.healthy():
            raise RuntimeError('Runtime supervision check failed')
        report['status'] = 'passed'
        print('Real-SPIRE HTTPS UI, transfer/rejection, retry across UI restart and isolation: passed', flush=True)
    except Exception as exc:
        report['failure'] = str(exc)
        print('HTTPS UI suite failed: '+str(exc), flush=True)
    finally:
        if stack:
            report['cleanup'] = stack.close()
            if any(report['cleanup'][key] for key in ['compose_exit_code', 'image_exit_code']) or not report['cleanup']['ephemeral_state_removed'] or not report['cleanup']['retained_identity_unchanged']:
                report['status'] = 'failed'
        shutil.rmtree(fixture)
        report['temporary_browser_keys_removed'] = not fixture.exists()
        (ROOT/'docs/evidence/ui-demo.json').write_text(json.dumps(report, indent=2)+'\n')
    return 0 if report['status'] == 'passed' else 1


if __name__ == '__main__':
    raise SystemExit(main())
