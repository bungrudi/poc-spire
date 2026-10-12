#!/usr/bin/env python3
"""Align the owned kubelet scheduler and Pod cgroup with its approved node budget."""
import json
import subprocess
import time
from pathlib import Path

import yaml
import kubernetes_demo as runtime


def memory_bytes(quantity):
    if quantity.endswith('Ki'):
        return int(quantity[:-2]) * 1024
    if quantity.endswith('Mi'):
        return int(quantity[:-2]) * 1024 ** 2
    if quantity.endswith('Gi'):
        return int(quantity[:-2]) * 1024 ** 3
    return int(quantity)


def main():
    value = runtime.descriptor()
    node = json.loads(runtime.kube(value, 'get', 'node', value['node'], '-o', 'json'))
    capacity = node['status']['capacity']
    expected = {'cpu': str(int(capacity['cpu']) - runtime.CPUS),
                'memory': str((memory_bytes(capacity['memory']) - runtime.MEMORY) // 1024) + 'Ki'}
    if int(expected['cpu']) < 0 or int(expected['memory'][:-2]) < 0:
        raise RuntimeError('reported node capacity below approved cap')
    configuration = yaml.safe_load(runtime.execute([runtime.DOCKER, 'exec', value['node'], 'cat', '/var/lib/kubelet/config.yaml']))
    if configuration.get('cgroupRoot') != '/kubelet' or configuration.get('cgroupDriver') != 'systemd':
        raise RuntimeError('unrecognized pinned kind Pod cgroup layout')
    configuration['systemReserved'] = expected
    configuration['kubeReserved'] = {'cpu': '500m', 'memory': '512Mi'}
    configuration['enforceNodeAllocatable'] = ['pods']
    configuration['evictionHard'] = {'memory.available': '150Mi', 'nodefs.available': '10%', 'imagefs.available': '15%'}
    configuration['serverTLSBootstrap'] = True
    configuration['readOnlyPort'] = 0
    configuration['authentication']['anonymous']['enabled'] = False
    body = yaml.safe_dump(configuration)
    runtime.execute([runtime.DOCKER, 'exec', '-i', value['node'], 'tee', '/var/lib/kubelet/config.yaml'], data=body)
    runtime.execute([runtime.DOCKER, 'exec', value['node'], 'systemctl', 'restart', 'kubelet'])
    for attempt in range(45):
        node = json.loads(runtime.kube(value, 'get', 'node', value['node'], '-o', 'json'))
        alloc = node['status']['allocatable']
        cpu_millis = int(alloc['cpu'][:-1]) if alloc['cpu'].endswith('m') else int(alloc['cpu']) * 1000
        if memory_bytes(alloc['memory']) <= runtime.MEMORY and cpu_millis <= runtime.CPUS * 1000:
            break
        if attempt == 44:
            raise RuntimeError('scheduler still exceeds owned node cgroup budget')
        time.sleep(1)
    observed = runtime.execute([runtime.DOCKER, 'exec', value['node'], 'sh', '-c',
                                'cat /sys/fs/cgroup/memory.max /sys/fs/cgroup/memory.current /sys/fs/cgroup/memory.peak /sys/fs/cgroup/cpu.max /sys/fs/cgroup/memory.events /sys/fs/cgroup/kubelet.slice/kubelet-kubepods.slice/memory.max']).splitlines()
    if observed[0] != str(runtime.MEMORY) or observed[3] != '300000 100000':
        raise RuntimeError('actual node memory/CPU cgroups differ')
    if observed[-1] == 'max' or int(observed[-1]) > runtime.MEMORY:
        raise RuntimeError('Pod cgroup budget not bounded')
    # This profile has no PVC/durability. Remove kind's unused storage addon.
    runtime.kube(value, 'delete', 'namespace', 'local-path-storage', '--ignore-not-found', '--timeout=60s', timeout=70)
    runtime.kube(value, 'delete', 'storageclass', 'standard', '--ignore-not-found', timeout=30)
    result = {'capacity': capacity, 'allocatable': alloc, 'system_reserved': expected,
              'actual_node_memory_max': int(observed[0]), 'actual_node_memory_current': int(observed[1]),
              'actual_node_memory_peak': int(observed[2]), 'actual_node_cpu_max': observed[3],
              'actual_pod_memory_max': int(observed[-1]), 'memory_events': observed[4:-1],
              'host_available_memory_bytes': runtime.memory(), 'unused_storage_addon_removed': True,
              'aggregate_app_requests_and_pressure_fixture': 'NOT RUN'}
    if runtime.memory() < 2 * runtime.GIB:
        raise RuntimeError('host reserve breached')
    runtime.private_json(Path(value['work']) / 'budget-result.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
