import base64
import copy
import hashlib
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0,str(Path(__file__).resolve().parent))
from shared_target import Target,immutable


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.path=Path(self.tmp.name)/'target.json';self.ca=b'public-test-ca'
        self.config={'schema_version':1,'lab_id':'test-lab','account_id':'581695527661','region':'ap-southeast-2',
            'runtime_directory':self.tmp.name+'/runtime',
            'target':{'context':'explicit','kubeconfig':self.tmp.name+'/kubeconfig','server':'https://test.invalid',
                'ca_sha256':hashlib.sha256(self.ca).hexdigest(),'kube_system_uid':'system-uid'}}
        self.path.write_text(json.dumps(self.config))
        self.target=Target(self.path,'poc-spire')
        self.cluster={'clusters':[{'cluster':{'server':'https://test.invalid','certificate-authority-data':base64.b64encode(self.ca).decode()}}]}
        self.objects={('namespace','kube-system',None):{'metadata':{'uid':'system-uid'}},
            ('configmap','identity','poc-platform'):{'data':{k:self.config[k] for k in ('lab_id','account_id','region')}}}

    def check(self):
        with patch.object(self.target,'kube',return_value=json.dumps(self.cluster)),patch.object(self.target,'get',side_effect=lambda k,n,namespace=None:self.objects.get((k,n,namespace))):
            return self.target.check()

    def test_accepts_exact_target(self):self.assertIs(self.check(),self.target)
    def test_rejects_endpoint_change(self):
        self.cluster['clusters'][0]['cluster']['server']='https://other.invalid'
        with self.assertRaisesRegex(RuntimeError,'endpoint'):self.check()
    def test_rejects_ca_change(self):
        self.cluster['clusters'][0]['cluster']['certificate-authority-data']=base64.b64encode(b'wrong').decode()
        with self.assertRaisesRegex(RuntimeError,'CA'):self.check()
    def test_rejects_tls_bypass(self):
        self.cluster['clusters'][0]['cluster']['insecure-skip-tls-verify']=True
        with self.assertRaisesRegex(RuntimeError,'bypass'):self.check()
    def test_rejects_recreated_cluster(self):
        self.objects[('namespace','kube-system',None)]['metadata']['uid']='replacement'
        with self.assertRaisesRegex(RuntimeError,'UID'):self.check()
    def test_rejects_account_marker_change(self):
        self.objects[('configmap','identity','poc-platform')]['data']['account_id']='other'
        with self.assertRaisesRegex(RuntimeError,'marker'):self.check()
    def test_explicit_context_is_mandatory(self):
        self.config['target']['context']='';self.path.write_text(json.dumps(self.config))
        with self.assertRaisesRegex(ValueError,'explicit'):Target(self.path,'poc-spire')
    def test_wrong_account_or_region_is_refused(self):
        for field,value in [('account_id','000000000000'),('region','ap-southeast-3')]:
            config=copy.deepcopy(self.config);config[field]=value;self.path.write_text(json.dumps(config))
            with self.assertRaisesRegex(ValueError,'approved AWS'):Target(self.path,'poc-spire')
    def test_foreign_namespace_not_adopted(self):
        with patch.object(self.target,'check'),patch.object(self.target,'get',return_value={'metadata':{'uid':'foreign'}}),patch.object(self.target,'kube') as mutation:
            with self.assertRaisesRegex(RuntimeError,'collision'):self.target.namespaces(['poc-run'])
            mutation.assert_not_called()
    def test_foreign_cluster_resource_blocks_whole_batch(self):
        self.target.receipt={'namespaces':{'poc-run':'uid'},'cluster_resources':[]}
        objects=[{'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'allowed','namespace':'poc-run'}},
            {'apiVersion':'rbac.authorization.k8s.io/v1','kind':'ClusterRole','metadata':{'name':'foreign'}}]
        with patch.object(self.target,'get',return_value={'metadata':{'uid':'foreign'}}),patch.object(self.target,'kube') as mutation:
            with self.assertRaisesRegex(RuntimeError,'collision'):self.target.apply(objects)
            mutation.assert_not_called()
    def test_objects_outside_ownership_block_batch(self):
        self.target.receipt={'namespaces':{'poc-run':'uid'},'cluster_resources':[]}
        obj={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'bad','namespace':'istio-system'}}
        with patch.object(self.target,'kube') as mutation:
            with self.assertRaisesRegex(RuntimeError,'outside'):self.target.apply([obj])
            mutation.assert_not_called()
    def test_stale_namespace_uid_prevents_all_removal(self):
        self.target.receipt={'namespaces':{'poc-run':'old'},'cluster_resources':[]}
        with patch.object(self.target,'check'),patch.object(self.target,'get',return_value={'metadata':{'uid':'new'}}),patch('shared_target.delete_exact') as delete:
            with self.assertRaisesRegex(RuntimeError,'ownership'):self.target.remove()
            delete.assert_not_called()
    def test_other_poc_is_not_in_removal_inventory(self):
        self.target.receipt={'namespaces':{'poc-run':'own'},'cluster_resources':[]}
        obj={'apiVersion':'v1','kind':'Namespace','metadata':{'name':'poc-run','uid':'own','resourceVersion':'2','labels':{'poc-platform/owner':'poc-spire'}}}
        with patch.object(self.target,'check'),patch.object(self.target,'get',return_value=obj),patch.object(self.target,'kube'),patch('shared_target.delete_exact') as delete:
            self.target.remove()
            self.assertEqual(delete.call_count,1)
            self.assertEqual(delete.call_args.args[1]['metadata']['name'],'poc-run')
    def test_digest_must_be_complete(self):
        self.assertTrue(immutable('registry/app@sha256:'+'a'*64))
        for value in ['app:latest','app@sha256:abc','app@sha256:'+'g'*64,None]:self.assertFalse(immutable(value))
    def test_csi_removal_waits_for_namespace_unmounts(self):
        self.target.receipt={'namespaces':{'poc-run':'own'},'cluster_resources':[{'kind':'CSIDriver','name':'csi.spiffe.io','uid':'driver'}]}
        namespace={'apiVersion':'v1','kind':'Namespace','metadata':{'name':'poc-run','uid':'own','resourceVersion':'2','labels':{'poc-platform/owner':'poc-spire'}}}
        driver={'apiVersion':'storage.k8s.io/v1','kind':'CSIDriver','metadata':{'name':'csi.spiffe.io','uid':'driver','resourceVersion':'2','labels':{'poc-platform/owner':'poc-spire'}}}
        events=[]
        with patch.object(self.target,'check'),patch.object(self.target,'get',side_effect=lambda kind,name:namespace if kind=='namespace' else driver),patch.object(self.target,'kube',side_effect=lambda *args,**kwargs:events.append('wait')),patch('shared_target.delete_exact',side_effect=lambda target,obj:events.append(obj['kind'])):
            self.target.remove()
        self.assertEqual(events,['Namespace','wait','CSIDriver'])


if __name__=='__main__':unittest.main()
