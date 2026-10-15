#!/usr/bin/env python3
"""Existing-cluster SPIRE adapter; reuse guarded application and identity implementations."""
import argparse
import base64
import contextlib
from contextvars import ContextVar, copy_context
from concurrent.futures import ThreadPoolExecutor
import hashlib
import http.client
import ipaddress
import json
import os
from pathlib import Path
import re
import select
import ssl
import subprocess
import sys
import time
import urllib.request
import uuid
import yaml

import kubernetes_demo as runtime
from shared_target import Target, immutable, private_json, run

ROOT=Path(__file__).resolve().parents[1]
OWNER='poc-spire'
# Runtime helpers import this module by name even when its CLI is __main__.
if __name__=='__main__':sys.modules.setdefault('shared_cluster',sys.modules[__name__])
_CHECKED_TARGET=ContextVar('spire_checked_target',default=None)


def local_inputs(target):
    """Fingerprint checked local inputs without invoking the AWS credential plugin."""
    config=target.path.read_bytes()
    if json.loads(config)!=target.config:
        raise RuntimeError('shared configuration changed during checked operation')
    kubeconfig=Path(target.base[2]);raw=kubeconfig.read_bytes()
    value=yaml.safe_load(raw)
    contexts=[c['context'] for c in value.get('contexts',[]) if c['name']==target.target['context']]
    if len(contexts)!=1:raise RuntimeError('checked kubeconfig context changed')
    clusters=[c['cluster'] for c in value.get('clusters',[]) if c['name']==contexts[0]['cluster']]
    if len(clusters)!=1:raise RuntimeError('checked kubeconfig cluster changed')
    files={target.path:config,kubeconfig:raw}
    cluster=clusters[0]
    if cluster.get('server')!=target.target['server'] or cluster.get('insecure-skip-tls-verify'):
        raise RuntimeError('checked kubeconfig endpoint or TLS policy changed')
    if 'certificate-authority-data' in cluster:
        ca_bytes=base64.b64decode(cluster['certificate-authority-data'],validate=True)
    else:
        ca=Path(cluster['certificate-authority'])
        if not ca.is_absolute():ca=kubeconfig.parent/ca
        ca_bytes=ca.read_bytes();files[ca]=ca_bytes
    if hashlib.sha256(ca_bytes).hexdigest()!=target.target['ca_sha256']:
        raise RuntimeError('checked kubeconfig CA changed')
    return {str(path):hashlib.sha256(body).hexdigest() for path,body in files.items()}


@contextlib.contextmanager
def checked_scope(target=None, *, checked=False):
    """Reuse one validation only for this lifecycle, stage or observation cycle."""
    target=target or Target(os.environ['POC_SPIRE_SHARED_CONFIG'],OWNER)
    before=local_inputs(target)
    if not checked:target.check()
    if local_inputs(target)!=before:
        raise RuntimeError('local target inputs changed during validation')
    token=_CHECKED_TARGET.set((target,before))
    try:yield target
    finally:_CHECKED_TARGET.reset(token)


def checked_target(config):
    scope=_CHECKED_TARGET.get()
    if scope is None:return Target(config,OWNER).check()
    target,expected=scope
    if Path(config).resolve()!=target.path or local_inputs(target)!=expected:
        raise RuntimeError('local target inputs changed during checked operation')
    return target


def parallel_reads(function, items, *, deferred=False):
    """Only independent reads/probes; propagate the operation scope to each worker."""
    with ThreadPoolExecutor(max_workers=8) as executor:
        futures=[executor.submit(copy_context().run,function,item) for item in items]
    # Keep the original validation order: a later unavailable read must not hide
    # destructive loss already proved by an earlier independent observation.
    return futures if deferred else (future.result() for future in futures)


def check_owned_namespaces(target):
    for name,uid in (target.receipt or {}).get('namespaces',{}).items():
        current=target.get('namespace',name)
        if not current or current['metadata']['uid']!=uid or current['metadata'].get('labels',{}).get('poc-platform/owner')!=OWNER:
            raise RuntimeError('owned namespace was lost or changed before mutation')


def validate(config):
    p=config['spire']
    if not re.fullmatch('[a-f0-9]{12}',p.get('run_id','')):raise ValueError('SPIRE run_id must be twelve hex digits')
    image=p['image']
    if not immutable(image.get('immutable_reference')):raise ValueError('pin SPIRE application image digest')
    from kubernetes_images import components
    if set(image.get('binary_sha256',{}))!=set(components('normal')):
        raise ValueError('exact normal image executable inventory is required')
    if any(not re.fullmatch('[a-f0-9]{64}',s) for s in image['binary_sha256'].values()):raise ValueError('invalid executable digest')
    if not re.fullmatch('[a-f0-9]{40}',image.get('source_revision','')):raise ValueError('pin the image source revision')


def descriptor_path(target):return target.runtime/'descriptor.json'


def shared_descriptor():
    target=checked_target(os.environ['POC_SPIRE_SHARED_CONFIG'])
    value=json.loads(descriptor_path(target).read_text())
    expected=hashlib.sha256(target.path.read_bytes()).hexdigest()
    if value.get('config_sha256')!=expected or value.get('deployment_mode')!='shared' or value.get('run')!=target.config['spire']['run_id']:
        raise RuntimeError('shared descriptor/configuration mismatch')
    if Path(value['work']) != target.runtime/value['run']:raise RuntimeError('shared runtime path mismatch')
    return value


def normalize(objects, target):
    image=target.config['spire']['image']['immutable_reference']
    for obj in objects:
        if not obj:continue
        if obj['kind']=='Pod':spec=obj['spec']
        elif obj['kind'] in ('Deployment','DaemonSet'):spec=obj['spec']['template']['spec']
        elif obj['kind']=='Job':spec=obj['spec']['template']['spec']
        else:continue
        # The identity contract deliberately binds applications to one observed node.
        spec['nodeSelector']={'poc-platform/lab':target.config['lab_id'],'poc-platform/workload':'spire'}
        for c in spec.get('containers',[])+spec.get('initContainers',[]):
            if c.get('image')==image:c['imagePullPolicy']='IfNotPresent'
    return objects


def shared_kube(value,*args,timeout=120,data=None):
    target=checked_target(value['config'])
    args=list(args)
    if args and args[0]=='apply':
        i=args.index('-f');source=args[i+1]
        body=data if source=='-' else Path(source).read_text()
        target.apply(normalize(list(yaml.safe_load_all(body)),target))
        return ''
    mutating=any(a in ('patch','replace','delete','create','label','annotate') for a in args)
    if mutating:
        namespace=args[args.index('-n')+1] if '-n' in args else None
        if args[0]=='replace' and '-f' in args and data:
            obj=json.loads(data)
            namespace=obj['metadata'].get('namespace')
            current=target.get(obj['kind'],obj['metadata']['name'],namespace)
            if not current or current['metadata']['uid']!=obj['metadata'].get('uid'):
                raise RuntimeError('replace target UID changed')
        if namespace not in (target.receipt or {}).get('namespaces',{}):
            raise RuntimeError('mutation outside owned namespace refused')
        check_owned_namespaces(target)
    return target.kube(*args,data=data,timeout=timeout)



def infrastructure_policies(config, node):
    addresses=[a['address'] for a in node.get('status',{}).get('addresses',[]) if a['type']=='InternalIP']
    if len(addresses)!=1:raise RuntimeError('selected SPIRE node requires exactly one InternalIP')
    try:address=ipaddress.IPv4Address(addresses[0])
    except ipaddress.AddressValueError:raise RuntimeError('selected SPIRE node requires a private IPv4 InternalIP') from None
    if address not in ipaddress.IPv4Network('10.80.0.0/16'):raise RuntimeError('selected SPIRE node is outside the lab VPC')
    ns='spire-'+config['spire']['run_id']
    dns={'to':[{'namespaceSelector':{'matchLabels':{'kubernetes.io/metadata.name':'kube-system'}},'podSelector':{'matchLabels':{'k8s-app':'kube-dns'}}}],
         'ports':[{'protocol':'UDP','port':53},{'protocol':'TCP','port':53}]}
    api={'ports':[{'protocol':'TCP','port':443}]}
    def policy(name,role,ingress,egress):
        return {'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':{'name':name,'namespace':ns},
                'spec':{'podSelector':{'matchLabels':{'poc-role':role}},'policyTypes':['Ingress','Egress'],'ingress':ingress,'egress':egress}}
    return [policy('shared-server-connectivity','spire-server',
                   [{'from':[{'podSelector':{'matchLabels':{'poc-role':'spire-agent'}}}], 'ports':[{'protocol':'TCP','port':8081}]}],[dns,api]),
            policy('shared-agent-connectivity','spire-agent',[],[dns,api,
                   {'to':[{'podSelector':{'matchLabels':{'poc-role':'spire-server'}}}], 'ports':[{'protocol':'TCP','port':8081}]},
                   {'to':[{'ipBlock':{'cidr':str(address)+'/32'}}], 'ports':[{'protocol':'TCP','port':10250}]}])]


def base_objects(target,node):
    value=target.config;run_id=value['spire']['run_id'];ns='spire-'+run_id
    name=value['lab_id']+'-spire-kubelet'
    return [*infrastructure_policies(value,target.get('node',node)),
        {'apiVersion':'v1','kind':'ServiceAccount','metadata':{'name':'spire-agent','namespace':ns},'automountServiceAccountToken':False},
        {'apiVersion':'rbac.authorization.k8s.io/v1','kind':'ClusterRole','metadata':{'name':name},
         'rules':[{'apiGroups':[''],'resources':['nodes/pods'],'resourceNames':[node],'verbs':['get']}]},
        {'apiVersion':'rbac.authorization.k8s.io/v1','kind':'ClusterRoleBinding','metadata':{'name':name},
         'subjects':[{'kind':'ServiceAccount','name':'spire-agent','namespace':ns}],
         'roleRef':{'apiGroup':'rbac.authorization.k8s.io','kind':'ClusterRole','name':name}}]


def telemetry_objects(config):
    ns='poc-'+config['spire']['run_id']
    return [
        {'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':{'name':'observability-egress','namespace':ns},
         'spec':{'podSelector':{},'policyTypes':['Egress'],'egress':[{'to':[{'namespaceSelector':{'matchLabels':{'kubernetes.io/metadata.name':'observability'}}}],
                  'ports':[{'protocol':'TCP','port':4317},{'protocol':'TCP','port':4318}]}]}},
        {'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':{'name':'observability-scrape','namespace':ns},
         'spec':{'podSelector':{},'policyTypes':['Ingress'],'ingress':[{'from':[{'namespaceSelector':{'matchLabels':{'kubernetes.io/metadata.name':'observability'}}}],
                  'ports':[{'protocol':'TCP','port':15020}]}]}},
        {'apiVersion':'telemetry.istio.io/v1','kind':'Telemetry','metadata':{'name':'lab-traces','namespace':ns},
         'spec':{'tracing':[{'providers':[{'name':'otel-tracing'}],'randomSamplingPercentage':100}]}}]



def selected_spire_node(nodes, lab_id):
    if len(nodes)!=1:raise RuntimeError('exactly one labelled SPIRE worker is required')
    node=nodes[0];labels=node['metadata']['labels']
    if any(labels.get(k)!=v for k,v in {'kubernetes.io/arch':'amd64','poc-platform/lab':lab_id,'poc-platform/workload':'spire'}.items()):
        raise RuntimeError('SPIRE worker labels differ from the explicit target')
    if not any(x['type']=='Ready' and x['status']=='True' for x in node.get('status',{}).get('conditions',[])):
        raise RuntimeError('selected SPIRE worker is not Ready')
    return node['metadata']['name']


def deploy(target):
    validate(target.config)
    if target.receipt:raise RuntimeError('existing SPIRE run: verify or export/remove before fresh deployment')
    nodes=json.loads(target.kube('get','nodes','-l','poc-platform/lab='+target.config['lab_id']+',poc-platform/workload=spire','-o','json'))['items']
    node=selected_spire_node(nodes,target.config['lab_id']);run_id=target.config['spire']['run_id'];work=target.runtime/run_id
    if work.exists():raise RuntimeError('fresh run required: previous runtime directory exists')
    for kind,name in [('CSIDriver','csi.spiffe.io'),('ClusterRole',target.config['lab_id']+'-psat'),('ClusterRoleBinding',target.config['lab_id']+'-psat'),
                      ('ClusterRole',target.config['lab_id']+'-spire-kubelet'),('ClusterRoleBinding',target.config['lab_id']+'-spire-kubelet')]:
        if target.get(kind,name):raise RuntimeError('foreign SPIRE cluster resource collision: '+kind+'/'+name)
    base=base_objects(target,node)
    target.namespaces(['spire-'+run_id,'poc-'+run_id]);work.mkdir(mode=0o700)
    target.kube('label','namespace','poc-'+run_id,'istio-injection=enabled','--overwrite')
    value={'deployment_mode':'shared','run':run_id,'work':str(work),'cluster':target.config['lab_id'],'node':node,
           'config':str(target.path),'config_sha256':hashlib.sha256(target.path.read_bytes()).hexdigest()}
    private_json(descriptor_path(target),value)
    private_json(work/'normal-image.json',target.config['spire']['image'])
    private_json(work/'bootstrap-image.json',{'tag':target.config['spire']['image']['immutable_reference']})
    pins=json.loads((ROOT/'infra/kubernetes-baseline/artifacts.json').read_text())
    pin=pins['upstream_manifests']['spiffe-csi-example']
    cache=ROOT/'.cache/kubernetes-baseline/artifacts';cache.mkdir(parents=True,exist_ok=True)
    path=cache/'spiffe-csi-example.yaml'
    if not path.exists():path.write_bytes(urllib.request.urlopen(pin['url'],timeout=30).read())
    if 'sha256:'+hashlib.sha256(path.read_bytes()).hexdigest()!=pin['sha256']:raise RuntimeError('CSI checksum mismatch')
    target.apply(base)
    import kubernetes_spire,kubernetes_policy,kubernetes_application,kubernetes_finish_bootstrap
    for stage in (kubernetes_spire.main,kubernetes_policy.main):
        with checked_scope(Target(target.path,OWNER)):stage()
    # Nested stages save their cluster-resource UIDs through separate Targets.
    # Reload before the outer apply saves its inventory again.
    target.receipt=json.loads(target.receipt_path.read_text())
    target.check()
    target.apply(telemetry_objects(target.config))
    for stage in (kubernetes_application.main,kubernetes_finish_bootstrap.main):
        with checked_scope(Target(target.path,OWNER)):stage()
    target.receipt=json.loads(target.receipt_path.read_text())
    target.receipt['status']='ready';target.save()


@contextlib.contextmanager
def forwarded(target,value):
    port=target.config['spire'].get('ui_port',18443)
    process=subprocess.Popen([*target.base,'-n','poc-'+value['run'],'port-forward','service/ui',f'{port}:8443','--address=127.0.0.1'],
                             stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,text=True)
    try:
        ready,_,_=select.select([process.stdout],[],[],15)
        if not ready or 'Forwarding from 127.0.0.1:' not in process.stdout.readline():raise RuntimeError('bounded loopback forwarding failed')
        yield port
    finally:
        process.terminate();process.wait(timeout=10)


def verify(target):
    import kubernetes_guard
    value=shared_descriptor();work=Path(value['work'])
    health=json.loads((work/'guard-health.json').read_text())
    if health['status']!='observing' or time.time()-health['at']>15:raise RuntimeError('SPIRE run guard is not observing')
    for role in ('money','ui','caller-trusted','caller-unidentified','kafka'):
        target.kube('-n','poc-'+value['run'],'rollout','status','deployment/'+role,'--timeout=180s',timeout=190)
    from verify_kubernetes_public import Browser as BaseBrowser
    owned=json.loads((work/'application-ownership.json').read_text())
    with forwarded(target,value) as port:
        class Browser(BaseBrowser):
            def __init__(self):
                self.stream=owned['stream'];self.url=f'https://127.0.0.1:{port}';self.cookie=None;self.csrf=None
                self.context=ssl.create_default_context(cafile=str(work/'app-pki/browser/ca.pem'))
            def request(self,method,path,body=None,headers=None):
                c=http.client.HTTPSConnection('127.0.0.1',port,context=self.context,timeout=15);values=dict(headers or {})
                if self.cookie:values['Cookie']=self.cookie
                if body is not None and not isinstance(body,(str,bytes)):body=json.dumps(body);values['Content-Type']='application/json'
                if method=='POST':values.setdefault('Origin',self.url);values.setdefault('X-CSRF-Token',self.csrf or '')
                try:
                    c.request(method,path,body,values);r=c.getresponse();raw=r.read(65537)
                    if len(raw)>65536:raise RuntimeError('bounded UI response exceeded')
                    if r.getheader('Set-Cookie'):self.cookie=r.getheader('Set-Cookie').split(';',1)[0]
                    return r.status,raw
                finally:c.close()
        browser=Browser();code,session=browser.json('GET','/api/session')
        if code!=200 or 'assumed' not in session['auth_profile']:raise RuntimeError('assumed-operator UI unavailable')
        browser.csrf=session['csrf_token']
        baseline=work/'shared-verification-initial.json'
        code,initial=browser.json('GET','/api/balances')
        if code!=200 or not initial['server_verified']:raise RuntimeError('verified ledger unavailable')
        if baseline.exists():initial=json.loads(baseline.read_text())
        else:private_json(baseline,initial)
        cases=[]
        for scenario,expected in [('trusted','transfer_succeeded'),('unidentified','tls_rejected'),('kafka-trusted','transfer_succeeded'),('kafka-unidentified','money_rejected'),('kafka-tampered','money_rejected')]:
            keypath=work/('verify-'+scenario+'.json')
            if not keypath.exists():private_json(keypath,{'browser_key':uuid.uuid4().hex})
            evidence,_=browser.attempt(scenario,json.loads(keypath.read_text())['browser_key'])
            details=evidence.get('caller_evidence',evidence.get('kafka_evidence',{}))
            if details.get('outcome')!=expected:raise RuntimeError('scenario outcome failed: '+scenario)
            cases.append({'scenario':scenario,'outcome':expected})
        code,final=browser.json('GET','/api/balances')
        if code!=200 or final['version']!=initial['version']+2 or final['balances']['demo-a']!=initial['balances']['demo-a']-200 or final['balances']['demo-b']!=initial['balances']['demo-b']+200:
            raise RuntimeError('ledger conservation/idempotency check failed')
        for headers in [{'X-CSRF-Token':''},{'Origin':'https://wrong.invalid'},{'Host':'wrong.invalid'}]:
            code,_=browser.request('POST','/api/attempts',{'caller':'trusted','browser_key':uuid.uuid4().hex},headers)
            if code!=403:raise RuntimeError('browser boundary did not reject request')
    result={'run':value['run'],'five_scenarios':cases,'ledger_version':final['version'],'browser_denials':3,
            'extended_wrong_executable_and_failure_injection_acceptance':'NOT RUN by this command'}
    private_json(work/'shared-acceptance.json',result)
    return result


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('action',choices=['render','deploy','verify','remove','export']);parser.add_argument('--config',required=True);parser.add_argument('--output')
    args=parser.parse_args();os.environ['POC_SPIRE_SHARED_CONFIG']=str(Path(args.config).resolve())
    target=Target(args.config,OWNER)
    if args.action=='render':
        validate(target.config)
        from kubernetes_policy import policies
        value={'run':target.config['spire']['run_id']}
        objects=policies(value)+telemetry_objects(target.config)
        if args.output:
            Path(args.output).parent.mkdir(parents=True,exist_ok=True)
            Path(args.output).write_text(yaml.safe_dump_all(objects,sort_keys=False))
        print(json.dumps({'policy_objects':len(objects),'identity_and_application_objects':'generated privately during deploy'}));return
    with target.lock(),checked_scope(target,checked=True):
        if args.action=='deploy':deploy(target)
        elif args.action=='verify':print(json.dumps(verify(target)))
        elif args.action=='export':
            result={'component':'poc-spire','target':target.target,'status':(target.receipt or {}).get('status','not-deployed')}
            work=target.runtime/target.config['spire']['run_id']
            for name in ('shared-acceptance.json','guard-health.json','full-bootstrap-result.json'):
                if (work/name).exists():result[name]=json.loads((work/name).read_text())
            private_json(args.output or target.runtime/'evidence.json',result)
        elif args.action=='remove':
            if target.receipt and target.receipt.get('status')=='removed':
                print('SPIRE component is already removed');return
            if descriptor_path(target).exists():value=shared_descriptor()
            else:
                work=target.runtime/target.config['spire']['run_id']
                if (work/'guard-process.json').exists():raise RuntimeError('guard receipt exists without descriptor; explicit recovery required')
                value={'run':target.config['spire']['run_id'],'work':str(work)}
            import kubernetes_guard
            kubernetes_guard.stop(value)
            target.remove()
            import shutil
            if Path(value['work']).is_symlink():raise RuntimeError('refuse aliased private runtime cleanup')
            if Path(value['work']).exists():shutil.rmtree(Path(value['work']))


if __name__=='__main__':
    try:main()
    except (ValueError,RuntimeError,KeyError) as exc:sys.exit(str(exc))
