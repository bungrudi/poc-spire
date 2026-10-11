#!/usr/bin/env python3
"""Loopback HTTPS stack; no host proxy, certificates or services are installed."""
import argparse
import http.client
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import ssl
import time
from urllib.parse import urlparse

from attestation import ROOT, run, wait_for
from identity import IdentityRuntime
from local_demo import LocalStack


class RemoteStack(LocalStack):
    def __init__(self, browser_dir, port=18087, origin='https://spire.adianto.dev', ca_file=None):
        parsed = urlparse(origin)
        if parsed.scheme != 'https' or not parsed.hostname or parsed.path or parsed.query or parsed.fragment or parsed.username or parsed.port:
            raise ValueError('An exact HTTPS origin on port 443 is required')
        browser_dir = Path(browser_dir).resolve()
        pem = browser_dir / 'browser.pem'
        info = pem.lstat()
        if not pem.is_file() or pem.is_symlink() or info.st_uid != os.getuid() or info.st_mode & 0o077:
            raise ValueError('Owner-only provisioned browser PEM required')
        super().__init__(port)
        self.metadata = ROOT / '.runtime/remote-demo.json'
        self.compose = ['docker', 'compose', '-f', str(ROOT/'infra/remote-demo/compose.yaml')]
        self.url = origin
        self.hostname = parsed.hostname
        self.context = ssl.create_default_context(cafile=str(ca_file) if ca_file else None)
        self.env.update(POC_ORIGIN=origin, POC_BROWSER_DIR=str(browser_dir))

    def request(self, method, path, body=None, headers=None):
        connection = http.client.HTTPConnection(self.hostname, timeout=8)
        raw = socket.create_connection(('127.0.0.1', self.port), timeout=8)
        connection.sock = self.context.wrap_socket(raw, server_hostname=self.hostname)
        try:
            connection.request(method, path, body=body, headers=headers or {})
            response = connection.getresponse()
            data = response.read(65537)
            if len(data) > 65536:
                raise RuntimeError('Oversized browser response')
            return response.status, dict(response.getheaders()), data
        finally:
            connection.close()

    def start(self):
        self.runtime = IdentityRuntime(ephemeral_root=self.work)
        if self.metadata.exists():
            raise RuntimeError('Another demo descriptor exists; inspect its owner before starting')
        for name in ['money', 'trusted-caller', 'untrusted-caller', 'ui', 'security-client', 'control-client']:
            run(['go', 'build', '-trimpath', '-o', str(self.work/name), './cmd/'+name], env=self.env)
            (self.work/name).chmod(0o555)
        shutil.copy(ROOT/'infra/local-ui/Dockerfile', self.work/'Dockerfile')
        run(['docker', 'build', '--network', 'none', '-t', self.image, str(self.work)])
        self.built = True
        image_digest = run(['docker', 'image', 'inspect', self.image, '--format', '{{.Id}}']).stdout.strip()
        self.runtime.start()
        for identity, binary, role in [
            ('spiffe://poc.internal/money', 'money', 'demo-money'),
            ('spiffe://poc.internal/caller/trusted', 'trusted-caller', 'demo-trusted'),
            ('spiffe://poc.internal/ui', 'ui', 'demo-ui')]:
            self.runtime.register(identity, self.work/binary, image_digest, role, ttl=90)
        self.up = True
        run(self.compose+['up', '-d'], env=self.env)

        def ready():
            try:
                return self.request('GET', '/healthz')[0] == 200
            except (OSError, ValueError):
                return False
        wait_for(ready, 'HTTPS demo listener', seconds=40)
        for directory in ['trusted-control', 'untrusted-control']:
            wait_for(lambda: (self.work/directory/'api.sock').is_socket(), 'Private caller socket', seconds=30)
        info = {'project': self.project, 'work': str(self.work), 'origin': self.url,
                'pid': os.getpid(), 'ephemeral': True, 'human_identity': 'assumed demo operator'}
        self.metadata.write_text(json.dumps(info))
        self.metadata.chmod(0o600)
        return self

    def healthy(self):
        if any(p.poll() is not None for p in self.runtime.processes):
            return False
        data = run(self.compose+['ps', '--all', '--format', 'json'], env=self.env).stdout
        rows = [json.loads(line) for line in data.splitlines() if line.strip()]
        return len(rows) == 4 and all(row.get('State') == 'running' for row in rows)


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('--browser-dir', type=Path, default=ROOT/'.runtime/browser')
    parser.add_argument('--port', type=int, default=18087)
    args = parser.parse_args()
    stack = RemoteStack(args.browser_dir, args.port)
    def stop(*_):
        raise KeyboardInterrupt()
    signal.signal(signal.SIGTERM, stop)
    try:
        stack.start()
        print('Loopback HTTPS SPIRE demo ready; operator is assumed, not authenticated', flush=True)
        while True:
            time.sleep(5)
            if not stack.healthy():
                raise RuntimeError('Demo process stopped; inspect protected diagnostics')
    except KeyboardInterrupt:
        pass
    finally:
        cleanup = stack.close()
        if not cleanup['retained_identity_unchanged'] or cleanup['compose_exit_code'] or cleanup['image_exit_code']:
            raise RuntimeError('Demo cleanup requires inspection')


if __name__ == '__main__':
    main()
