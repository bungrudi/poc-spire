#!/usr/bin/env python3
"""Build static development apps and import a real immutable OCI manifest into the owned node."""
import argparse
import gzip
import hashlib
import io
import json
import os
import subprocess
import tarfile
from pathlib import Path

import kubernetes_demo as runtime

OCI_MANIFEST = 'application/vnd.oci.image.manifest.v1+json'
OCI_CONFIG = 'application/vnd.oci.image.config.v1+json'
OCI_INDEX = 'application/vnd.oci.image.index.v1+json'


def digest(data):
    return 'sha256:' + hashlib.sha256(data).hexdigest()


def canonical(value):
    return json.dumps(value, separators=(',', ':'), sort_keys=True).encode()


def add(archive, name, data):
    entry = tarfile.TarInfo(name)
    entry.size = len(data)
    entry.mode = 0o600
    entry.mtime = 0
    archive.addfile(entry, io.BytesIO(data))


def docker_to_oci(source, target, reference):
    with tarfile.open(source) as original:
        manifests = json.load(original.extractfile('manifest.json'))
        if len(manifests) != 1:
            raise ValueError('single owned image required')
        item = manifests[0]
        config = original.extractfile(item['Config']).read()
        config_json = json.loads(config)
        if config_json['architecture'] != 'amd64' or config_json['os'] != 'linux':
            raise ValueError('unexpected image platform')
        layers = []
        blobs = {digest(config): config}
        for name, expected in zip(item['Layers'], config_json['rootfs']['diff_ids'], strict=True):
            body = original.extractfile(name).read()
            compressed = body.startswith(b'\x1f\x8b')
            if digest(gzip.decompress(body) if compressed else body) != expected:
                raise ValueError('layer/config digest mismatch')
            layer_digest = digest(body)
            media = 'application/vnd.oci.image.layer.v1.tar' + ('+gzip' if compressed else '')
            layers.append({'mediaType': media, 'digest': layer_digest, 'size': len(body)})
            blobs[layer_digest] = body
        manifest = canonical({'schemaVersion': 2, 'mediaType': OCI_MANIFEST,
                              'config': {'mediaType': OCI_CONFIG, 'digest': digest(config), 'size': len(config)}, 'layers': layers})
        manifest_digest = digest(manifest)
        blobs[manifest_digest] = manifest
        index = canonical({'schemaVersion': 2, 'mediaType': OCI_INDEX, 'manifests': [{
            'mediaType': OCI_MANIFEST, 'digest': manifest_digest, 'size': len(manifest),
            'platform': {'os': 'linux', 'architecture': 'amd64'},
            'annotations': {'io.containerd.image.name': reference, 'org.opencontainers.image.ref.name': reference}}]})
        with tarfile.open(target, 'w') as output:
            add(output, 'oci-layout', canonical({'imageLayoutVersion': '1.0.0'}))
            add(output, 'index.json', index)
            for key, body in blobs.items():
                add(output, 'blobs/sha256/' + key.removeprefix('sha256:'), body)
        return {'index_digest': digest(index), 'platform_manifest_digest': manifest_digest, 'config_digest': digest(config)}


def descriptor_graph(source, inspected):
    root = inspected.get('Descriptor')
    if not root or inspected['Id'] != root['digest']:
        raise ValueError('Docker store descriptor unavailable/inconsistent')
    with tarfile.open(source) as archive:
        def verified(descriptor):
            body = archive.extractfile('blobs/sha256/' + descriptor['digest'].removeprefix('sha256:')).read()
            if digest(body) != descriptor['digest'] or len(body) != descriptor['size']:
                raise ValueError('exported descriptor digest/size mismatch')
            return json.loads(body)
        current = root
        graph = []
        obj = verified(current)
        while 'manifests' in obj:
            graph.append({'media_type': current['mediaType'], 'digest': current['digest']})
            matching = [item for item in obj['manifests'] if item.get('platform', {}).get('os') == 'linux'
                        and item.get('platform', {}).get('architecture') == 'amd64']
            if len(matching) != 1:
                raise ValueError('unique native linux/amd64 descriptor required')
            current = matching[0]
            obj = verified(current)
        if current['mediaType'] not in (OCI_MANIFEST, 'application/vnd.docker.distribution.manifest.v2+json'):
            raise ValueError('unsupported native platform manifest')
        config = verified(obj['config'])
        if config['architecture'] != 'amd64' or config['os'] != 'linux':
            raise ValueError('native config platform differs')
        return {'store_descriptor': root, 'indexes': graph, 'platform_manifest_digest': current['digest'],
                'config_digest': obj['config']['digest'], 'filesystem_diff_ids': config['rootfs']['diff_ids']}


def main(profile="feasibility"):
    value = runtime.descriptor()
    work = Path(value['work'])
    marker = work / (profile+'-image.json')
    if marker.exists():
        raise RuntimeError('refuse silent replacement of built feasibility artifact')
    directory = work / (profile+'-image')
    if directory.is_symlink():
        raise RuntimeError('aliased build directory')
    directory.mkdir(mode=0o700, exist_ok=True)
    env = {**os.environ, 'CGO_ENABLED': '0', 'GOTOOLCHAIN': 'local', 'GOMAXPROCS': '2', 'GOFLAGS': '-p=2'}
    names = ['money','trusted-caller','untrusted-caller','ui','kafka-init','control-client','security-client']
    if profile != 'feasibility': names += ['control-adapter','kubernetes-init','bundle-publisher','kubernetes-run-guard','kubernetes-ready']
    for name in names:
        runtime.execute(['go','build','-trimpath',*(['-tags','kafkatest'] if profile=='test' else []),'-o',directory/name,'./cmd/'+name], env=env, timeout=120)
    for role in ('approved', 'ineligible'):
        name = 'attestation-' + role
        runtime.execute(['go', 'build', '-trimpath', '-ldflags', '-X main.buildRole=' + role,
                         '-o', directory / name, './cmd/attestation-fixture'], env=env, timeout=120)
        names.append(name)
    (directory / 'Dockerfile').write_text('FROM scratch\nCOPY ' + ' '.join(names) + ' /\nUSER 1000:1000\nENTRYPOINT ["/money"]\n')
    (directory / '.dockerignore').write_text('*\n!Dockerfile\n' + ''.join('!' + n + '\n' for n in names))
    tag = 'poc-k8s-'+profile+':' + value['run']
    runtime.execute([runtime.DOCKER, 'build', '--network', 'none', '--label', runtime.OWNER + '=' + value['run'], '-t', tag, directory], timeout=120)
    saved = directory / 'docker.tar'
    runtime.execute([runtime.DOCKER, 'image', 'save', '--output', saved, tag], timeout=120)
    archive = directory / 'image.oci.tar'
    reference = 'docker.io/poc-spire/'+profile+':' + value['run']
    records = docker_to_oci(saved, archive, reference)
    image_config = runtime.execute([runtime.DOCKER, 'image', 'inspect', tag, '--format', '{{.Id}}']).strip()
    inspected = runtime.docker_json('image', 'inspect', tag)[0]
    # Docker 29 .Id is its typed store descriptor (an OCI index on this host).
    # Decode/hash its graph, then verify the reconstructed executable filesystem.
    with tarfile.open(saved) as exported:
        original_manifest = json.load(exported.extractfile('manifest.json'))[0]
        exported_config = json.load(exported.extractfile(original_manifest['Config']))
    if exported_config['rootfs']['diff_ids'] != inspected['RootFS']['Layers']:
        raise RuntimeError('Docker/OCI executable filesystem differs')
    for field in ('User', 'Entrypoint', 'Labels'):
        if exported_config['config'].get(field) != inspected['Config'].get(field):
            raise RuntimeError('Docker/OCI deployment semantics differ')
    graph = descriptor_graph(saved, inspected)
    if graph['config_digest'] != records['config_digest']:
        raise RuntimeError('native/exported OCI config differs')
    if runtime.docker_json('image', 'inspect', tag)[0]['Id'] != image_config:
        raise RuntimeError('image tag changed while hashing/exporting')
    records['docker_image_id'] = image_config
    records['native_descriptor_graph'] = graph
    with archive.open('rb') as input_stream:
        result = subprocess.run([runtime.DOCKER, 'exec', '-i', value['node'], 'ctr', '-n', 'k8s.io', 'images', 'import', '--digests', '-'],
                                stdin=input_stream, capture_output=True, timeout=180)
    if result.returncode:
        raise RuntimeError('owned OCI import failed')
    immutable = reference.split(':')[0] + '@' + records['platform_manifest_digest']
    runtime.execute([runtime.DOCKER, 'exec', value['node'], 'ctr', '-n', 'k8s.io', 'images', 'tag', reference, immutable], timeout=60)
    records.update({'tag': tag, 'immutable_reference': reference.split(':')[0] + '@' + records['platform_manifest_digest'],
                    'binary_sha256': {n: hashlib.sha256((directory / n).read_bytes()).hexdigest() for n in names},
                    'source_revision': runtime.execute(['git', 'rev-parse', 'HEAD']).strip(),
                    'source_dirty': bool(runtime.execute(['git', 'status', '--porcelain']).strip()),
                    'profile':profile,'purpose':'development artifact; final source freeze remains pending'})
    runtime.private_json(marker, records)
    print(json.dumps({'immutable_reference': records['immutable_reference'], 'config_digest': records['config_digest'],
                      'source_revision': records['source_revision'], 'source_dirty': records['source_dirty']}, indent=2))


if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--profile',choices=('feasibility','normal','test'),default='feasibility')
    main(parser.parse_args().profile)
