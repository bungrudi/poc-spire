#!/usr/bin/env python3
"""Render the complete guarded, hardened, disposable application topology."""
import base64
import importlib.util
import json
import os
import time
import uuid
from pathlib import Path

import yaml
import kubernetes_demo as runtime
from kubernetes_attestation import admin, observed, register
from attestation import spiffe_ids


def resources(memory='96Mi', cpu='25m'):
    return {'requests':{'cpu':cpu,'memory':'16Mi'},'limits':{'cpu':'500m','memory':memory}}


def security(uid=1000):
    return {'runAsUser':uid,'runAsGroup':1000,'readOnlyRootFilesystem':True,'allowPrivilegeEscalation':False,
            'capabilities':{'drop':['ALL']},'seccompProfile':{'type':'RuntimeDefault'}}


def env(value):
    return [{'name':'POC_RUN','value':value['run']},{'name':'POC_POD_UID','valueFrom':{'fieldRef':{'fieldPath':'metadata.uid'}}}]


def secret(ns,name,directory):
    data={p.name:base64.b64encode(p.read_bytes()).decode() for p in directory.iterdir()
          if p.name in ('client.key','client.pem','ca.pem','client.json','broker.pem','browser.pem')}
    return {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':ns},'type':'Opaque','data':data}


def app_objects(value,image,stream,work,origin='https://127.0.0.1:18089'):
    ns='poc-'+value['run'];normal=image['immutable_reference'];pins=json.loads((runtime.ROOT/'infra/kubernetes-baseline/artifacts.json').read_text())
    kafka_image=next(i['immutable_reference'] for i in pins['images'] if i['reference'].startswith('apache/kafka:'))
    items=[]
    for sa in ('money','ui','caller-trusted','caller-unidentified','kafka','kafka-admin'):
        items.append({'apiVersion':'v1','kind':'ServiceAccount','metadata':{'name':sa,'namespace':ns},'automountServiceAccountToken':False})
    items.append({'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'run-guard','namespace':ns,'labels':{'poc-run':value['run']}},
                  'data':{'run.json':json.dumps({'run':value['run'],'phase':'bootstrap','pods':{}})}})
    for role,folder in [('money','money-pki'),('caller-trusted','trusted-pki'),('caller-unidentified','untrusted-pki')]:
        directory=work/folder
        config=json.loads((directory/'client.json').read_text())
        if role.startswith('caller-'):config['state_dir']='/state/private'
        (directory/'client.json').write_text(json.dumps(config))
        items.append(secret(ns,role+'-pki',directory))
    items.extend([secret(ns,'kafka-pki',work/'broker-pki'),secret(ns,'kafka-admin-pki',work/'admin-pki'),secret(ns,'browser-pki',work/'browser')])
    cfg=(runtime.ROOT/'infra/kafka-signing/kafka.properties.tmpl').read_text().replace('1@kafka:9094','1@localhost:9094').replace('CONTROLLER://:9094','CONTROLLER://127.0.0.1:9094')
    cluster_id=base64.urlsafe_b64encode(uuid.uuid4().bytes).decode().rstrip('=')
    broker_script='#!/bin/bash\nset -euo pipefail\n[[ -f /var/lib/kafka/data/meta.properties && -f /var/lib/kafka/data/owned-initialized ]]\nexec /opt/kafka/bin/kafka-server-start.sh /config/kafka.properties\n'
    admin_properties='security.protocol=SSL\nssl.protocol=TLSv1.3\nssl.enabled.protocols=TLSv1.3\nssl.endpoint.identification.algorithm=https\nssl.keystore.type=PEM\nssl.keystore.location=/pki/client.pem\nssl.truststore.type=PEM\nssl.truststore.location=/pki/ca.pem\n'
    items.append({'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'kafka-config','namespace':ns},
                  'data':{'kafka.properties':cfg,'cluster-id':cluster_id,'kubernetes-broker-run.sh':broker_script,
                          'acl-bootstrap.sh':(runtime.ROOT/'infra/kafka-signing/acl-bootstrap.sh').read_text(),'admin.properties':admin_properties}})
    for role in ('money','ui','caller-trusted','caller-unidentified','kafka'):
        labels={'poc-run':value['run'],'poc-role':role}
        volumes=[{'name':'guard','configMap':{'name':'run-guard'}},{'name':'tmp','emptyDir':{'medium':'Memory','sizeLimit':'16Mi'}}]
        containers=[];init=[];annotations={'sidecar.istio.io/nativeSidecar':'false'}
        if role=='kafka':
            volumes += [{'name':'data','emptyDir':{'sizeLimit':'256Mi'}},{'name':'config','configMap':{'name':'kafka-config','defaultMode':0o444}},
                        {'name':'pki','secret':{'secretName':'kafka-pki','defaultMode':0o440}},
                        {'name':'helpers','emptyDir':{'sizeLimit':'48Mi'}},{'name':'logs','emptyDir':{'medium':'Memory','sizeLimit':'16Mi'}}]
            init=[{'name':'helper-install','image':normal,'imagePullPolicy':'Never','command':['/kubernetes-init'],
                   'args':['-install','/helpers'],'securityContext':security(),'resources':resources(),
                   'volumeMounts':[{'name':'helpers','mountPath':'/helpers'}]},
                  {'name':'storage-init','image':kafka_image,'command':['/helpers/kubernetes-init'],
                   'args':['-role','kafka','-run',value['run'],'-state','/var/lib/kafka/data'],'env':env(value)+[{'name':'KAFKA_HEAP_OPTS','value':'-Xms64m -Xmx128m'}],
                   'securityContext':security(),'resources':resources('256Mi'),
                   'volumeMounts':[{'name':'helpers','mountPath':'/helpers','readOnly':True},{'name':'guard','mountPath':'/run/guard','readOnly':True},
                                   {'name':'data','mountPath':'/var/lib/kafka/data'},{'name':'config','mountPath':'/config','readOnly':True},{'name':'pki','mountPath':'/pki','readOnly':True},{'name':'tmp','mountPath':'/tmp'}]}]
            containers=[{'name':'kafka','image':kafka_image,'command':['/helpers/run-guard'],
                         'args':['-role','kafka','--','/bin/bash','/config/kubernetes-broker-run.sh'],'env':env(value)+[{'name':'POC_STREAM','value':stream},{'name':'KAFKA_HEAP_OPTS','value':'-Xms256m -Xmx384m'}],
                         'securityContext':security(),'resources':{'requests':{'cpu':'250m','memory':'384Mi'},'limits':{'cpu':'1000m','memory':'704Mi'}},
                         'startupProbe':{'exec':{'command':['/helpers/kubernetes-broker-ready']},'periodSeconds':3,'timeoutSeconds':3,'failureThreshold':60},
                         'readinessProbe':{'exec':{'command':['/helpers/kubernetes-broker-ready']},'periodSeconds':3,'timeoutSeconds':3,'failureThreshold':2},
                         'volumeMounts':[{'name':'helpers','mountPath':'/helpers','readOnly':True},{'name':'guard','mountPath':'/run/guard','readOnly':True},{'name':'data','mountPath':'/var/lib/kafka/data'},
                                         {'name':'config','mountPath':'/config','readOnly':True},{'name':'pki','mountPath':'/pki','readOnly':True},{'name':'tmp','mountPath':'/tmp'},{'name':'logs','mountPath':'/opt/kafka/logs'}]}]
            annotations['traffic.sidecar.istio.io/excludeOutboundPorts']='9094'
        else:
            volumes += [{'name':'workload','csi':{'driver':'csi.spiffe.io','readOnly':True}}]
            mount=[{'name':'workload','mountPath':'/workload','readOnly':True},{'name':'guard','mountPath':'/run/guard','readOnly':True},{'name':'tmp','mountPath':'/tmp'}]
            binary={'money':'money','ui':'ui','caller-trusted':'trusted-caller','caller-unidentified':'untrusted-caller'}[role]
            args=['-readiness-file','/tmp/readiness.json','-run-guard','/run/guard/run.json','-run',value['run']]
            if role=='ui':
                volumes.append({'name':'browser','secret':{'secretName':'browser-pki','defaultMode':0o440}});mount.append({'name':'browser','mountPath':'/browser','readOnly':True})
                args += ['-mode','demo','-origin',origin,'-listen',':8443','-tls-cert','/browser/browser.pem','-tls-key','/browser/browser.pem',
                         '-kafka-stream',stream,'-control-transport','mtls','-startup-timeout','180s']
            else:
                volumes.append({'name':'pki','secret':{'secretName':role+'-pki','defaultMode':0o440}});mount.append({'name':'pki','mountPath':'/kafka','readOnly':True})
                args += ['-kafka-config','/kafka/client.json']
                if role=='money':args += ['-listen',':8443','-startup-timeout','180s']
                else:
                    volumes += [{'name':'state','emptyDir':{'sizeLimit':'20Mi'}},{'name':'control','emptyDir':{'sizeLimit':'1Mi'}},{'name':'adapter-tmp','emptyDir':{'medium':'Memory','sizeLimit':'16Mi'}}]
                    mount += [{'name':'state','mountPath':'/state'},{'name':'control','mountPath':'/control'}]
                    init=[{'name':'state-init','image':normal,'imagePullPolicy':'Never','command':['/kubernetes-init'],
                           'args':['-role',role,'-run',value['run'],'-state','/state/private','-stream',stream],'env':env(value),
                           'securityContext':security(),'resources':resources(),
                           'volumeMounts':[{'name':'guard','mountPath':'/run/guard','readOnly':True},{'name':'state','mountPath':'/state'}]}]
                    args += ['-operator-uid','10002','-reopen-control']
                    if role=='caller-trusted':args += ['-startup-timeout','180s']
                    else:
                        volumes.append({'name':'bundle','configMap':{'name':'spire-bundle'}});mount.append({'name':'bundle','mountPath':'/trust/spire','readOnly':True})
                        args += ['-public-bundle','/trust/spire/bundle.pem']
                    adapter_role=role.removeprefix('caller-')
                    containers.append({'name':'adapter','image':normal,'imagePullPolicy':'Never','command':['/control-adapter'],
                         'args':['-role',adapter_role,'-run-guard','/run/guard/run.json','-run',value['run'],'-readiness-file','/tmp/readiness.json'],
                         'env':env(value),'securityContext':security(10002),'resources':resources('64Mi'),
                         'readinessProbe':{'exec':{'command':['/kubernetes-ready']},'periodSeconds':2,'timeoutSeconds':1},
                         'volumeMounts':[{'name':'workload','mountPath':'/workload','readOnly':True},{'name':'guard','mountPath':'/run/guard','readOnly':True},
                                         {'name':'control','mountPath':'/control'},{'name':'adapter-tmp','mountPath':'/tmp'}]})
            containers.insert(0,{'name':'caller' if role.startswith('caller-') else role,'image':normal,'imagePullPolicy':'Never','command':['/'+binary],
                                'args':args,'env':env(value),'securityContext':security(),'resources':resources(),
                                'readinessProbe':{'exec':{'command':['/kubernetes-ready']},'periodSeconds':2,'timeoutSeconds':1},'volumeMounts':mount})
        spec={'serviceAccountName':role,'automountServiceAccountToken':False,'securityContext':{'fsGroup':1000,'seccompProfile':{'type':'RuntimeDefault'}},
              'containers':containers,'volumes':volumes,'terminationGracePeriodSeconds':15}
        if init:spec['initContainers']=init
        items.append({'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':role,'namespace':ns},
                      'spec':{'replicas':1,'strategy':{'type':'Recreate'},'selector':{'matchLabels':labels},
                              'template':{'metadata':{'labels':labels,'annotations':annotations},'spec':spec}}})
    for name,role,port in [('money','money',8443),('trusted-control','caller-trusted',9001),('unidentified-control','caller-unidentified',9001),('kafka','kafka',9093),('ui','ui',8443)]:
        items.append({'apiVersion':'v1','kind':'Service','metadata':{'name':name,'namespace':ns},'spec':{'publishNotReadyAddresses':True,
                      'selector':{'poc-role':role},'ports':[{'name':'tcp-'+name,'port':port,'targetPort':port}]}})
    return items


def main():
    value=runtime.descriptor();work=Path(value['work']);ns='poc-'+value['run']
    if (work/'application-started.json').exists():raise RuntimeError('application initialization already started; explicit fresh run required')
    if json.loads(runtime.kube(value,'-n',ns,'get','pods','-o','json'))['items']:raise RuntimeError('refuse existing/foreign application Pod adoption')
    runtime.private_json(work/'application-started.json',{'run':value['run'],'phase':'initialization-started'})
    stream=uuid.uuid4().hex;pkidir=work/'app-pki';pkidir.mkdir(mode=0o700)
    spec=importlib.util.spec_from_file_location('k8s_pki',runtime.ROOT/'infra/kafka-signing/broker-pki.py');pki=importlib.util.module_from_spec(spec);spec.loader.exec_module(pki)
    pki.generate(pkidir,stream,controller_local=True)
    adminpki=pkidir/'admin-pki';(adminpki/'client.pem').write_text((adminpki/'client.key').read_text()+(adminpki/'client.pem').read_text())
    origin='https://127.0.0.1:18089'
    if (work/'public-profile.json').exists():
        from kubernetes_public import provisioned_profile
        profile=provisioned_profile()
        if json.loads((work/'public-profile.json').read_text())!=profile:
            raise RuntimeError('public profile drift')
        origin=profile['origin']
        for path in (pkidir/'browser').iterdir():path.unlink()
        (pkidir/'browser/browser.pem').write_bytes(Path(profile['browser_pem']).read_bytes())
        (pkidir/'browser/browser.pem').chmod(0o600)
    image=json.loads((work/'normal-image.json').read_text());objects=app_objects(value,image,stream,pkidir,origin=origin)
    private_manifest=work/'application-private.yaml';private_manifest.write_text(yaml.safe_dump_all(objects,sort_keys=False));private_manifest.chmod(0o600)
    runtime.apply_csi_workloads(value,str(private_manifest),timeout=120)
    expected_roles={'money','ui','caller-trusted','caller-unidentified','kafka'}
    for _ in range(90):
        pods=json.loads(runtime.kube(value,'-n',ns,'get','pods','-o','json'))['items']
        if {p['metadata']['labels'].get('poc-role') for p in pods}==expected_roles:break
        time.sleep(1)
    else:raise RuntimeError('initial expected Pods unavailable')
    guard={'run':value['run'],'phase':'bootstrap','pods':{p['metadata']['labels']['poc-role']:p['metadata']['uid'] for p in pods}}
    runtime.kube(value,'-n',ns,'patch','configmap','run-guard','--type=merge','-p',json.dumps({'data':{'run.json':json.dumps(guard)}}))
    runtime.private_json(work/'application-ownership.json',{'run':value['run'],'namespace_uid':json.loads(runtime.kube(value,'get','namespace',ns,'-o','json'))['metadata']['uid'],'pods':guard['pods'],
                       'owners':{p['metadata']['labels']['poc-role']:p['metadata']['ownerReferences'][0]['uid'] for p in pods},'stream':stream})
    print('Owned application Pods released after run/Pod UID barrier; registration and broker bootstrap follow.')


if __name__=='__main__':main()
