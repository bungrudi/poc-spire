import io
import json
import tarfile
import tempfile
import unittest
from pathlib import Path

import kubernetes_images as images


class ImageSeparationTests(unittest.TestCase):
    def test_normal_has_no_adversarial_fixture_components(self):
        normal=set(images.components('normal'))
        fixtures={'security-client','control-client','kafka-init','attestation-approved','attestation-ineligible','kubernetes-destination-probe'}
        self.assertFalse(normal & fixtures)
        self.assertTrue(fixtures <= set(images.components('test')))
        self.assertIn('kubernetes-init',normal)
        self.assertIn('kubernetes-broker-ready',normal)

    def test_exported_layer_inventory_rejects_unexpected_fixture_or_missing_runtime(self):
        with tempfile.TemporaryDirectory() as directory:
            saved=Path(directory)/'docker.tar'
            def artifact(names, mode=0o555):
                layer=io.BytesIO()
                with tarfile.open(fileobj=layer,mode='w') as archive:
                    for name in names:
                        entry=tarfile.TarInfo(name);entry.mode=mode;entry.size=3
                        archive.addfile(entry,io.BytesIO(b'app'))
                with tarfile.open(saved,'w') as archive:
                    for name,body in [('manifest.json',json.dumps([{'Layers':['layer.tar']}]).encode()),('layer.tar',layer.getvalue())]:
                        entry=tarfile.TarInfo(name);entry.size=len(body);archive.addfile(entry,io.BytesIO(body))
            artifact(['money'])
            self.assertEqual(set(images.file_inventory(saved,['money'])),{'money'})
            artifact(['money'], mode=0o700)
            with self.assertRaisesRegex(ValueError,'inaccessible to the non-root workload UID'):images.file_inventory(saved,['money'])
            artifact(['money','security-client'])
            with self.assertRaisesRegex(ValueError,'unexpected image'):images.file_inventory(saved,['money'])
            artifact([])
            with self.assertRaisesRegex(ValueError,'inventory differs'):images.file_inventory(saved,['money'])


if __name__=='__main__':unittest.main()
