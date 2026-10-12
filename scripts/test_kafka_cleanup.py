import subprocess
import tempfile
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import Mock,patch

from kafka_demo import KafkaStack

class KafkaCleanupTests(unittest.TestCase):
    def stack(self):
        root=Path(tempfile.mkdtemp(prefix='poc-kafka-',dir='/tmp'))
        self.addCleanup(lambda:__import__('shutil').rmtree(root,ignore_errors=True))
        stack=KafkaStack.__new__(KafkaStack)
        stack.work=root;stack.compose=['docker','compose','-p','owned-project'];stack.env={};stack.image='owned-image';stack.closed=False;stack.up=True;stack.built=True;stack.runtime=Mock()
        return stack

    def test_down_timeout_attempts_later_cleanup_and_can_retry(self):
        stack=self.stack()
        with patch('kafka_demo.run',side_effect=[subprocess.TimeoutExpired('docker',60),SimpleNamespace(returncode=0),SimpleNamespace(returncode=0)]) as run:
            first=stack.close()
            self.assertFalse(first['owned_cleanup_complete']);self.assertTrue(stack.work.exists())
            stack.runtime is None
            second=stack.close()
            self.assertTrue(second['owned_cleanup_complete']);self.assertFalse(stack.work.exists())
            self.assertEqual(run.call_count,3)
            self.assertEqual(run.call_args_list[0].args[0][:4],['docker','compose','-p','owned-project'])

    def test_nonzero_down_does_not_claim_success(self):
        stack=self.stack()
        with patch('kafka_demo.run',side_effect=[SimpleNamespace(returncode=1),SimpleNamespace(returncode=0),SimpleNamespace(returncode=1)]):
            self.assertFalse(stack.close()['owned_cleanup_complete'])
            self.assertFalse(stack.close()['owned_cleanup_complete'])
            self.assertTrue(stack.work.exists())

    def test_native_failure_still_attempts_image_removal_and_retains_retry_state(self):
        stack=self.stack();runtime=stack.runtime;runtime.close.side_effect=[RuntimeError('native failure'),None]
        with patch('kafka_demo.run',side_effect=[SimpleNamespace(returncode=0),SimpleNamespace(returncode=0)]) as run:
            first=stack.close();self.assertFalse(first['owned_cleanup_complete']);self.assertTrue(stack.work.exists())
            self.assertEqual(run.call_count,2);self.assertIs(stack.runtime,runtime)
            self.assertTrue(stack.close()['owned_cleanup_complete']);self.assertEqual(runtime.close.call_count,2)

    def test_successful_second_close_reports_actual_complete_state(self):
        stack=self.stack()
        with patch('kafka_demo.run',return_value=SimpleNamespace(returncode=0)) as run:
            self.assertTrue(stack.close()['owned_cleanup_complete']);self.assertTrue(stack.close()['owned_cleanup_complete']);self.assertEqual(run.call_count,2)

if __name__=='__main__':unittest.main()

class CleanupPropagationTests(unittest.TestCase):
    def test_native_failure_is_not_hidden_by_zero_docker_codes(self):
        from kafka_demo import require_complete_cleanup
        with self.assertRaises(RuntimeError):require_complete_cleanup({'owned_cleanup_complete':False,'errors':[{'action':'native-runtime'}],'compose_exit_code':0,'image_exit_code':0,'ephemeral_state_removed':False})

    def test_smoke_cannot_save_pass_when_cleanup_failed(self):
        import json
        import kafka_smoke
        class Stack:
            stage='ready'
            work=Path('/fixture')
            image_digest='fixture'
            binary_hashes={}
            def start(self):pass
            def attempt(self,scenario):
                if scenario.startswith('kafka-'):return {'status':'done','kafka_evidence':{'outcome':'transfer_succeeded' if scenario=='kafka-trusted' else 'money_rejected','money_queries':[]}},'key'
                return {'status':'done','caller_evidence':{'outcome':'transfer_succeeded' if scenario=='trusted' else 'tls_rejected'}},'key'
            def close(self):return {'owned_cleanup_complete':False,'errors':[{'action':'native-runtime'}],'compose_exit_code':0,'image_exit_code':0,'ephemeral_state_removed':False}
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);(root/'docs/evidence').mkdir(parents=True)
            with patch.object(kafka_smoke,'ROOT',root),patch.object(kafka_smoke,'KafkaStack',Stack),patch.object(kafka_smoke,'run',return_value=SimpleNamespace(stdout='',stderr='flag provided but not defined',returncode=2)):
                with self.assertRaises(RuntimeError):kafka_smoke.main()
            report=json.loads((root/'docs/evidence/kafka-smoke-current.json').read_text())
            self.assertEqual(report['status'],'failed_cleanup')
