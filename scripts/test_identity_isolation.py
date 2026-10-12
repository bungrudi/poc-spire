import fcntl
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import uuid

import identity


class IdentityIsolationTests(unittest.TestCase):
    def root(self):
        directory=tempfile.TemporaryDirectory(prefix='poc-kafka-',dir='/tmp')
        self.addCleanup(directory.cleanup)
        root=Path(directory.name);root.chmod(0o700)
        identity.private_write(root/'isolated-runtime.json',json.dumps({'version':1,'purpose':'poc-spire-kafka','ephemeral':True,'root':str(root),'instance':uuid.uuid4().hex}))
        return root

    def test_isolated_mode_requires_initialized_private_marker(self):
        root=self.root();(root/'isolated-runtime.json').unlink()
        with self.assertRaises((RuntimeError,FileNotFoundError)):
            identity.IdentityRuntime(ephemeral_root=root,isolated=True)
        self.assertFalse((root/'identity').exists())

    def test_isolated_mode_rejects_alias_and_socket_collision(self):
        root=self.root();alias=root.parent/('poc-kafka-alias-'+uuid.uuid4().hex)
        alias.symlink_to(root);self.addCleanup(alias.unlink)
        with self.assertRaises(RuntimeError):identity.IdentityRuntime(ephemeral_root=alias,isolated=True)
        (root/'workload').symlink_to(root)
        with self.assertRaises(RuntimeError):identity.IdentityRuntime(ephemeral_root=root,isolated=True)

    def test_busy_own_lock_is_never_bypassed(self):
        root=self.root();runtime=identity.IdentityRuntime(ephemeral_root=root,isolated=True)
        self.addCleanup(runtime.close)
        with self.assertRaises(BlockingIOError):identity.IdentityRuntime(ephemeral_root=root,isolated=True)

    def test_default_guard_stays_global_and_isolation_coexists(self):
        with tempfile.TemporaryDirectory(prefix='identity-policy-') as directory:
            project=Path(directory);(project/'.runtime').mkdir();temporary=project/'.runtime/backend-default';temporary.mkdir(mode=0o700)
            with patch.object(identity,'ROOT',project):
                first=identity.IdentityRuntime(ephemeral_root=temporary)
                try:
                    with self.assertRaises(BlockingIOError):identity.IdentityRuntime()
                    isolated=identity.IdentityRuntime(ephemeral_root=self.root(),isolated=True)
                    isolated.close()
                finally:first.close()
                self.assertTrue((project/'.runtime/identity/runtime.lock').exists())

    def test_existing_state_cannot_be_adopted_by_another_run(self):
        root=self.root();state=root/'identity';state.mkdir(mode=0o700)
        identity.private_write(state/'runtime.json',json.dumps({'isolated_instance':'different'}))
        with self.assertRaises(RuntimeError):identity.IdentityRuntime(ephemeral_root=root,isolated=True)
        # Constructor failure released its own lock.
        with (root/'runtime.lock').open('a') as file:fcntl.flock(file,fcntl.LOCK_EX|fcntl.LOCK_NB)

    def test_isolated_mode_cannot_change_retained_policy(self):
        with self.assertRaises(RuntimeError):identity.IdentityRuntime(isolated=True)
        with self.assertRaises(RuntimeError):identity.IdentityRuntime(ca_ttl=600)
        root=self.root();(root/'isolated-runtime.json').chmod(0o644)
        with self.assertRaises(RuntimeError):identity.IdentityRuntime(ephemeral_root=root,isolated=True)

if __name__=='__main__':unittest.main()
