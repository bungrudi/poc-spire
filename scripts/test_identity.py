import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from identity import BundlePublisher, IdentityRuntime, private_write


class BundlePublisherTest(unittest.TestCase):
    def test_failed_or_malformed_refresh_preserves_last_bundle(self):
        with tempfile.TemporaryDirectory() as root:
            private, public = Path(root) / 'private.pem', Path(root) / 'public.pem'
            private.write_text('last verified bundle'); public.write_text('last verified bundle')
            publisher = BundlePublisher(private, public, lambda: 'malformed PEM')
            with self.assertRaises(Exception): publisher.refresh()
            self.assertEqual(public.read_text(), 'last verified bundle')
            self.assertEqual(private.read_text(), 'last verified bundle')
            publisher.fetch = lambda: (_ for _ in ()).throw(subprocess.TimeoutExpired(['secret-arg'], 3))
            with self.assertRaises(subprocess.TimeoutExpired): publisher.refresh()
            self.assertEqual(public.read_text(), 'last verified bundle')

    def test_public_permissions_are_set_before_atomic_replace(self):
        with tempfile.TemporaryDirectory() as root:
            public = Path(root) / 'bundle.pem'
            private_write(public, 'public fixture', mode=0o444)
            self.assertEqual(public.stat().st_mode & 0o777, 0o444)
            private_write(public, 'complete replacement', mode=0o444)
            self.assertEqual(public.read_text(), 'complete replacement')
            self.assertEqual(public.stat().st_mode & 0o777, 0o444)
            self.assertFalse(public.with_suffix('.pem.new').exists())

    def test_test_option_cannot_change_retained_ca_policy(self):
        with patch('identity.os.getuid', return_value=1000), patch('identity.os.getgid', return_value=1000):
            with self.assertRaisesRegex(RuntimeError, 'Retained PoC CA policy'):
                IdentityRuntime(ca_ttl=120)



class RegistrationPolicyTest(unittest.TestCase):
    def runtime_and_entry(self, root):
        import hashlib
        runtime=object.__new__(IdentityRuntime)
        runtime.server='fixture-server';runtime.server_socket='/fixture/admin.sock'
        runtime.metadata={'parent':'spiffe://poc.internal/test/parent'};runtime.created_entries=[]
        binary=Path(root)/'binary';binary.write_bytes(b'fixture executable')
        entry={'id':'existing-user-entry','spiffe_id':{'trust_domain':'poc.internal','path':'/caller/trusted'},
               'parent_id':{'trust_domain':'poc.internal','path':'/test/parent'},'x509_svid_ttl':90,
               'selectors':[{'type':'unix','value':'uid:1000'},
                            {'type':'unix','value':'sha256:'+hashlib.sha256(binary.read_bytes()).hexdigest()},
                            {'type':'docker','value':'label:poc-spire.role:fixture'},
                            {'type':'docker','value':'image_config_digest:sha256:fixture'}]}
        return runtime,binary,entry

    def test_broader_parent_duplicate_and_ttl_policy_cannot_be_silently_reused(self):
        import copy,json
        from types import SimpleNamespace
        for case in ['broader','parent','duplicate','ttl']:
            with self.subTest(case=case),tempfile.TemporaryDirectory() as root:
                runtime,binary,entry=self.runtime_and_entry(root)
                entries=[copy.deepcopy(entry)]
                if case=='broader':entries[0]['selectors']=entries[0]['selectors'][:1]
                if case=='parent':entries[0]['parent_id']['path']='/wrong-parent'
                if case=='duplicate':entries.append(copy.deepcopy(entry))
                if case=='ttl':entries[0]['x509_svid_ttl']=3600
                with patch('identity.run',return_value=SimpleNamespace(stdout=json.dumps({'entries':entries}))) as command:
                    with self.assertRaises(RuntimeError):runtime.register('spiffe://poc.internal/caller/trusted',binary,'sha256:fixture','fixture',ttl=90)
                    self.assertEqual(command.call_count,1) # Read only; no update/create/delete.
                    self.assertEqual(runtime.created_entries,[])

    def test_exact_existing_entry_is_reused_and_not_owned_by_test_cleanup(self):
        import json
        from types import SimpleNamespace
        with tempfile.TemporaryDirectory() as root:
            runtime,binary,entry=self.runtime_and_entry(root)
            with patch('identity.run',return_value=SimpleNamespace(stdout=json.dumps({'entries':[entry]}))) as command:
                self.assertEqual(runtime.register('spiffe://poc.internal/caller/trusted',binary,'sha256:fixture','fixture',ttl=90),'existing-user-entry')
                self.assertEqual(command.call_count,1)
                self.assertEqual(runtime.created_entries,[])

if __name__ == '__main__': unittest.main()
