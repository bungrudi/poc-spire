#!/usr/bin/env python3
"""Verify deployed fake transfers; cookies/CSRF values stay solely in memory."""
import http.cookiejar
import json
import os
import ssl
import time
import urllib.error
import urllib.request
import uuid

from attestation import ROOT


def main():
    origin = 'https://spire.adianto.dev'
    jar = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}),
        urllib.request.HTTPSHandler(context=ssl.create_default_context()),
        urllib.request.HTTPCookieProcessor(jar))
    csrf = ''
    report = {'origin': origin, 'status': 'failed', 'human_identity': 'assumed, not authenticated', 'tests': []}

    def request(method, path, body=None, extra=None):
        headers = {'Origin': origin, 'X-CSRF-Token': csrf}
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            headers['Content-Type'] = 'application/json'
        headers.update(extra or {})
        req = urllib.request.Request(origin+path, data=data, method=method, headers=headers)
        try:
            response = opener.open(req, timeout=15)
        except urllib.error.HTTPError as exc:
            response = exc
        with response:
            raw = response.read(65537)
            if len(raw) > 65536:
                raise RuntimeError('Oversized browser response')
            try:
                value = json.loads(raw)
            except json.JSONDecodeError:
                value = None
            return response.status, response.headers, raw, value

    def balances():
        status, _, _, value = request('GET', '/api/balances')
        if status != 200 or not value.get('server_verified') or value.get('expected_server_id') != 'spiffe://poc.internal/money':
            raise RuntimeError('Verified Money balances unavailable')
        return value

    def attempt(caller, key):
        status, _, _, value = request('POST', '/api/attempts', {'caller': caller, 'browser_key': key})
        if status != 202:
            raise RuntimeError('Deployed browser attempt was rejected')
        deadline = time.monotonic()+20
        while value['status'] == 'pending' and time.monotonic() < deadline:
            time.sleep(0.3)
            status, _, _, value = request('GET', '/api/attempts/'+value['attempt_id'])
            if status != 200:
                raise RuntimeError('Deployed result polling failed')
        if value['status'] != 'done':
            raise RuntimeError('No completed measured caller outcome')
        return value

    try:
        status, headers, page, _ = request('GET', '/')
        if status != 200 or b'operator identity is assumed, not authenticated' not in page:
            raise RuntimeError('Demo page unavailable or human assumption not disclosed')
        cookies = list(jar)
        if len(cookies) != 1 or not cookies[0].secure or not cookies[0].has_nonstandard_attr('HttpOnly'):
            raise RuntimeError('Unsafe browser session')
        if headers.get('Cache-Control') != 'no-store' or 'frame-ancestors' not in headers.get('Content-Security-Policy', ''):
            raise RuntimeError('Missing browser defenses')
        status, _, _, session = request('GET', '/api/session')
        if status != 200 or 'assumed' not in session.get('auth_profile', ''):
            raise RuntimeError('Unexpected demo session')
        csrf = session['csrf_token']
        initial = balances()
        body = {'caller': 'trusted', 'browser_key': uuid.uuid4().hex}
        for extra in [{'X-CSRF-Token': ''}, {'Origin': 'https://foreign.example'}, {'Sec-Fetch-Site': 'cross-site'}]:
            if request('POST', '/api/attempts', body, extra)[0] != 403:
                raise RuntimeError('Public CSRF/origin boundary failed')
        if balances() != initial:
            raise RuntimeError('Rejected browser requests changed balances')
        report['tests'].append({'case': 'public-HTTPS-disclosure-session-CSRF', 'passed': True})
        key = uuid.uuid4().hex
        first = attempt('trusted', key)
        evidence = first['caller_evidence']
        after = balances()
        if evidence['outcome'] != 'transfer_succeeded' or evidence['money_http_status'] != 200 or not evidence['server_verified'] or evidence['money_result']['verified_caller_id'] != 'spiffe://poc.internal/caller/trusted':
            raise RuntimeError('Trusted workload did not produce verified transfer evidence')
        if after['version'] != initial['version']+1 or after['balances']['demo-a'] != initial['balances']['demo-a']-100 or after['balances']['demo-b'] != initial['balances']['demo-b']+100:
            raise RuntimeError('Exactly one USD 1.00 transfer was not observed')
        if attempt('trusted', key)['attempt_id'] != first['attempt_id'] or balances() != after:
            raise RuntimeError('Public browser retry transferred twice')
        report['tests'].append({'case': 'public-real-trusted-transfer-stable-retry', 'passed': True})
        rejected = attempt('unidentified', uuid.uuid4().hex)['caller_evidence']
        if rejected['outcome'] != 'tls_rejected' or rejected['identity_eligibility'] != 'no_matching_identity' or not rejected['server_verified'] or 'money_http_status' in rejected or balances() != after:
            raise RuntimeError('Unidentified result was not verified no-SVID/TLS rejection without mutation')
        report['tests'].append({'case': 'public-real-unidentified-TLS-rejection', 'passed': True})
        report['final_fake_ledger'] = {'balances': after['balances'], 'version': after['version']}
        if request('POST', '/auth/logout')[0] != 200 or request('GET', '/api/session')[0] != 401:
            raise RuntimeError('Session clearing failed')
        report['status'] = 'passed'
        print('Public HTTPS, trusted transfer/retry, unidentified TLS rejection and unchanged rejected balances: passed')
    except Exception as exc:
        report['failure'] = str(exc)
        print('Public demo verification failed: '+str(exc))
    finally:
        jar.clear()
        report['verified_at_utc'] = time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())
        (ROOT/'docs/evidence/public-demo.json').write_text(json.dumps(report, indent=2)+'\n')
    return 0 if report['status'] == 'passed' else 1


if __name__ == '__main__':
    raise SystemExit(main())
