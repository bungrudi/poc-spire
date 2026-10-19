import json
import subprocess
import tempfile
import threading
import unittest
from pathlib import Path
from unittest.mock import patch

import kubernetes_guard as guard


class GuardTests(unittest.TestCase):
    def test_shared_observer_waits_for_real_health_after_local_window(self):
        with tempfile.TemporaryDirectory() as directory:
            work=Path(directory)
            class Child:
                def poll(self):
                    return None
            calls=[]
            def advance(_):
                calls.append(1)
                if len(calls)==48:
                    (work/'guard-health.json').write_text(json.dumps({'status':'observing'}))
            with patch.object(guard.time,'sleep',side_effect=advance):
                guard.wait_for_observer({'deployment_mode':'shared'},work,Child())
            self.assertEqual(len(calls),48)
            (work/'guard-health.json').unlink()
            calls.clear()
            with patch.object(guard.time,'sleep',side_effect=advance):
                with self.assertRaisesRegex(RuntimeError,'readiness unavailable'):
                    guard.wait_for_observer({'deployment_mode':'local'},work,Child())
            self.assertEqual(len(calls),40)

    def fixture(self):
        run='abcdef012345';ns='poc-'+run;sns='spire-'+run
        roles=['money','ui','caller-trusted','caller-unidentified','kafka']
        baseline={'run':run,'namespace_uid':'namespace','spire_namespace_uid':'spire-ns',
                  'server_pod_uid':'issuer','pods':{r:r+'-uid' for r in roles},'owners':{r:r+'-owner' for r in roles},
                  'stream':'a'*32,'image_ids':{r:'image@sha256:actual' for r in ['caller-trusted','caller-unidentified','kafka','spire-server']},'states':{r:'a'*64 for r in ['caller-trusted','caller-unidentified','kafka','spire-server']},
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
        pod={'metadata':{'name':'caller'},'status':{'containerStatuses':[{'name':'caller','state':{'running':{'startedAt':'fixture'}}}]}}
        completed=subprocess.CompletedProcess([],21,b'{"valid":false,"unavailable":true}',b'')
        with patch.object(guard.subprocess,'run',return_value=completed):
            self.assertIsNone(guard.probe(value,'caller-trusted',pod,'a'*32))

    def test_shared_observation_runs_all_independent_reads_and_probes_concurrently(self):
        value,baseline,_,_,get=self.fixture();value['deployment_mode']='shared'
        reads=threading.Barrier(8,timeout=3);probes=threading.Barrier(4,timeout=3)
        seen_reads=[];seen_probes=[]
        def read(value,namespace,kind,name=None):
            seen_reads.append((namespace,kind,name));reads.wait()
            return get(value,namespace,kind,name)
        def probe(value,role,*args):
            self.assertEqual(len(seen_reads),8)
            seen_probes.append(role);probes.wait()
            return {'valid':True,'incarnation':'a'*64}
        with patch.object(guard,'get',side_effect=read),patch.object(guard,'probe',side_effect=probe):
            self.assertIsNone(guard.observe(value,baseline))
        self.assertEqual(len(set(seen_reads)),8)
        self.assertEqual(set(seen_probes),set(baseline['states']))

    def test_shared_observation_rejects_replacement_before_any_state_probe(self):
        value,baseline,pods,_,get=self.fixture();value['deployment_mode']='shared'
        pods[0]['metadata']['uid']='replacement'
        with patch.object(guard,'get',side_effect=get),patch.object(guard,'probe') as probe:
            self.assertEqual(guard.observe(value,baseline),'original_application_pod_lost_or_replaced')
            probe.assert_not_called()

    def test_shared_observation_api_failure_does_not_become_success_or_destructive_loss(self):
        value,baseline,_,_,get=self.fixture();value['deployment_mode']='shared'
        def read(value,namespace,kind,name=None):
            if kind=='namespace':raise RuntimeError('API unavailable')
            return get(value,namespace,kind,name)
        with patch.object(guard,'get',side_effect=read),patch.object(guard,'probe') as probe:
            with self.assertRaisesRegex(RuntimeError,'API unavailable'):guard.observe(value,baseline)
            probe.assert_not_called()

    def test_shared_known_loss_is_not_hidden_by_later_read_or_probe_failure(self):
        value,baseline,_,_,get=self.fixture();value['deployment_mode']='shared'
        def read(value,namespace,kind,name=None):
            if kind=='namespace':return None
            if name=='spire-bundle':raise RuntimeError('later API failure')
            return get(value,namespace,kind,name)
        with patch.object(guard,'get',side_effect=read),patch.object(guard,'probe') as probe:
            self.assertEqual(guard.observe(value,baseline),'original_namespace_lost_or_replaced')
            probe.assert_not_called()
        def state(value,role,*args):
            if role=='caller-trusted':return {'valid':False}
            raise RuntimeError('later probe failure')
        with patch.object(guard,'get',side_effect=get),patch.object(guard,'probe',side_effect=state):
            self.assertEqual(guard.observe(value,baseline),'initialized_caller-trusted_state_lost_corrupt_or_replaced')

    def test_shared_probe_keeps_five_second_budget_and_reports_unavailability(self):
        import shared_cluster
        value,_,_,_,_=self.fixture();value.update(deployment_mode='shared',config='fixture',work='/tmp/owned',cluster='owned')
        pod={'metadata':{'name':'caller'},'status':{'containerStatuses':[{'name':'caller','state':{'running':{'startedAt':'fixture'}}}]}}
        target=type('Target',(),{'base':['kubectl','--kubeconfig','dedicated','--context','explicit']})()
        with patch.object(shared_cluster,'checked_target',return_value=target), \
             patch.object(guard.subprocess,'run',side_effect=subprocess.TimeoutExpired('exec',5)) as execute:
            self.assertIsNone(guard.probe(value,'caller-trusted',pod,'a'*32))
            self.assertEqual(execute.call_args.kwargs['timeout'],5)
            self.assertEqual(execute.call_args.args[0][:5],target.base)



class StartupTerminationTests(unittest.TestCase):
    def fixture(self):
        value={'run':'abcdef012345','work':'/tmp/owned','cluster':'owned'}
        image='repo@sha256:actual'
        pod={'metadata':{'name':'kafka','uid':'original'},'status':{'containerStatuses':[{'name':'kafka','imageID':image,
             'state':{'waiting':{'reason':'CrashLoopBackOff'}},'lastState':{'terminated':{'exitCode':78,
             'message':'poc-spire:destructive-startup-state:kafka:abcdef012345:original'}}}]}}
        return value,image,pod

    def test_exact_owned_source_state_failure_is_detected_even_in_crashloop(self):
        value,image,pod=self.fixture()
        with patch.object(guard.subprocess,'run') as execute:
            self.assertEqual(guard.probe(value,'kafka',pod,'a'*32,True,image),{'valid':False})
            execute.assert_not_called()
        status=pod['status']['containerStatuses'][0]
        status['state']={'running':{}}
        self.assertTrue(guard.state_loss_termination(value,'kafka',pod,status,image),'last destructive startup must not be forgotten after repair')

    def test_ordinary_outage_wrong_message_or_changed_image_has_no_state_loss_signal(self):
        for alteration in ('ordinary-exit','missing-message','wrong-pod','wrong-run','wrong-image'):
            value,image,pod=self.fixture();status=pod['status']['containerStatuses'][0];state=status['lastState']['terminated']
            if alteration=='ordinary-exit':state['exitCode']=137
            if alteration=='missing-message':state.pop('message')
            if alteration=='wrong-pod':state['message']=state['message'].replace('original','foreign')
            if alteration=='wrong-run':state['message']=state['message'].replace('abcdef012345','foreign')
            if alteration=='wrong-image':status['imageID']='repo@sha256:foreign'
            with self.subTest(alteration=alteration),patch.object(guard.subprocess,'run') as execute:
                self.assertIsNone(guard.probe(value,'kafka',pod,'a'*32,True,image))
                execute.assert_not_called()


if __name__ == '__main__':
    unittest.main()
