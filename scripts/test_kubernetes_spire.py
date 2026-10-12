import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import kubernetes_spire as spire


class IssuerOwnershipTests(unittest.TestCase):
    def test_lost_pod_or_namespace_cannot_rebootstrap_initialized_run(self):
        with tempfile.TemporaryDirectory() as directory:
            value = {'work': directory, 'run': 'abcdef012345', 'cluster': 'owned'}
            marker = spire.ensure_fresh_issuer(value)
            marker.write_text(json.dumps({'phase': 'initialized', 'server_pod_uid': 'original'}))
            # No Pod/namespace is needed to refuse a retry: the run marker is authoritative.
            with patch.object(spire.runtime, 'descriptor', return_value=value), \
                    patch.object(spire.runtime, 'kube') as kube:
                with self.assertRaisesRegex(RuntimeError, 'fresh run required'):
                    spire.main()
                kube.assert_not_called()
            self.assertEqual(json.loads(marker.read_text())['server_pod_uid'], 'original')

    def test_partial_bootstrap_cannot_be_silently_retried(self):
        with tempfile.TemporaryDirectory() as directory:
            value = {'work': directory}
            marker = spire.ensure_fresh_issuer(value)
            marker.write_text(json.dumps({'phase': 'initialization-started'}))
            with self.assertRaisesRegex(RuntimeError, 'fresh run required'):
                spire.ensure_fresh_issuer(value)


if __name__ == '__main__':
    unittest.main()
