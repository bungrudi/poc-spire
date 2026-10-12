#!/usr/bin/env python3
"""Resolve the owner-reviewed Kubernetes pins without installing or deploying."""
import argparse
import concurrent.futures
import hashlib
import json
import re
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CACHE = ROOT / '.cache/kubernetes-baseline/artifacts'
OUTPUT = ROOT / 'infra/kubernetes-baseline/artifacts.json'
APPROVED = '68351e7bbe655544c95ba9d845e1045cecf5aa6c'
NODE = 'kindest/node:v1.35.8@sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0'
IMAGES = [NODE, 'quay.io/calico/calico:v3.33.0',
          'quay.io/calico/third-party-cni-plugins:v3.33.0',
          'quay.io/calico/node:v3.33.0', 'istio/pilot:1.31.1',
          'istio/proxyv2:1.31.1', 'istio/install-cni:1.31.1',
          'ghcr.io/spiffe/spire-server:1.14.0',
          'ghcr.io/spiffe/spire-agent:1.14.0',
          'ghcr.io/spiffe/spiffe-csi-driver:0.2.13',
          'registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.6.0',
          'apache/kafka:4.0.2@sha256:836cafdad9f4825880d7cf1d5a21202915ae2527bd0ef1c3600c526ed7814d1f']
ACCEPT = ', '.join(('application/vnd.oci.image.index.v1+json',
                   'application/vnd.docker.distribution.manifest.list.v2+json',
                   'application/vnd.oci.image.manifest.v1+json',
                   'application/vnd.docker.distribution.manifest.v2+json'))


def sha(data):
    return 'sha256:' + hashlib.sha256(data).hexdigest()


def fetch(url, headers=None, max_bytes=8 * 1024 * 1024):
    req = urllib.request.Request(url, headers={'User-Agent': 'poc-spire-pin-resolver', **(headers or {})})
    with urllib.request.urlopen(req, timeout=60) as response:
        data = response.read(max_bytes + 1)
        if len(data) > max_bytes:
            raise ValueError('artifact exceeds download bound')
        return data, dict(response.headers)


def resolve_image(reference):
    first = reference.split('/', 1)[0]
    if '.' in first or ':' in first:
        registry, repo_ref = reference.split('/', 1)
    else:
        registry, repo_ref = 'registry-1.docker.io', reference
    if '@' in repo_ref:
        repo_tag, selector = repo_ref.split('@', 1)
        repo = repo_tag.rsplit(':', 1)[0]
    else:
        repo, selector = repo_ref.rsplit(':', 1)
    auth_headers = {'Accept': ACCEPT}

    def request(kind, name):
        url = f'https://{registry}/v2/{repo}/{kind}/{name}'
        try:
            return fetch(url, auth_headers)
        except urllib.error.HTTPError as error:
            if error.code != 401 or 'Authorization' in auth_headers:
                raise
            challenge = error.headers.get('WWW-Authenticate', '')
            if not challenge.startswith('Bearer '):
                raise ValueError('registry did not offer bearer auth') from None
            fields = dict(re.findall(r'(\w+)="([^"]*)"', challenge))
            realm = fields.pop('realm')
            if not realm.startswith('https://'):
                raise ValueError('registry token endpoint is not HTTPS')
            token_data, _ = fetch(realm + '?' + urllib.parse.urlencode(fields))
            token = json.loads(token_data)
            auth_headers['Authorization'] = 'Bearer ' + (token.get('token') or token['access_token'])
            return fetch(url, auth_headers)

    raw, headers = request('manifests', selector)
    digest = sha(raw)
    if selector.startswith('sha256:') and digest != selector:
        raise ValueError('approved immutable image digest mismatch')
    manifest = json.loads(raw)
    index_digest = None
    if 'manifests' in manifest:
        index_digest = digest
        matching = [m for m in manifest['manifests'] if m.get('platform', {}).get('os') == 'linux'
                    and m.get('platform', {}).get('architecture') == 'amd64'
                    and not m.get('platform', {}).get('variant')]
        if len(matching) != 1:
            raise ValueError('unique linux/amd64 platform required')
        platform_digest = matching[0]['digest']
        platform_raw, _ = request('manifests', platform_digest)
        if sha(platform_raw) != platform_digest:
            raise ValueError('platform manifest digest mismatch')
        manifest = json.loads(platform_raw)
    else:
        platform_digest = digest
    config_digest = manifest['config']['digest']
    config_raw, _ = request('blobs', config_digest)
    if sha(config_raw) != config_digest:
        raise ValueError('image config digest mismatch')
    config = json.loads(config_raw)
    if config.get('os') != 'linux' or config.get('architecture') != 'amd64':
        raise ValueError('resolved image platform mismatch')
    return {'reference': reference, 'registry': registry, 'repository': repo,
            'index_digest': index_digest, 'platform_manifest_digest': platform_digest,
            'config_digest': config_digest, 'platform': 'linux/amd64',
            'immutable_reference': reference.split('@')[0] + '@' + digest}


def release_tool(repo, tag, asset):
    url = f'https://api.github.com/repos/{repo}/releases/tags/{tag}'
    raw, _ = fetch(url)
    release = json.loads(raw)
    selected = [a for a in release['assets'] if a['name'] == asset]
    if len(selected) != 1 or not selected[0].get('digest', '').startswith('sha256:'):
        raise ValueError('published asset checksum missing')
    found = selected[0]
    return {'url': found['browser_download_url'], 'sha256': found['digest'],
            'release_url': release['html_url'], 'release_tag': release['tag_name'],
            'bytes': found['size']}


def resolve():
    tools = {'kind': release_tool('kubernetes-sigs/kind', 'v0.33.0', 'kind-linux-amd64'),
             'istio': release_tool('istio/istio', '1.31.1', 'istio-1.31.1-linux-amd64.tar.gz')}
    checksum_url = 'https://dl.k8s.io/release/v1.35.8/bin/linux/amd64/kubectl.sha256'
    raw, _ = fetch(checksum_url)
    checksum = raw.decode().strip()
    if not re.fullmatch('[0-9a-f]{64}', checksum):
        raise ValueError('invalid published kubectl checksum')
    tools['kubectl'] = {'url': checksum_url.removesuffix('.sha256'), 'sha256': 'sha256:' + checksum,
                        'checksum_url': checksum_url, 'release_tag': 'v1.35.8'}
    sources = {}
    for name, url in {
        'calico': 'https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml',
        'spiffe-csi-example': 'https://raw.githubusercontent.com/spiffe/spiffe-csi/v0.2.13/example/config/spiffe-csi-driver.yaml'
    }.items():
        raw, _ = fetch(url)
        sources[name] = {'url': url, 'sha256': sha(raw)}
        (CACHE / (name + '.yaml')).write_bytes(raw)
    images, failures = [], []
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        pending = {pool.submit(resolve_image, image): image for image in IMAGES}
        for future in concurrent.futures.as_completed(pending):
            ref = pending[future]
            try:
                images.append(future.result())
                print('Resolved', ref, flush=True)
            except urllib.error.HTTPError as error:
                failures.append({'reference': ref, 'http_status': error.code})
                print('Unresolved', ref, 'HTTP', error.code, flush=True)
            except Exception as error:
                failures.append({'reference': ref, 'error_type': type(error).__name__})
                print('Unresolved', ref, type(error).__name__, flush=True)
    result = {'approved_revision': APPROVED, 'platform': 'linux/amd64', 'tools': tools,
              'upstream_manifests': sources, 'images': sorted(images, key=lambda i: i['reference']),
              'resolution_failures': failures, 'status': 'resolved' if not failures else 'incomplete',
              'runtime_compatibility': 'NOT RUN',
              'registrar_basis': 'v2.6.0 from the pinned v0.2.13 CSI example; compatibility/security still requires review.'}
    (CACHE / 'resolution.json').write_text(json.dumps(result, indent=2) + '\n')
    if failures:
        raise SystemExit('Artifact resolution incomplete; no installation authorized by this resolver.')
    OUTPUT.write_text(json.dumps(result, indent=2) + '\n')
    print('Saved', OUTPUT.relative_to(ROOT))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    CACHE.mkdir(parents=True, exist_ok=True)
    resolve()
