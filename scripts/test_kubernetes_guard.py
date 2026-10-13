import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import kubernetes_guard as guard


class GuardTests(unittest.TestCase):
    def fixture(self):
        run='abcdef012345';ns='poc-'+run;sns='spire-'+run
        roles=['money','ui','caller-trusted','caller-unidentified','kafka']
        baseline={'run':run,'namespace_uid':'namespace','spire_namespace_uid':'spire-ns',
                  'server_pod_uid':'issuer','pods':{r:r+'-uid' for r in roles},'owners':{r:r+'-owner' for r in roles},
                  'stream':'a'*32,'states':{r:'a'*64 for r in ['caller-trusted','caller-unidentified','kafka','spire-server']},
                  'guard_uids':{ns:'app-guard',sns:'spire-guard'},'bundle_uids':{ns:'app-bundle',sns:'spire-bundle'}}
        pods=[{'metadata':{'name':r,'uid':r+'-uid','resourceVersion':'7','labels':{'poc-role':r},
                           'ownerReferences':[{'kind':'ReplicaSet','uid':r+'-owner'}]}} for r in roles]
        issuer={'metadata':{'name':'spire-server','uid':'issuer','resourceVersion':'7'}}
        cms={n:{'metadata':{'uid':uid,'resourceVersion':'8','namespace':n,'name':'run-guard'},
                'data':{'run.json':json.dumps({'run':run,'phase':'running','pods':baseline['pods']})}}
             for n,uid in baseline['guard_uids'].items()}
        def get(value,namespace,kind,name=None):
            if kind=='namespace':return {'metadata':{'uid':'namespace' if namespace==ns else 'spire-ns'}}
            if kind=='pods':return {'items':pods if namespace==ns else [issuer]}
            if kind=='pod':return issuer
            if kind=='configmap':return cms[namespace] if name=='run-guard' else {'metadata':{'uid':baseline['bundle_uids'][namespace]}}
            raise AssertionError(kind)
        return {'run':run},baseline,pods,cms,get

    def test_issuer_outage_busy_or_unavailable_probe_is_not_destructive(self):
        value,baseline,_,_,get=self.fixture()
        def probe(value,role,*args):
            return None if role in ('spire-server','caller-trusted') else {'valid':True,'incarnation':'a'*64}
        with patch.object(guard,'get',side_effect=get),patch.object(guard,'probe',side_effect=probe):
            self.assertIsNone(guard.observe(value,baseline))

    def test_observed_initialized_state_loss_is_destructive(self):
        value,baseline,_,_,get=self.fixture()
        with patch.object(guard,'get',side_effect=get),patch.object(guard,'probe',return_value={'valid':False}):
            self.assertEqual(guard.observe(value,baseline),'initialized_caller-trusted_state_lost_corrupt_or_replaced')
        with patch.object(guard,'get',side_effect=get),patch.object(guard,'probe',return_value={'valid':True,'incarnation':'b'*64}):
            self.assertEqual(guard.observe(value,baseline),'initialized_caller-trusted_state_lost_corrupt_or_replaced')

    def test_replacement_pod_or_owner_is_not_adopted(self):
        _,baseline,pods,_,_=self.fixture()
        pods[0]['metadata']['uid']='replacement'
        self.assertEqual(guard.validate_pods(baseline,pods)[0],'original_application_pod_lost_or_replaced')
        pods[0]['metadata']['uid']=baseline['pods']['money']
        pods[0]['metadata']['ownerReferences'][0]['uid']='replacement-owner'
        self.assertEqual(guard.validate_pods(baseline,pods)[0],'original_application_owner_changed')

    def test_invalidity_is_monotonic_and_guard_writes_have_uid_version_preconditions(self):
        value,baseline,_,cms,get=self.fixture()
        with tempfile.TemporaryDirectory() as directory:
            value['work']=directory
            with patch.object(guard,'get',side_effect=get),patch.object(guard.runtime,'kube') as kube:
                guard.invalidate(value,baseline,'observed_loss')
                guard.invalidate(value,baseline,'resources_reappeared')
            record=json.loads((Path(directory)/'run-invalid.json').read_text())
            self.assertEqual(record['reason'],'observed_loss')
            writes=[json.loads(call.kwargs['data']) for call in kube.call_args_list if call.args[1]=='replace']
            self.assertEqual(len(writes),2)
            self.assertEqual({x['metadata']['uid'] for x in writes},set(baseline['guard_uids'].values()))
            self.assertTrue(all(x['metadata']['resourceVersion']=='8' and json.loads(x['data']['run.json'])['phase']=='invalid' for x in writes))
            guard.runtime.private_json(Path(directory)/'guard-ownership.json',baseline)
            guard.runtime.private_json(Path(directory)/'guard-process.json',{'pid':1})
            with patch.object(guard.runtime,'kube') as kube:
                with self.assertRaisesRegex(RuntimeError,'cannot be promoted'):guard.promote(value)
                kube.assert_not_called()

    def test_busy_exec_has_no_destructive_result(self):
        value,_,_,_,_=self.fixture();value.update(work='/tmp/owned',cluster='owned')
        pod={'metadata':{'name':'caller'},'status':{'containerStatuses':[{'name':'caller','state':{'running':{}}}]}}
        completed=subprocess.CompletedProcess([],21,b'{"valid":false,"unavailable":true}',b'')
        with patch.object(guard.subprocess,'run',return_value=completed):
            self.assertIsNone(guard.probe(value,'caller-trusted',pod,'a'*32))


if __name__=='__main__':unittest.main()
