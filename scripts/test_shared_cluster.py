import copy
import base64
import hashlib
import importlib.util
import contextlib
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0,str(Path(__file__).resolve().parent))
import shared_cluster
import kubernetes_policy
import kubernetes_application
from shared_target import Target
from kubernetes_images import components
import yaml


class SharedSPIRETests(unittest.TestCase):
    def config(self):
        return {'lab_id':'test-lab','spire':{'run_id':'abcdef123456','image':{'immutable_reference':'registry/app@sha256:'+'a'*64,
            'source_revision':'b'*40,'binary_sha256':{name:'c'*64 for name in components('normal')}}}}

    def test_normal_inventory_is_complete(self):shared_cluster.validate(self.config())
    def test_public_ui_requires_exact_aws_origin_port_and_private_alb_cidrs(self):
        value=self.config();public=value['spire']
        public.update(ui_origin='https://d123abc.cloudfront.net',ui_node_port=30443,alb_cidrs=['10.40.0.0/24'])
        shared_cluster.validate(value)
        for key,bad in [('ui_origin','http://d123abc.cloudfront.net'),('ui_node_port',30444),('alb_cidrs',['0.0.0.0/0'])]:
            trial=copy.deepcopy(value);trial['spire'][key]=bad
            with self.subTest(key=key),self.assertRaises(ValueError):shared_cluster.validate(trial)
        with patch.object(shared_cluster,'checked_target',return_value=type('Target',(),{'config':value})()):
            policies=kubernetes_policy.policies({'run':'abcdef123456','deployment_mode':'shared','config':'fixture'})
        ingress=next(p for p in policies if p['metadata']['name']=='browser-alb-ingress')
        self.assertEqual(ingress['spec']['ingress'][0]['from'],[{'ipBlock':{'cidr':'10.40.0.0/24'}}])
        self.assertEqual(ingress['spec']['podSelector']['matchLabels'],{'poc-role':'ui'})

    def test_public_route_exposes_only_ready_ui_nodeport(self):
        with tempfile.TemporaryDirectory() as directory:
            spec=importlib.util.spec_from_file_location('fixture_pki',Path(__file__).resolve().parents[1]/'infra/kafka-signing/broker-pki.py')
            pki=importlib.util.module_from_spec(spec);spec.loader.exec_module(pki)
            pki.generate(directory,'a'*32,browser_dns='d123abc.cloudfront.net')
            objects=kubernetes_application.app_objects({'run':'abcdef123456'},self.config()['spire']['image'],'a'*32,Path(directory),
                origin='https://d123abc.cloudfront.net',ui_node_port=30443)
        services={o['metadata']['name']:o for o in objects if o['kind']=='Service'}
        self.assertEqual(services['ui']['spec']['type'],'NodePort')
        self.assertEqual(services['ui']['spec']['ports'][0]['nodePort'],30443)
        self.assertFalse(services['ui']['spec']['publishNotReadyAddresses'])
        self.assertTrue(all(o['spec'].get('type')!='NodePort' for n,o in services.items() if n!='ui'))
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

    def test_infrastructure_connectivity_is_owned_and_kubelet_is_single_node(self):
        node={'status':{'addresses':[{'type':'InternalIP','address':'10.80.0.42'}]}}
        policies=shared_cluster.infrastructure_policies(self.config(),node)
        self.assertTrue(all(p['metadata']['namespace']=='spire-abcdef123456' for p in policies))
        self.assertEqual({p['spec']['podSelector']['matchLabels']['poc-role'] for p in policies},{'spire-server','spire-agent'})
        rule=policies[1]['spec']['egress'][-1]
        self.assertEqual(rule,{'to':[{'ipBlock':{'cidr':'10.80.0.42/32'}}],'ports':[{'protocol':'TCP','port':10250}]})
        self.assertEqual(policies[1]['spec']['ingress'],[])
        for addresses in ([],[{'type':'InternalIP','address':'203.0.113.1'}],[{'type':'InternalIP','address':'::1'}],node['status']['addresses']*2):
            with self.assertRaises(RuntimeError):shared_cluster.infrastructure_policies(self.config(),{'status':{'addresses':addresses}})

    def test_telemetry_does_not_open_application_ports(self):
        objects=shared_cluster.telemetry_objects(self.config())
        ingress=next(o for o in objects if o['metadata']['name']=='observability-scrape')
        self.assertEqual(ingress['spec']['ingress'][0]['ports'],[{'protocol':'TCP','port':15020}])
        self.assertTrue(all(o['metadata']['namespace']=='poc-abcdef123456' for o in objects))
    def test_shared_replace_retains_uid_precondition(self):
        value={'config':'unused'}
        target=type('Target',(),{})();target.receipt={'namespaces':{'poc-run':'ns-uid'}}
        target.check=lambda:target
        target.get=lambda kind,*args:{'metadata':{'uid':'ns-uid' if kind=='namespace' else 'original','labels':{'poc-platform/owner':'poc-spire'}}}
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

    def test_nested_deployment_retains_all_exact_cluster_resources_for_removal(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            config=self.config()
            config.update({'schema_version':1,'account_id':'581695527661','region':'ap-southeast-2',
                'runtime_directory':str(root/'runtime'),
                'target':{'context':'explicit','kubeconfig':str(root/'kubeconfig'),
                    'server':'https://test.invalid','ca_sha256':hashlib.sha256(b'test-ca').hexdigest(),'kube_system_uid':'system-uid'}})
            path=root/'target.json';path.write_text(json.dumps(config))
            (root/'kubeconfig').write_text(yaml.safe_dump({'contexts':[{'name':'explicit','context':{'cluster':'test'}}],
                'clusters':[{'name':'test','cluster':{'server':'https://test.invalid',
                    'certificate-authority-data':base64.b64encode(b'test-ca').decode()}}]}))
            target=Target(path,'poc-spire')
            node={'kind':'Node','metadata':{'name':'spire-node','uid':'node-uid',
                'labels':{'poc-platform/lab':'test-lab','poc-platform/workload':'spire','kubernetes.io/arch':'amd64'}},
                'status':{'conditions':[{'type':'Ready','status':'True'}],
                    'addresses':[{'type':'InternalIP','address':'10.80.0.42'}]}}
            live={('node','spire-node',None):node}
            events=[]

            def get(instance,kind,name,namespace=None):
                return copy.deepcopy(live.get((kind.lower(),name,namespace)))

            def store(obj):
                obj=copy.deepcopy(obj);meta=obj['metadata']
                key=(obj['kind'].lower(),meta['name'],meta.get('namespace'))
                meta['uid']=live.get(key,{}).get('metadata',{}).get('uid','uid-'+key[0]+'-'+key[1])
                meta['resourceVersion']='1'
                live[key]=obj

            def kube(instance,*args,data=None,**kwargs):
                if args[:2]==('get','nodes'):
                    return json.dumps({'items':[node]})
                if args==('-n','istio-system','get','configmap','istio','-o','json'):
                    return json.dumps({'data':{'mesh':'trustDomain: poc.internal'}})
                if args[0] in ('create','apply'):
                    for obj in yaml.safe_load_all(data):store(obj)
                    return ''
                if args[0]=='wait':
                    events.append(('wait',args[2]));return ''
                if args[0]=='label':return ''
                raise AssertionError('unexpected Kubernetes call: '+repr(args))

            nested=[('ClusterRole','test-lab-psat'),('ClusterRoleBinding','test-lab-psat'),('CSIDriver','csi.spiffe.io')]

            def nested_spire():
                # Match the real nested adapter calls: independent Target instances
                # persist additions while the outer deploy instance remains alive.
                for kind,name in nested:
                    Target(path,'poc-spire').apply([{'apiVersion':'v1','kind':kind,'metadata':{'name':name}}])

            artifacts=root/'infra/kubernetes-baseline';artifacts.mkdir(parents=True)
            cache=root/'.cache/kubernetes-baseline/artifacts';cache.mkdir(parents=True)
            body=b'isolated CSI fixture'
            (cache/'spiffe-csi-example.yaml').write_bytes(body)
            (artifacts/'artifacts.json').write_text(json.dumps({'upstream_manifests':{'spiffe-csi-example':
                {'sha256':'sha256:'+hashlib.sha256(body).hexdigest(),'url':'https://unused.invalid'}}}))

            with patch.object(shared_cluster,'ROOT',root),patch.object(Target,'check',lambda instance:instance), \
                 patch.object(Target,'get',get),patch.object(Target,'kube',kube), \
                 patch('kubernetes_spire.main',side_effect=nested_spire),patch('kubernetes_policy.main'), \
                 patch('kubernetes_application.main'),patch('kubernetes_finish_bootstrap.main'):
                shared_cluster.deploy(target)
                receipt=json.loads(target.receipt_path.read_text())
                expected={(kind,name,'uid-'+kind.lower()+'-'+name) for kind,name in nested+
                    [('ClusterRole','test-lab-spire-kubelet'),('ClusterRoleBinding','test-lab-spire-kubelet')]}
                self.assertEqual({(r['kind'],r['name'],r['uid']) for r in receipt['cluster_resources']},expected)
                self.assertEqual(receipt['status'],'ready')
                self.assertEqual(set(receipt['namespaces']),{'spire-abcdef123456','poc-abcdef123456'})

                def delete(instance,obj):
                    events.append((obj['kind'],obj['metadata']['name'],obj['metadata']['uid']))

                with patch('shared_target.delete_exact',side_effect=delete):
                    Target(path,'poc-spire').remove()
                self.assertEqual({event for event in events if event[0] in ('ClusterRole','ClusterRoleBinding','CSIDriver')},expected)
                first_resource=next(i for i,event in enumerate(events) if event[0] in ('ClusterRole','ClusterRoleBinding','CSIDriver'))
                self.assertEqual(sum(event[0]=='wait' for event in events[:first_resource]),2)


class CheckedScopeTests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name);self.path=self.root/'target.json';self.kubeconfig=self.root/'kubeconfig'
        self.ca=b'public-test-ca';self.config=SharedSPIRETests().config()
        self.config.update({'schema_version':1,'account_id':'581695527661','region':'ap-southeast-2',
            'runtime_directory':str(self.root/'runtime'),
            'target':{'context':'explicit','kubeconfig':str(self.kubeconfig),'server':'https://test.invalid',
                'ca_sha256':hashlib.sha256(self.ca).hexdigest(),'kube_system_uid':'system-uid'}})
        self.path.write_text(json.dumps(self.config))
        self.local={'contexts':[{'name':'explicit','context':{'cluster':'test'}}],
            'clusters':[{'name':'test','cluster':{'server':'https://test.invalid',
                'certificate-authority-data':base64.b64encode(self.ca).decode()}}]}
        self.kubeconfig.write_text(yaml.safe_dump(self.local))
        self.target=Target(self.path,'poc-spire')
        self.target.receipt={'target':self.config['target'],'namespaces':{'poc-abcdef123456':'ns-uid'},'cluster_resources':[]}
        self.target.save()
        self.work=self.target.runtime/'abcdef123456';self.work.mkdir()
        self.value={'deployment_mode':'shared','run':'abcdef123456','work':str(self.work),'cluster':'test-lab',
            'config':str(self.path),'config_sha256':hashlib.sha256(self.path.read_bytes()).hexdigest()}
        shared_cluster.private_json(shared_cluster.descriptor_path(self.target),self.value)

    def test_scope_reuses_check_in_worker_threads_and_expires_between_operations(self):
        with patch.object(Target,'check',autospec=True,side_effect=lambda target:target) as check, \
             patch.dict(shared_cluster.os.environ,{'POC_SPIRE_SHARED_CONFIG':str(self.path)}):
            with shared_cluster.checked_scope(self.target):
                self.assertEqual(shared_cluster.shared_descriptor(),self.value)
                results=shared_cluster.parallel_reads(lambda _:shared_cluster.checked_target(self.path),range(8))
                self.assertTrue(all(target is self.target for target in results))
                self.assertEqual(check.call_count,1)
            shared_cluster.checked_target(self.path)
            self.assertEqual(check.call_count,2)
            with shared_cluster.checked_scope(self.target):pass
            self.assertEqual(check.call_count,3)

    def test_scope_is_cleared_after_exception(self):
        with patch.object(Target,'check',autospec=True,side_effect=lambda target:target) as check:
            with self.assertRaisesRegex(ValueError,'fixture'):
                with shared_cluster.checked_scope(self.target):raise ValueError('fixture')
            shared_cluster.checked_target(self.path)
            self.assertEqual(check.call_count,2)

    def test_cli_and_imported_runtime_share_the_same_checked_scope(self):
        spec=importlib.util.spec_from_file_location('__main__',shared_cluster.__file__)
        cli=importlib.util.module_from_spec(spec)
        with patch.dict(sys.modules,{'__main__':cli}),patch.object(sys,'argv',['shared_cluster.py','--help']), \
             contextlib.redirect_stdout(io.StringIO()):
            del sys.modules['shared_cluster']
            with self.assertRaises(SystemExit) as result:spec.loader.exec_module(cli)
            self.assertEqual(result.exception.code,0)
            self.assertIs(sys.modules['shared_cluster'],cli)
            with patch.object(Target,'check',return_value=self.target) as check,patch.object(Target,'kube',return_value='{}'), \
                 cli.checked_scope(self.target):
                shared_cluster.runtime.kube(self.value,'get','pods')
                self.assertEqual(check.call_count,1)

    def test_local_config_kubeconfig_ca_and_context_drift_prevent_api_calls(self):
        for change in ('config','endpoint','tls','ca','context','other-field'):
            self.path.write_text(json.dumps(self.config));self.kubeconfig.write_text(yaml.safe_dump(self.local))
            with self.subTest(change=change),patch.object(Target,'check',return_value=self.target), \
                 patch.object(Target,'kube') as api,shared_cluster.checked_scope(self.target):
                local=copy.deepcopy(self.local)
                if change=='config':self.path.write_text(json.dumps({**self.config,'lab_id':'other-lab'}))
                else:
                    cluster=local['clusters'][0]['cluster']
                    if change=='endpoint':cluster['server']='https://wrong.invalid'
                    if change=='tls':cluster['insecure-skip-tls-verify']=True
                    if change=='ca':cluster['certificate-authority-data']=base64.b64encode(b'wrong').decode()
                    if change=='context':local['contexts'][0]['name']='wrong'
                    if change=='other-field':local['users']=[{'name':'changed'}]
                    self.kubeconfig.write_text(yaml.safe_dump(local))
                with self.assertRaises(RuntimeError):shared_cluster.shared_kube(self.value,'get','pods')
                api.assert_not_called()

    def test_referenced_ca_file_drift_is_rejected(self):
        ca=self.root/'ca.pem';ca.write_bytes(self.ca)
        local=copy.deepcopy(self.local);cluster=local['clusters'][0]['cluster']
        cluster.pop('certificate-authority-data');cluster['certificate-authority']='ca.pem'
        self.kubeconfig.write_text(yaml.safe_dump(local))
        with patch.object(Target,'check',return_value=self.target),shared_cluster.checked_scope(self.target):
            ca.write_bytes(b'changed')
            with self.assertRaisesRegex(RuntimeError,'CA'):shared_cluster.checked_target(self.path)

    def test_each_mutation_rechecks_live_namespace_and_refuses_replacement_or_loss(self):
        namespace={'metadata':{'uid':'ns-uid','labels':{'poc-platform/owner':'poc-spire'}}}
        with patch.object(Target,'check',return_value=self.target) as check, \
             patch.object(Target,'get',return_value=namespace) as get,patch.object(Target,'kube') as api, \
             shared_cluster.checked_scope(self.target):
            args=('-n','poc-abcdef123456','patch','configmap','guard','--type=merge','-p','{}')
            shared_cluster.shared_kube(self.value,*args)
            self.assertEqual(api.call_count,1);self.assertEqual(get.call_count,1)
            get.return_value={'metadata':{'uid':'replacement','labels':{'poc-platform/owner':'poc-spire'}}}
            with self.assertRaisesRegex(RuntimeError,'namespace'):shared_cluster.shared_kube(self.value,*args)
            get.return_value=None
            with self.assertRaisesRegex(RuntimeError,'namespace'):shared_cluster.shared_kube(self.value,*args)
            self.assertEqual(api.call_count,1);self.assertEqual(check.call_count,1)

    def test_monitor_validates_once_per_cycle_including_first_startup(self):
        import kubernetes_guard as guard
        shared_cluster.private_json(self.work/'guard-ownership.json',{'run':self.value['run']})
        with patch.dict(shared_cluster.os.environ,{'POC_SPIRE_SHARED_CONFIG':str(self.path)}), \
             patch.object(Target,'check',autospec=True,side_effect=lambda target:target) as check, \
             patch.object(guard,'observe',return_value=None) as observe, \
             patch.object(guard.time,'sleep',side_effect=[None,StopIteration]), \
             patch.object(guard.runtime,'private_json') as health:
            with self.assertRaises(StopIteration):guard.monitor(None,expected_run=self.value['run'])
            self.assertEqual(check.call_count,2);self.assertEqual(observe.call_count,2)
            self.assertEqual(health.call_count,2)
            self.assertTrue(all(call.args[1]['status']=='observing' for call in health.call_args_list))

    def test_stale_health_is_still_rejected_before_verification(self):
        shared_cluster.private_json(self.work/'guard-health.json',{'status':'observing','at':84})
        with patch.object(shared_cluster,'shared_descriptor',return_value=self.value), \
             patch.object(shared_cluster.time,'time',return_value=100),patch.object(self.target,'kube') as api:
            with self.assertRaisesRegex(RuntimeError,'not observing'):shared_cluster.verify(self.target)
            api.assert_not_called()


if __name__=='__main__':unittest.main()
