import json
import hashlib
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import kubernetes_demo as runtime


class OwnershipTests(unittest.TestCase):
    def test_reported_capacity_sample_is_the_sample_that_passed_final_gate(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);tools=root/'tools';tools.mkdir();pins=root/'infra/kubernetes-baseline';pins.mkdir(parents=True)
            catalog={'status':'resolved','resolution_failures':[],'tools':{}}
            for name in ('kind','kubectl'):
                data=name.encode();(tools/name).write_bytes(data)
                catalog['tools'][name]={'sha256':'sha256:'+hashlib.sha256(data).hexdigest()}
            (pins/'artifacts.json').write_text(json.dumps(catalog))
            info={'Architecture':'x86_64','CgroupVersion':'2','ServerVersion':'test'}
            floor=6*runtime.GIB
            with patch.object(runtime,'ROOT',root),patch.object(runtime,'TOOLS',tools),patch.object(runtime,'ACTIVE',root/'absent'), \
                 patch.object(runtime,'DOCKER','fake'),patch.object(runtime,'docker_json',return_value=info), \
                 patch.object(runtime.shutil,'disk_usage',return_value=runtime.shutil._ntuple_diskusage(40*runtime.GIB,8*runtime.GIB,32*runtime.GIB)), \
                 patch.object(runtime,'execute',side_effect=lambda args: '[]' if args[0]=='ip' else ''), \
                 patch.object(runtime.socket,'socket'):
                with patch.object(runtime,'memory',side_effect=[floor+1,floor-1]):
                    with self.assertRaisesRegex(RuntimeError,'startup floor'):runtime.preflight()
                with patch.object(runtime,'memory',side_effect=[floor+1,floor+2]):
                    self.assertEqual(runtime.preflight()['available_memory_bytes'],floor+2)

    def test_partial_csi_apply_records_unmount_intent_before_mutation(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / 'apps-mounted'
            def fail(*args, **kwargs):
                self.assertTrue(marker.exists(), 'CSI unmount intent was recorded too late')
                raise RuntimeError('partial apply timeout')
            with patch.object(runtime, 'kube', side_effect=fail):
                with self.assertRaisesRegex(RuntimeError, 'partial apply timeout'):
                    runtime.apply_csi_workloads({'work':directory}, '-', data='owned secret fixture')
            self.assertTrue(marker.exists(), 'cleanup lost partial CSI apply intent')

    def test_typed_store_descriptor_cleanup_record_needs_no_config_alias(self):
        record = {'docker_image_id': 'sha256:index', 'native_descriptor_graph': {'config_digest': 'sha256:config'}}
        self.assertEqual(runtime.owned_image_id(record), 'sha256:index')
        self.assertEqual(runtime.owned_image_id({'config_digest': 'historical-store-id'}), 'historical-store-id')

    def test_foreign_marker_cannot_be_adopted_or_deleted(self):
        with tempfile.TemporaryDirectory() as temp:
            base = Path(temp)
            run = 'abcdef012345'
            work = base / run
            work.mkdir()
            value = {'run': run, 'cluster': 'poc-k8s-' + run,
                     'node': 'poc-k8s-' + run + '-control-plane', 'work': str(work)}
            active = base / 'active.json'
            active.write_text(json.dumps(value))
            (work / 'owner.json').write_text(json.dumps({**value, 'run': '111111111111'}))
            with patch.object(runtime, 'RUNTIME', base), patch.object(runtime, 'ACTIVE', active), \
                    patch.object(runtime, 'execute') as execute:
                with self.assertRaisesRegex(RuntimeError, 'marker mismatch'):
                    runtime.stop()
                execute.assert_not_called()
                self.assertTrue(active.exists())
            (work / 'owner.json').unlink()
            (work / 'owner.json').symlink_to(active)
            with patch.object(runtime, 'RUNTIME', base), patch.object(runtime, 'ACTIVE', active):
                with self.assertRaisesRegex(RuntimeError, 'marker mismatch'):
                    runtime.descriptor()

    def test_resource_wrapper_bounds_owned_run_and_rejects_foreign_run(self):
        with tempfile.TemporaryDirectory() as temp:
            base = Path(temp)
            capture = base / 'captured.json'
            fake = base / 'real-docker'
            fake.write_text('#!/usr/bin/env python3\nimport json,sys\nfrom pathlib import Path\n'
                            + 'Path(' + repr(str(capture)) + ').write_text(json.dumps(sys.argv[1:]))\n')
            fake.chmod(0o700)
            value = {'work': str(base), 'node': 'owned-control-plane',
                     'cluster': 'owned', 'run': 'abcdef012345'}
            with patch.object(runtime, 'DOCKER', str(fake)):
                wrapped = runtime.wrapper(value) / 'docker'
            original = ['run', '--name', value['node'], '--label', 'io.x-k8s.kind.cluster=owned',
                        '--privileged', '--restart=on-failure:1', 'pinned-image@sha256:abc']
            result = subprocess.run([str(wrapped), *original], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0)
            observed = json.loads(capture.read_text())
            for setting in ('--memory=4294967296', '--memory-swap=4294967296', '--cpus=3', '--restart=no'):
                self.assertIn(setting, observed)
            self.assertNotIn('--restart=on-failure:1', observed)
            capture.unlink()
            result = subprocess.run([str(wrapped), 'run', '--name', 'foreign', '--privileged', 'image'],
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(capture.exists())
            for conflicting in ('--memory=8g', '--cpus=6', '--cgroup-parent=foreign'):
                result = subprocess.run([str(wrapped), *original[:-1], conflicting, original[-1]],
                                        capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(capture.exists())


if __name__ == '__main__':
    unittest.main()
