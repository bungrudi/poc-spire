#!/usr/bin/env python3
"""Install checksum-verified tools into the ignored project cache only."""
import hashlib
import json
import os
import tarfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEST = ROOT / '.cache/kubernetes-baseline/tools'


def main():
    pins = json.loads((ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
    if pins['status'] != 'resolved' or pins['resolution_failures']:
        raise SystemExit('unresolved artifact catalog')
    DEST.mkdir(parents=True, exist_ok=True)
    for name, pin in pins['tools'].items():
        filename = 'istio-1.31.1-linux-amd64.tar.gz' if name == 'istio' else name
        target = DEST / filename
        expected = pin['sha256'].removeprefix('sha256:')
        if not target.exists() or hashlib.sha256(target.read_bytes()).hexdigest() != expected:
            temporary = target.with_suffix(target.suffix + '.download')
            digest = hashlib.sha256()
            total = 0
            request = urllib.request.Request(pin['url'], headers={'User-Agent': 'poc-spire-tools'})
            try:
                with urllib.request.urlopen(request, timeout=60) as response, temporary.open('wb') as output:
                    while data := response.read(1024 * 1024):
                        total += len(data)
                        if total > 256 * 1024 * 1024:
                            raise ValueError('tool exceeds download limit')
                        output.write(data)
                        digest.update(data)
                if digest.hexdigest() != expected:
                    raise ValueError('published tool checksum mismatch')
                os.replace(temporary, target)
            finally:
                temporary.unlink(missing_ok=True)
        if name != 'istio':
            target.chmod(0o755)
        else:
            with tarfile.open(target) as archive:
                archive.extractall(DEST, filter='data')
        print('Verified project-local tool:', name, pin['release_tag'], flush=True)


if __name__ == '__main__':
    main()
