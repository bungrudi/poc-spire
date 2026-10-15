import copy
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0,str(Path(__file__).resolve().parent))
import shared_cluster
from kubernetes_images import components


class SharedSPIRETests(unittest.TestCase):
    def config(self):
        return {'lab_id':'test-lab','spire':{'run_id':'abcdef123456','image':{'immutable_reference':'registry/app@sha256:'+'a'*64,
            'source_revision':'b'*40,'binary_sha256':{name:'c'*64 for name in components('normal')}}}}

    def test_normal_inventory_is_complete(self):shared_cluster.validate(self.config())
    def test_test_binary_cannot_enter_normal_image(self):
        value=self.config();value['spire']['image']['binary_sha256']['security-client']='d'*64
        with self.assertRaisesRegex(ValueError,'inventory'):shared_cluster.validate(value)
    def test_mutable_image_refused(self):
        value=self.config();value['spire']['image']['immutable_reference']='registry/app:latest'
        with self.assertRaisesRegex(ValueError,'digest'):shared_cluster.validate(value)
    def test_cloud_image_is_pullable_and_node_is_explicit(self):
        value=self.config();target=type('Target',(),{'config':value})()
        obj={'kind':'Pod','spec':{'containers':[{'image':value['spire']['image']['immutable_reference'],'imagePullPolicy':'Never'}]}}
        result=shared_cluster.normalize([obj],target)[0]['spec']
        self.assertEqual(result['containers'][0]['imagePullPolicy'],'IfNotPresent')
        self.assertEqual(result['nodeSelector'],{'poc-platform/lab':'test-lab','poc-platform/workload':'spire'})
    def test_selected_spire_worker_rejects_missing_ambiguous_foreign_and_unready(self):
        node={'metadata':{'name':'spire-node','labels':{'poc-platform/lab':'test-lab','poc-platform/workload':'spire','kubernetes.io/arch':'amd64'}},
              'status':{'conditions':[{'type':'Ready','status':'True'}]}}
        self.assertEqual(shared_cluster.selected_spire_node([node],'test-lab'),'spire-node')
        for nodes in ([],[node,node]):
            with self.assertRaises(RuntimeError):shared_cluster.selected_spire_node(nodes,'test-lab')
        for key,value in (('poc-platform/workload','observability'),('poc-platform/lab','other'),('kubernetes.io/arch','arm64')):
            other=copy.deepcopy(node);other['metadata']['labels'][key]=value
            with self.assertRaises(RuntimeError):shared_cluster.selected_spire_node([other],'test-lab')
        other=copy.deepcopy(node);other['status']['conditions'][0]['status']='False'
        with self.assertRaises(RuntimeError):shared_cluster.selected_spire_node([other],'test-lab')

    def test_telemetry_does_not_open_application_ports(self):
        objects=shared_cluster.telemetry_objects(self.config())
        ingress=next(o for o in objects if o['metadata']['name']=='observability-scrape')
        self.assertEqual(ingress['spec']['ingress'][0]['ports'],[{'protocol':'TCP','port':15020}])
        self.assertTrue(all(o['metadata']['namespace']=='poc-abcdef123456' for o in objects))
    def test_shared_replace_retains_uid_precondition(self):
        value={'config':'unused'}
        target=type('Target',(),{})();target.receipt={'namespaces':{'poc-run':'ns-uid'}}
        target.check=lambda:target
        target.get=lambda *args:{'metadata':{'uid':'original'}}
        calls=[];target.kube=lambda *args,**kwargs:calls.append((args,kwargs)) or ''
        data={'kind':'ConfigMap','metadata':{'name':'guard','namespace':'poc-run','uid':'original','resourceVersion':'7'}}
        with patch('shared_cluster.Target',return_value=target):
            shared_cluster.shared_kube(value,'replace','-f','-',data=json.dumps(data))
        self.assertEqual(json.loads(calls[0][1]['data'])['metadata']['resourceVersion'],'7')
    def test_foreign_replace_is_refused(self):
        target=type('Target',(),{})();target.receipt={'namespaces':{'poc-run':'ns-uid'}};target.check=lambda:target
        target.get=lambda *args:{'metadata':{'uid':'replacement'}}
        with patch('shared_cluster.Target',return_value=target):
            with self.assertRaisesRegex(RuntimeError,'UID'):
                shared_cluster.shared_kube({'config':'unused'},'replace','-f','-',data=json.dumps({'kind':'ConfigMap','metadata':{'name':'guard','namespace':'poc-run','uid':'original'}}))


if __name__=='__main__':unittest.main()
