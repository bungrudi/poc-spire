import copy
import json
import unittest
from unittest.mock import patch

import kubernetes_attestation as attestation
import kubernetes_mesh as mesh
import kubernetes_mesh_identity as identity


class MeshIdentityTests(unittest.TestCase):
    def test_custom_socket_and_proxy_only_csi_volume(self):
        for native in ('true', 'false'):
            template = {'metadata': {'annotations': {'sidecar.istio.io/nativeSidecar': native}},
                        'spec': {'containers': [{'name': 'caller', 'volumeMounts': [{'name': 'workload', 'mountPath': '/workload'}]}]}}
            identity.configure(template)
            metadata = json.loads(template['metadata']['annotations']['proxy.istio.io/config'])['proxyMetadata']
            self.assertEqual(metadata['WORKLOAD_IDENTITY_SOCKET_FILE'], 'api.sock')
            self.assertEqual(template['spec']['volumes'], [{'name': 'workload-socket', 'csi': {'driver': 'csi.spiffe.io', 'readOnly': True}}])
            self.assertNotIn('workload-socket', str(template['spec']['containers']))
            with self.assertRaisesRegex(RuntimeError, 'conflicting'):
                identity.configure(template)
        self.assertEqual(mesh.operator()['spec']['meshConfig']['trustDomain'], 'poc.internal')

    def test_proxy_grant_cannot_match_application_or_another_pod(self):
        pins = json.loads((mesh.runtime.ROOT / 'infra/kubernetes-baseline/artifacts.json').read_text())
        pin = next(i for i in pins['images'] if i['reference'] == 'istio/proxyv2:1.31.1')
        pod = {'metadata': {'namespace': 'poc-run', 'uid': 'pod-uid', 'labels': {'poc-run': 'run', 'poc-role': 'caller-trusted'}},
               'spec': {'nodeName': 'node', 'serviceAccountName': 'caller-trusted', 'containers': [
                   {'name': 'istio-proxy', 'image': pin['immutable_reference'], 'securityContext': {'runAsUser': 1337}}]}}
        for native in (False, True):
            item = copy.deepcopy(pod)
            if native:
                item['spec']['initContainers'] = item['spec'].pop('containers')
            with patch.object(attestation, 'observed', return_value=(item, 'repo@' + pin['platform_manifest_digest'])), patch.object(attestation, 'ensure_entry') as grant:
                identity.register_proxy({'run': 'run', 'node': 'node'}, 'node-parent', 'pod')
            value, parent, sid, selectors = grant.call_args.args
            self.assertEqual(parent, 'node-parent')
            self.assertEqual(sid, 'spiffe://poc.internal/ns/poc-run/sa/caller-trusted')
            for selector in ['unix:uid:1337', 'k8s:container-name:istio-proxy', 'k8s:pod-uid:pod-uid',
                             'k8s:sa:caller-trusted', 'k8s:pod-label:poc-run:run']:
                self.assertIn(selector, selectors)
        for field, value in [('image', 'untrusted'), ('securityContext', {'runAsUser': 1000})]:
            item = copy.deepcopy(pod)
            item['spec']['containers'][0][field] = value
            with patch.object(attestation, 'observed', return_value=(item, 'repo@' + pin['platform_manifest_digest'])), patch.object(attestation, 'ensure_entry') as grant:
                with self.assertRaisesRegex(RuntimeError, 'unexpected proxy'):
                    identity.register_proxy({'run': 'run', 'node': 'node'}, 'node-parent', 'pod')
                grant.assert_not_called()

    def test_shared_mesh_rejects_mismatched_trust_domain_without_writes(self):
        class Target:
            def __init__(self, td):
                self.td, self.calls = td, []

            def kube(self, *args):
                self.calls.append(args)
                return json.dumps({'data': {'mesh': 'trustDomain: ' + self.td}})

        for td in ('cluster.local', 'poc.internal'):
            target = Target(td)
            if td == 'cluster.local':
                with self.assertRaisesRegex(RuntimeError, 'platform configuration'):
                    identity.require_shared_mesh(target)
            else:
                identity.require_shared_mesh(target)
            self.assertEqual(len(target.calls), 1)
            self.assertIn('get', target.calls[0])
