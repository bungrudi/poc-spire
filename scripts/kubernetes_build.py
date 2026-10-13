#!/usr/bin/env python3
"""Build and load only an owned bootstrap image; no network or credential build inputs."""
import hashlib
import json
import os
from pathlib import Path

import kubernetes_demo as runtime


def main():
    value = runtime.descriptor()
    root = Path(value['work'])
    marker = root / 'bootstrap-image.json'
    if marker.exists():
        raise RuntimeError('owned bootstrap image already built; refuse silent replacement')
    directory = root / 'bootstrap-image'
    directory.mkdir(mode=0o700)
    env = {**os.environ, 'CGO_ENABLED': '0', 'GOTOOLCHAIN': 'local', 'GOMAXPROCS': '2', 'GOFLAGS': '-p=2'}
    runtime.execute(['go', 'build', '-trimpath', '-o', directory / 'run-guard', './cmd/kubernetes-run-guard'], env=env, timeout=120)
    (directory / 'Dockerfile').write_text('FROM scratch\nCOPY run-guard /run-guard\nUSER 0:0\nENTRYPOINT ["/run-guard"]\n')
    (directory / '.dockerignore').write_text('*\n!Dockerfile\n!run-guard\n')
    tag = 'poc-k8s-bootstrap:' + value['run']
    intent = runtime.image_build_intent(value,'bootstrap',tag)
    runtime.execute([runtime.DOCKER, 'build', '--network', 'none', '--label', runtime.OWNER + '=' + value['run'], '-t', tag, directory], timeout=120)
    runtime.image_created(value,intent)
    inspected = runtime.docker_json('image', 'inspect', tag)[0]
    store_id = inspected['Id']
    source = directory / 'docker.tar'
    runtime.execute([runtime.DOCKER, 'image', 'save', '--output', source, tag], timeout=120)
    from kubernetes_images import descriptor_graph
    graph = descriptor_graph(source, inspected)
    if runtime.docker_json('image', 'inspect', tag)[0]['Id'] != store_id:
        raise RuntimeError('bootstrap image changed during export')
    runtime.private_json(marker, {'tag': tag, 'docker_image_id': store_id, 'native_descriptor_graph': graph,
                                  'binary_sha256': hashlib.sha256((directory / 'run-guard').read_bytes()).hexdigest(),
                                  'source_revision': runtime.execute(['git', 'rev-parse', 'HEAD']).strip(),
                                  'go_version': runtime.execute(['go', 'version']).strip()})
    runtime.execute([runtime.TOOLS / 'kind', 'load', 'docker-image', tag, '--name', value['cluster']], timeout=120)
    print('Built and loaded owned bootstrap helper', tag)


if __name__ == '__main__':
    main()
