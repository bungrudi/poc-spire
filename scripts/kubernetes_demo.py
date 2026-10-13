#!/usr/bin/env python3
"""Own and bound the disposable Kubernetes baseline; never use a default context."""
import argparse
import hashlib
import ipaddress
import json
import os
import shutil
import socket
import stat
import subprocess
import sys
import time
import uuid
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RUNTIME = ROOT / '.runtime/kubernetes-baseline'
TOOLS = ROOT / '.cache/kubernetes-baseline/tools'
ACTIVE = RUNTIME / 'active.json'
DOCKER = shutil.which('docker')
GIB = 1024 ** 3
MEMORY = 4 * GIB
CPUS = 3
POD_CIDR = '10.245.0.0/16'
SERVICE_CIDR = '10.246.0.0/16'
NODE_CIDR = '172.31.244.0/24'
OWNER = 'poc-spire.kubernetes.owner'


def execute(args, *, timeout=120, env=None, data=None):
    result = subprocess.run([str(a) for a in args], input=data, text=True,
                            capture_output=True, timeout=timeout, env=env)
    if result.returncode:
        # Kubernetes output may include credentials. Do not print command/output.
        raise RuntimeError(f'operation failed with exit {result.returncode}; inspect owned private diagnostics')
    return result.stdout


def private_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    temporary = path.with_suffix('.new')
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')
    os.replace(temporary, path)


def memory():
    values = dict(line.split(':', 1) for line in Path('/proc/meminfo').read_text().splitlines())
    return int(values['MemAvailable'].split()[0]) * 1024


def docker_json(*args):
    return json.loads(execute([DOCKER, *args]))


def protected_snapshot():
    containers = {}
    for cid in execute([DOCKER, 'ps', '-q']).split():
        item = docker_json('inspect', cid)[0]
        containers[cid] = {k: item['State'][k] for k in ('Pid', 'StartedAt', 'Running')}
    retained = {}
    directory = Path('/home/ubuntu/Workspace/poc-spire/.runtime/identity')
    if directory.exists():
        for path in directory.rglob('*'):
            info = path.lstat()
            if stat.S_ISREG(info.st_mode):
                retained[str(path.relative_to(directory))] = {
                    'digest': hashlib.sha256(path.read_bytes()).hexdigest(),
                    'mode': stat.S_IMODE(info.st_mode), 'uid': info.st_uid, 'gid': info.st_gid}
    default = Path.home() / '.kube/config'
    with urllib.request.urlopen('https://spire.adianto.dev/healthz', timeout=8) as response:
        public_healthy = response.status == 200
    return {'containers': containers, 'public_https_healthy': public_healthy, 'retained_private_fingerprints': retained,
            'default_context': hashlib.sha256(default.read_bytes()).hexdigest() if default.exists() else None}


def continuity(before):
    after = protected_snapshot()
    return {'public_https_healthy': before['public_https_healthy'] and after['public_https_healthy'],
            'protected_containers_unchanged': all(after['containers'].get(k) == v
                                                for k, v in before['containers'].items()),
            'retained_identity_unchanged': before['retained_private_fingerprints'] == after['retained_private_fingerprints'],
            'default_context_unchanged': before['default_context'] == after['default_context']}


def preflight():
    if memory() < 6 * GIB:
        raise RuntimeError('KB-01: available memory below approved 6 GiB startup floor')
    free = shutil.disk_usage(ROOT).free
    if free < 20 * GIB:
        raise RuntimeError('KB-01: disk below approved 20 GiB startup floor')
    if not DOCKER or os.cpu_count() < CPUS:
        raise RuntimeError('KB-01: Docker or CPU budget unavailable')
    info = docker_json('info', '--format', '{{json .}}')
    if info['Architecture'] != 'x86_64' or info['CgroupVersion'] != '2':
        raise RuntimeError('KB-01: unsupported architecture/cgroup profile')
    if ACTIVE.exists():
        raise RuntimeError('KB-01: existing descriptor; refuse overwrite/adoption')
    for port in (16443, 18089):
        with socket.socket() as listener:
            listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            listener.bind(('127.0.0.1', port))
            listener.listen(1)
    ranges = []
    for nid in execute([DOCKER, 'network', 'ls', '-q']).split():
        item = docker_json('network', 'inspect', nid)[0]
        ranges.extend(c['Subnet'] for c in item['IPAM'].get('Config') or [] if c.get('Subnet'))
    ranges += [r['dst'] for r in json.loads(execute(['ip', '-j', 'route']))
               if r.get('dst') and r['dst'] != 'default']
    requested = [ipaddress.ip_network(cidr) for cidr in (POD_CIDR, SERVICE_CIDR, NODE_CIDR)]
    for existing in ranges:
        network = ipaddress.ip_network(existing, strict=False)
        if any(network.version == candidate.version and network.overlaps(candidate) for candidate in requested):
            raise RuntimeError('KB-01: network range collision; refuse startup')
    pins = json.loads((ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
    if pins['status'] != 'resolved' or pins['resolution_failures']:
        raise RuntimeError('KB-01: incomplete immutable pin resolution')
    for name in ('kind', 'kubectl'):
        binary = TOOLS / name
        if not binary.is_file() or 'sha256:' + hashlib.sha256(binary.read_bytes()).hexdigest() != pins['tools'][name]['sha256']:
            raise RuntimeError('KB-01: tool checksum mismatch')
    return {'available_memory_bytes': memory(), 'disk_free_bytes': free,
            'node_limit_memory_bytes': MEMORY, 'node_limit_cpus': CPUS,
            'host_reserve_bytes': 2 * GIB, 'ports': [16443, 18089],
            'pod_cidr': POD_CIDR, 'service_cidr': SERVICE_CIDR, 'node_cidr': NODE_CIDR,
            'docker_version': info['ServerVersion'], 'cgroup_version': info['CgroupVersion'],
            'platform': 'linux/amd64', 'runtime_compatibility': 'NOT RUN'}


def descriptor():
    if ACTIVE.is_symlink() or not ACTIVE.is_file():
        raise RuntimeError('owned descriptor unavailable')
    value = json.loads(ACTIVE.read_text())
    run = value['run']
    if len(run) != 12 or any(c not in '0123456789abcdef' for c in run):
        raise RuntimeError('invalid owned run')
    work = RUNTIME / run
    if value['work'] != str(work) or value['cluster'] != 'poc-k8s-' + run or value['node'] != value['cluster'] + '-control-plane':
        raise RuntimeError('descriptor scope mismatch')
    marker = work / 'owner.json'
    if marker.is_symlink() or json.loads(marker.read_text()) != value:
        raise RuntimeError('ownership marker mismatch')
    return value


def kube(value, *args, timeout=120, data=None):
    return execute([TOOLS / 'kubectl', '--kubeconfig', Path(value['work']) / 'kubeconfig',
                    '--context', 'kind-' + value['cluster'], *args], timeout=timeout, data=data)


def apply_csi_workloads(value, manifest, *, data=None, timeout=120):
    # Record intent before the request: timeout/partial apply can already have mounted CSI Pods.
    (Path(value['work']) / 'apps-mounted').write_text('owned app CSI mount intent\n')
    return kube(value, 'apply', '-f', manifest, data=data, timeout=timeout)


def wrapper(value):
    directory = Path(value['work']) / 'docker-wrapper'
    directory.mkdir(mode=0o700)
    path = directory / 'docker'
    source = '''#!/usr/bin/env python3
import os, sys
args=sys.argv[1:]
if args and args[0]=='run':
    expected=NODE
    if '--name' not in args or args[args.index('--name')+1]!=expected:
        raise SystemExit('refuse unowned Docker run')
    if 'io.x-k8s.kind.cluster='+CLUSTER not in args or '--privileged' not in args:
        raise SystemExit('refuse unrecognized kind node')
    if any(arg.startswith(('--memory', '--cpus', '--cpu-quota', '--cgroup-parent')) for arg in args):
        raise SystemExit('unexpected resource override')
    args=[arg for arg in args if not arg.startswith('--restart=')]
    args[1:1]=['--memory=4294967296','--memory-swap=4294967296','--cpus=3',
               '--restart=no','--label=poc-spire.kubernetes.owner='+RUN]
os.execv(DOCKER,[DOCKER,*args])
'''
    constants = 'NODE='+repr(value['node'])+'\nCLUSTER='+repr(value['cluster'])+'\nRUN='+repr(value['run'])+'\nDOCKER='+repr(DOCKER)+'\n'
    source = source.replace('import os, sys\n', 'import os, sys\n' + constants)
    path.write_text(source)
    path.chmod(0o700)
    return directory


def start():
    result = preflight()
    before = protected_snapshot()
    run = uuid.uuid4().hex[:12]
    cluster = 'poc-k8s-' + run
    value = {'run': run, 'cluster': cluster, 'node': cluster + '-control-plane',
             'network': cluster, 'work': str(RUNTIME / run)}
    work = Path(value['work'])
    if execute([DOCKER, 'ps', '-aq', '--filter', 'label=io.x-k8s.kind.cluster=' + cluster]).strip():
        raise RuntimeError('cluster name collision')
    if value['network'] in execute([DOCKER, 'network', 'ls', '--format', '{{.Name}}']).split():
        raise RuntimeError('network name collision')
    work.mkdir(parents=True, mode=0o700)
    private_json(work / 'owner.json', value)
    private_json(work / 'protected-baseline.json', before)
    private_json(ACTIVE, value)
    pins = json.loads((ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
    node = next(p['immutable_reference'] for p in pins['images'] if p['reference'].startswith('kindest/node:'))
    config = work / 'kind.yaml'
    total_memory = int(dict(line.split(':', 1) for line in Path('/proc/meminfo').read_text().splitlines())['MemTotal'].split()[0]) * 1024
    system_memory = max(0, (total_memory - MEMORY) // 1024)
    system_cpu = max(0, os.cpu_count() - CPUS)
    config.write_text('''kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: 16443
  disableDefaultCNI: true
  podSubnet: "10.245.0.0/16"
  serviceSubnet: "10.246.0.0/16"
nodes:
- role: control-plane
  image: NODE_IMAGE
  kubeadmConfigPatches:
  - |
    kind: KubeletConfiguration
    serverTLSBootstrap: true
    readOnlyPort: 0
    authentication:
      anonymous:
        enabled: false
      webhook:
        enabled: true
    authorization:
      mode: Webhook
    enforceNodeAllocatable: [pods]
    systemReserved:
      cpu: "SYSTEM_CPU"
      memory: "SYSTEM_MEMORYKi"
    kubeReserved:
      cpu: "500m"
      memory: "512Mi"
    evictionHard:
      memory.available: "150Mi"
      nodefs.available: "10%"
      imagefs.available: "15%"
'''.replace('NODE_IMAGE', node).replace('SYSTEM_CPU', str(system_cpu)).replace('SYSTEM_MEMORY', str(system_memory)))
    env = os.environ.copy()
    env['PATH'] = str(wrapper(value)) + ':' + env['PATH']
    env['KIND_EXPERIMENTAL_DOCKER_NETWORK'] = value['network']
    env['KUBECONFIG'] = str(work / 'kubeconfig')
    try:
        execute([DOCKER, 'network', 'create', '--driver', 'bridge', '--subnet', NODE_CIDR,
                 '--label', OWNER + '=' + run, value['network']])
        print('Starting owned node with 4 GiB / 3 CPU limits from Docker creation.', flush=True)
        outcome = subprocess.run([str(TOOLS / 'kind'), 'create', 'cluster', '--name', cluster,
                                  '--image', node, '--config', str(config), '--kubeconfig', env['KUBECONFIG'],
                                  '--wait', '0s'], text=True, capture_output=True, env=env, timeout=600)
        diagnostics = RUNTIME / 'diagnostics'
        diagnostics.mkdir(mode=0o700, exist_ok=True)
        # Bounded private diagnostics survive successful ownership cleanup.
        (diagnostics / (run + '-kind.txt')).write_text((outcome.stdout + outcome.stderr)[-65536:])
        if outcome.returncode:
            raise RuntimeError('kind initialization failed; owned diagnostic output retained')
        (work / 'kubeconfig').chmod(0o600)
        item = docker_json('inspect', value['node'])[0]
        limits = item['HostConfig']
        if limits['Memory'] != MEMORY or limits['MemorySwap'] != MEMORY or limits['NanoCpus'] != CPUS * 10**9:
            raise RuntimeError('KB-25: actual owned node limits differ')
        if item['Config']['Labels'].get(OWNER) != run:
            raise RuntimeError('owned node label mismatch')
        result['actual_node_limits'] = {k: limits[k] for k in ('Memory', 'MemorySwap', 'NanoCpus')}
        result['continuity'] = continuity(before)
        if not all(result['continuity'].values()) or memory() < 2 * GIB:
            raise RuntimeError('KB-25/27: protected continuity/reserve check failed')
        result['cluster'] = cluster
        result['api_version'] = json.loads(kube(value, 'version', '-o', 'json'))['serverVersion']['gitVersion']
        private_json(work / 'bootstrap-result.json', result)
        print(json.dumps(result, indent=2))
    except Exception:
        stop()
        raise


def owned_image_id(record):
    if 'docker_image_id' in record:
        return record['docker_image_id']
    if 'docker_config_digest' in record:
        return record['docker_config_digest']
    return record['config_digest']


def stop():
    value = descriptor()
    before = json.loads((Path(value['work']) / 'protected-baseline.json').read_text())
    from kubernetes_guard import stop as stop_observer
    stop_observer(value)
    ids = execute([DOCKER, 'ps', '-aq', '--filter', 'name=^' + value['node'] + '$']).split()
    if ids:
        if len(ids) != 1:
            raise RuntimeError('cleanup ambiguous node ownership')
        item = docker_json('inspect', ids[0])[0]
        if item['Config']['Labels'].get(OWNER) != value['run'] or item['Config']['Labels'].get('io.x-k8s.kind.cluster') != value['cluster']:
            raise RuntimeError('cleanup refused foreign node; descriptor retained')
        # Mounted app CSI volumes must be removed before driver/node teardown.
        if (Path(value['work']) / 'apps-mounted').exists():
            try:
                kube(value, 'delete', 'namespace', 'poc-' + value['run'], '--ignore-not-found', '--wait=true', '--timeout=90s', timeout=100)
            except Exception:
                # Retain the node/CSI and descriptor for safe retry; no false cleanup pass.
                raise RuntimeError('app unmount/namespace cleanup failed; ownership retained') from None
        execute([DOCKER, 'rm', '-f', '-v', ids[0]], timeout=120)
    networks = execute([DOCKER, 'network', 'ls', '--filter', 'name=^' + value['network'] + '$', '-q']).split()
    if networks:
        if len(networks) != 1:
            raise RuntimeError('cleanup ambiguous network ownership')
        item = docker_json('network', 'inspect', networks[0])[0]
        if item['Labels'].get(OWNER) != value['run'] or item['Containers']:
            raise RuntimeError('cleanup refused foreign/in-use network')
        execute([DOCKER, 'network', 'rm', networks[0]])
    for name in ('bootstrap-image.json','feasibility-image.json','normal-image.json','test-image.json'):
        image_record = Path(value['work']) / name
        if image_record.exists():
            built = json.loads(image_record.read_text())
            probe = subprocess.run([DOCKER, 'image', 'inspect', built['tag'], '--format', '{{.Id}}'], text=True, capture_output=True)
            if probe.returncode == 0:
                if probe.stdout.strip() != owned_image_id(built):
                    raise RuntimeError('cleanup refused reassigned image tag; ownership retained')
                execute([DOCKER, 'image', 'rm', built['tag']])
    checks = continuity(before)
    if not all(checks.values()):
        raise RuntimeError('protected continuity failure; descriptor retained')
    ACTIVE.unlink()
    shutil.rmtree(value['work'])
    print(json.dumps({'owned_cleanup_complete': True, **checks}))


if __name__ == '__main__':
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('preflight', 'start', 'stop'))
    args = parser.parse_args()
    try:
        if args.action == 'preflight':
            print(json.dumps(preflight(), indent=2))
        elif args.action == 'start':
            start()
        else:
            stop()
    except Exception as error:
        print(str(error), file=sys.stderr)
        raise SystemExit(1)
