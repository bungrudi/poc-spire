#!/usr/bin/env python3
"""Narrow actual owner/container registration, stock TLS/ACL Job and complete native readiness."""
import json
import time
from pathlib import Path

import yaml
import kubernetes_demo as runtime
from kubernetes_attestation import admin, observed, register
from attestation import spiffe_ids
import kubernetes_guard as guard_runner


def main():
    value=runtime.descriptor();work=Path(value['work']);ns='poc-'+value['run']
    owned=json.loads((work/'application-ownership.json').read_text());image=json.loads((work/'normal-image.json').read_text())
    node=json.loads(runtime.kube(value,'get','node',value['node'],'-o','json'))
    expected='spiffe://poc.internal/spire/agent/k8s_psat/'+value['cluster']+'/'+node['metadata']['uid']
    agents=json.loads(admin(value,'agent','list','-socketPath','/run/spire/admin/api.sock','-output','json'))['agents']
    if [s for a in agents for s in spiffe_ids(a.get('id',{}))]!=[expected]:raise RuntimeError('actual agent parent differs')
    pods=json.loads(runtime.kube(value,'-n',ns,'get','pods','-o','json'))['items']
    byrole={p['metadata']['labels']['poc-role']:p for p in pods}
    if {r:p['metadata']['uid'] for r,p in byrole.items()}!=owned['pods']:raise RuntimeError('initial Pod ownership differs')
    from kubernetes_mesh_identity import register_proxy, configure
    # Proxies start first; applications may be held behind proxy readiness.
    for pod in byrole.values():
        register_proxy(value, expected, pod['metadata']['name'])
    registrations=[('money','money','spiffe://poc.internal/money','money',1000),('ui','ui','spiffe://poc.internal/ui','ui',1000),
      ('caller-trusted','caller','spiffe://poc.internal/caller/trusted','trusted-caller',1000),
      ('caller-trusted','adapter','spiffe://poc.internal/control/caller/trusted','control-adapter',10002),
      ('caller-unidentified','adapter','spiffe://poc.internal/control/caller/unidentified','control-adapter',10002)]
    for role,container,sid,binary,uid in registrations:
        item,image_id=observed(value,byrole[role]['metadata']['name'],container)
        if item['metadata']['uid']!=owned['pods'][role] or item['metadata']['ownerReferences'][0]['uid']!=owned['owners'][role]:raise RuntimeError('actual owner/Pod changed')
        register(value,expected,sid,item,container,image_id,image['binary_sha256'][binary],uid)
    guard_runner.initial(value)
    kafka=byrole['kafka']['metadata']['name']
    for attempt in range(90):
        pod=json.loads(runtime.kube(value,'-n',ns,'get','pod',kafka,'-o','json'))
        status=next((s for s in pod.get('status',{}).get('containerStatuses',[]) if s['name']=='kafka'),None)
        if status and status.get('ready'):break
        time.sleep(1)
    else:raise RuntimeError('guarded stock broker did not start')
    pins=json.loads((runtime.ROOT/'infra/kubernetes-baseline/artifacts.json').read_text());kafka_image=next(i['immutable_reference'] for i in pins['images'] if i['reference'].startswith('apache/kafka:'))
    job={'apiVersion':'batch/v1','kind':'Job','metadata':{'name':'kafka-admin','namespace':ns},
         'spec':{'backoffLimit':0,'activeDeadlineSeconds':120,'template':{'metadata':{'labels':{'poc-role':'kafka-admin','poc-run':value['run']},
          'annotations':{'sidecar.istio.io/nativeSidecar':'true'}},'spec':{'serviceAccountName':'kafka-admin','automountServiceAccountToken':False,'restartPolicy':'Never',
          'securityContext':{'fsGroup':1000,'seccompProfile':{'type':'RuntimeDefault'}},
          'containers':[{'name':'admin','image':kafka_image,'command':['/bin/bash','/config/acl-bootstrap.sh'],
                         'env':[{'name':'POC_STREAM','value':owned['stream']},{'name':'KAFKA_HEAP_OPTS','value':'-Xms64m -Xmx128m'}],
                         'securityContext':{'runAsUser':1000,'runAsGroup':1000,'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,'capabilities':{'drop':['ALL']}},
                         'resources':{'requests':{'cpu':'50m','memory':'64Mi'},'limits':{'cpu':'750m','memory':'256Mi'}},
                         'volumeMounts':[{'name':'config','mountPath':'/config','readOnly':True},{'name':'pki','mountPath':'/pki','readOnly':True},{'name':'tmp','mountPath':'/tmp'}]}],
          'volumes':[{'name':'config','configMap':{'name':'kafka-config'}},{'name':'pki','secret':{'secretName':'kafka-admin-pki','defaultMode':0o440}},{'name':'tmp','emptyDir':{'medium':'Memory','sizeLimit':'16Mi'}}]}}}}
    configure(job['spec']['template'])
    runtime.apply_csi_workloads(value,'-',data=yaml.safe_dump(job),timeout=60)
    for _ in range(90):
        job_pods=json.loads(runtime.kube(value,'-n',ns,'get','pods','-l','job-name=kafka-admin','-o','json'))['items']
        if len(job_pods)==1:break
        time.sleep(1)
    else:raise RuntimeError('admin Job Pod unavailable')
    register_proxy(value,expected,job_pods[0]['metadata']['name'])
    runtime.kube(value,'-n',ns,'wait','--for=condition=Complete','job/kafka-admin','--timeout=120s',timeout=130)
    runtime.kube(value,'-n',ns,'delete','job','kafka-admin','--wait=true','--timeout=60s',timeout=70)
    runtime.kube(value,'-n',ns,'delete','secret','kafka-admin-pki',timeout=30)
    guard_runner.promote(value)
    for role in ('money','caller-trusted','caller-unidentified','ui'):
        runtime.kube(value,'-n',ns,'rollout','status','deployment/'+role,'--timeout=180s',timeout=190)
    runtime.private_json(work/'full-bootstrap-result.json',{'run':value['run'],'stream':owned['stream'],'complete_native_ui_readiness':True,
                        'broker_admin_job_native_sidecar_completed':True,'admin_job_and_credentials_removed':True,
                        'owner_container_image_sha_parent_conjunction':True,'full_runtime_acceptance':'NOT RUN'})
    print('Complete native dependency readiness reached; functional/security migration acceptance remains required.')


if __name__=='__main__':main()
