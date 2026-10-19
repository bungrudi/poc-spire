import importlib.util
import json
import os
from pathlib import Path
import socket
import ssl
import tempfile
import threading
import unittest

from attestation import ROOT,run
from kafka_demo import verify_compose

class KafkaInfrastructureTests(unittest.TestCase):
    def test_aws_browser_certificate_matches_assigned_cloudfront_origin(self):
        with tempfile.TemporaryDirectory() as directory:
            spec=importlib.util.spec_from_file_location('fixture_pki',ROOT/'infra/kafka-signing/broker-pki.py')
            module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
            module.generate(directory,'a'*32,browser_dns='d123abc.cloudfront.net')
            cert=ssl._ssl._test_decode_cert(str(Path(directory)/'browser/client.pem'))
            self.assertIn(('DNS','d123abc.cloudfront.net'),cert['subjectAltName'])
            self.assertNotIn(('DNS','localhost'),cert['subjectAltName'])
            with self.assertRaises(ValueError):module.generate(directory,'a'*32,browser_dns='bad.example.com')

    def test_rendered_boundaries_and_temporary_mount_options(self):
        env=os.environ.copy();env.update(POC_PROJECT='poc-kafka-render-test',POC_IMAGE='poc-kafka-render-test:apps',POC_WORK='/tmp/poc-kafka-render-test',POC_SOCKET_DIR='/tmp/poc-kafka-render-test/workload',POC_PORT='18088',POC_ORIGIN='https://127.0.0.1:18088',POC_STREAM='a'*32)
        value=json.loads(run(['docker','compose','-f',str(ROOT/'infra/kafka-signing/compose.yaml'),'--profile','bootstrap','config','--format','json'],env=env).stdout)
        verify_compose(value)
        value['services']['money']['deploy']={'replicas':2}
        with self.assertRaises(RuntimeError):verify_compose(value)
        value['services']['money'].pop('deploy')
        value['services']['kafka']['tmpfs']=['/tmp:uid=1000','gid=1000']
        with self.assertRaises(RuntimeError):verify_compose(value)

    def test_browser_key_has_no_broker_admission_credential(self):
        with tempfile.TemporaryDirectory(prefix='poc-kafka-',dir='/tmp') as directory:
            root=Path(directory);spec=importlib.util.spec_from_file_location('fixture_pki',ROOT/'infra/kafka-signing/broker-pki.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module);module.generate(root,'a'*32)
            self.assertNotEqual((root/'browser/ca.pem').read_bytes(),(root/'broker-pki/ca.pem').read_bytes())
            bad=run(['openssl','verify','-CAfile',str(root/'broker-pki/ca.pem'),'-purpose','sslclient',str(root/'browser/client.pem')],check=False)
            self.assertNotEqual(bad.returncode,0)
            text=run(['openssl','x509','-in',str(root/'browser/client.pem'),'-noout','-text']).stdout
            self.assertIn('TLS Web Server Authentication',text);self.assertNotIn('TLS Web Client Authentication',text)
            server=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);server.minimum_version=ssl.TLSVersion.TLSv1_3;server.verify_mode=ssl.CERT_REQUIRED;server.load_cert_chain(str(root/'broker-pki/broker.pem'));server.load_verify_locations(str(root/'broker-pki/ca.pem'))
            listener=socket.socket();listener.bind(('127.0.0.1',0));listener.listen(2);port=listener.getsockname()[1];accepted=[]
            def accept():
                for _ in range(2):
                    raw,_=listener.accept()
                    try:
                        with server.wrap_socket(raw,server_side=True) as connection:
                            connection.recv(1);connection.sendall(b'ok');accepted.append(True)
                    except ssl.SSLError:raw.close();accepted.append(False)
            worker=threading.Thread(target=accept);worker.start()
            try:
                for directory,expected in [('trusted-pki',True),('browser',False)]:
                    client=ssl.create_default_context(cafile=str(root/'broker-pki/ca.pem'));client.minimum_version=ssl.TLSVersion.TLSv1_3;client.load_cert_chain(str(root/directory/'client.pem'),str(root/directory/'client.key'))
                    valid=False
                    try:
                        with socket.create_connection(('127.0.0.1',port),timeout=2) as raw:
                            with client.wrap_socket(raw,server_hostname='kafka') as connection:connection.sendall(b'x');valid=connection.recv(2)==b'ok'
                    except ssl.SSLError:pass
                    self.assertEqual(valid,expected)
                worker.join(timeout=3);self.assertFalse(worker.is_alive());self.assertEqual(accepted,[True,False])
            finally:listener.close()

if __name__=='__main__':unittest.main()
