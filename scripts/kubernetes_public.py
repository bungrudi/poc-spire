#!/usr/bin/env python3
"""Named public profile and owned loopback route; no cluster repair or key export."""
import argparse
import base64
import json
import os
from pathlib import Path
import signal
import ssl
import stat
import subprocess
import time

import yaml
import kubernetes_demo as runtime

ORIGIN = 'https://spire.adianto.dev'
MAIN = Path('/home/ubuntu/Workspace/poc-spire')


def provisioned_profile():
    if runtime.ROOT != MAIN:
        raise RuntimeError('public profile must use the main checkout')
    pem = MAIN / '.runtime/browser/browser.pem'
    info = pem.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != MAIN.stat().st_uid or info.st_mode & 0o077:
        raise RuntimeError('owner-only provisioned browser PEM required')
    if pem.parent.stat().st_mode & 0o077:
        raise RuntimeError('owner-only browser directory required')
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(str(pem))
    cert = ssl._ssl._test_decode_cert(str(pem))
    if ('DNS', 'spire.adianto.dev') not in cert.get('subjectAltName', ()):
        raise RuntimeError('exact public browser hostname required')
    if not ssl.cert_time_to_seconds(cert['notBefore']) <= time.time() < ssl.cert_time_to_seconds(cert['notAfter']):
        raise RuntimeError('current browser certificate required')
    return {'origin': ORIGIN, 'browser_pem': str(pem), 'checkout': str(MAIN)}


def owned_public():
    profile = provisioned_profile()
    value = runtime.descriptor()
    marker = Path(value['work']) / 'public-profile.json'
    if not marker.is_file() or json.loads(marker.read_text()) != profile:
        raise RuntimeError('owned public profile unavailable or different')
    return value, profile


def renew():
    if not runtime.ACTIVE.exists():
        return
    value, profile = owned_public()
    obj = {'apiVersion': 'v1', 'kind': 'Secret',
           'metadata': {'name': 'browser-pki', 'namespace': 'poc-' + value['run']},
           'type': 'Opaque', 'data': {'browser.pem': base64.b64encode(Path(profile['browser_pem']).read_bytes()).decode()}}
    runtime.kube(value, 'apply', '-f', '-', data=yaml.safe_dump(obj), timeout=30)
    print('Owned public UI browser Secret updated; sessions and Money preserved.')


def serve():
    value, _ = owned_public()
    runtime.kube(value, '-n', 'poc-' + value['run'], 'get', 'service', 'ui', timeout=15)
    command = [str(runtime.TOOLS / 'kubectl'), '--kubeconfig', str(Path(value['work']) / 'kubeconfig'),
               '--context', 'kind-' + value['cluster'], '-n', 'poc-' + value['run'],
               'port-forward', 'service/ui', '18089:8443', '--address', '127.0.0.1']
    process = subprocess.Popen(command, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    stopping = False

    def stop(*_):
        nonlocal stopping
        stopping = True
        process.terminate()

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    print('Named public route forwarding the exact owned UI on literal loopback.', flush=True)
    result = process.wait()
    if not stopping:
        raise RuntimeError('owned route exited; supervision may restart the route, not repair the cluster')
    return result


if __name__ == '__main__':
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('serve', 'renew'))
    args = parser.parse_args()
    try:
        serve() if args.action == 'serve' else renew()
    except Exception:
        raise SystemExit('public profile/route operation failed; inspect owned private state') from None
