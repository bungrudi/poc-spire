#!/usr/bin/env python3
"""Opt-in isolated HTTPS/Kafka/native-SPIRE learning demo; own resources only."""
import argparse
import base64
import http.client
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import ssl
import tempfile
import time
import uuid

from attestation import ROOT, run, digest, wait_for
from identity import IdentityRuntime, private_write


def verify_compose(value):
    services=value['services']
    expected={'kafka':['/tmp:uid=1000,gid=1000,mode=700,size=64m','/opt/kafka/logs:uid=1000,gid=1000,mode=700,size=32m'],'admin':['/tmp:uid=1000,gid=1000,mode=700,size=32m']}
    for name,mounts in expected.items():
        if services[name].get('tmpfs')!=mounts:raise RuntimeError('Rendered temporary mounts do not match owned UID/destinations')
    if 'kafka' in services['ui']['networks']:raise RuntimeError('UI must not join Kafka network')
    for name,service in services.items():
        if name!='ui' and service.get('ports'):raise RuntimeError('Backend or broker host publication forbidden')
        if str(service.get('user'))!='1000:1000' or not service.get('read_only') or 'ALL' not in service.get('cap_drop',[]):raise RuntimeError('Container hardening mismatch')
    targets={v['target'] for v in services['ui']['volumes']}
    if any('kafka' in target or 'state' in target or 'pki' in target for target in targets):raise RuntimeError('UI received broker/private state mount')
    for caller,other in [('trusted-caller','untrusted'),('untrusted-caller','trusted')]:
        for volume in services[caller]['volumes']:
            if ('/'+other+'-control' in volume.get('source','') or '/'+other+'-state' in volume.get('source','')):raise RuntimeError('Cross-caller private mount')


class KafkaStack:
    def __init__(self, port=18088, test_build=False):
        if not isinstance(port,int) or not 1024<=port<=65535:
            raise ValueError('An unprivileged loopback HTTPS port is required')
        os.umask(0o077)
        self.work=Path(tempfile.mkdtemp(prefix='poc-kafka-',dir='/tmp'))
        self.stream=uuid.uuid4().hex
        self.project='poc-kafka-'+uuid.uuid4().hex[:10]
        self.image=self.project+':apps'
        self.port=port;self.test_build=test_build
        self.url=f'https://127.0.0.1:{port}'
        self.metadata=self.work/'stack.json'
        self.runtime=None;self.up=False;self.built=False;self.closed=False;self.stage="initialized"
        self.env=os.environ.copy();self.env.update(GOTOOLCHAIN='local',CGO_ENABLED='0',POC_PROJECT=self.project,POC_IMAGE=self.image,POC_WORK=str(self.work),POC_SOCKET_DIR=str(self.work/'workload'),POC_PORT=str(port),POC_ORIGIN=self.url,POC_STREAM=self.stream)
        self.compose=['docker','compose','-p',self.project,'-f',str(ROOT/'infra/kafka-signing/compose.yaml')]
        private_write(self.work/'isolated-runtime.json',json.dumps({'version':1,'purpose':'poc-spire-kafka','ephemeral':True,'root':str(self.work),'instance':uuid.uuid4().hex}))
        for name in ['trusted-control','untrusted-control','kafka-data','broker-config']:
            (self.work/name).mkdir(mode=0o700)
        self.context=None;self.cookie=None;self.csrf=None;self.bootstrap_diagnostic=""

    def request(self,method,path,body=None,headers=None):
        connection=http.client.HTTPSConnection('127.0.0.1',self.port,context=self.context,timeout=5)
        try:
            values=dict(headers or {})
            if self.cookie:values['Cookie']=self.cookie
            if body is not None and not isinstance(body,(bytes,str)):
                body=json.dumps(body);values['Content-Type']='application/json'
            if method=='POST':values.update(Origin=self.url,**{'X-CSRF-Token':self.csrf or ''})
            connection.request(method,path,body=body,headers=values)
            response=connection.getresponse();raw=response.read(65537)
            if len(raw)>65536:raise RuntimeError('Oversized UI response')
            if response.getheader('Set-Cookie'):self.cookie=response.getheader('Set-Cookie').split(';',1)[0]
            return response.status,raw
        finally:connection.close()

    def json(self,method,path,body=None,headers=None):
        code,raw=self.request(method,path,body,headers)
        return code,json.loads(raw)

    def start(self):
        # Reject collisions before creating or removing any external resource.
        with socket.socket() as listener:listener.bind(('127.0.0.1',self.port))
        existing=run(['docker','ps','-aq','--filter','label=com.docker.compose.project='+self.project]).stdout.strip()
        if existing:raise RuntimeError('Compose project collision; no cleanup of existing project')
        if self.metadata.exists():raise RuntimeError('Owned descriptor already exists; no overwrite')
        self.stage="native-runtime-initialization"
        self.runtime=IdentityRuntime(ephemeral_root=self.work,ca_ttl=600,isolated=True)
        spec=importlib.util.spec_from_file_location('kafka_fixture_pki',ROOT/'infra/kafka-signing/broker-pki.py')
        pki=importlib.util.module_from_spec(spec);spec.loader.exec_module(pki)
        ca=pki.generate(self.work,self.stream);self.context=ssl.create_default_context(cafile=str(ca))
        config=self.work/'broker-config'
        for source,target in [('kafka.properties.tmpl','kafka.properties'),('broker-start.sh','broker-start.sh'),('acl-bootstrap.sh','acl-bootstrap.sh')]:
            shutil.copyfile(ROOT/'infra/kafka-signing'/source,config/target);(config/target).chmod(0o600)
        private_write(config/'cluster-id',base64.urlsafe_b64encode(uuid.uuid4().bytes).decode().rstrip('='))
        private_write(config/'admin.properties','security.protocol=SSL\nssl.protocol=TLSv1.3\nssl.enabled.protocols=TLSv1.3\nssl.endpoint.identification.algorithm=https\nssl.keystore.type=PEM\nssl.keystore.location=/pki/client.pem\nssl.truststore.type=PEM\nssl.truststore.location=/pki/ca.pem\n')
        # The admin combined key/cert is used only by the isolated bootstrap job.
        admin=self.work/'admin-pki';private_write(admin/'client.pem',(admin/'client.key').read_text()+(admin/'client.pem').read_text())
        self.stage='application-build'
        names=['money','trusted-caller','untrusted-caller','ui','security-client','control-client','kafka-init']
        for name in names:
            args=['go','build','-trimpath']
            if self.test_build:args+=['-tags','kafkatest']
            run(args+['-o',str(self.work/name),'./cmd/'+name],env=self.env,timeout=120)
            (self.work/name).chmod(0o555)
        for role,path in [('trusted','trusted-state'),('unidentified','untrusted-state')]:
            run([str(self.work/'kafka-init'),'-state',str(self.work/path),'-stream',self.stream,'-role',role])
        shutil.copyfile(ROOT/'infra/local-ui/Dockerfile',self.work/'Dockerfile')
        private_write(self.work/'.dockerignore','*\n!Dockerfile\n'+''.join('!'+name+'\n' for name in names))
        run(['docker','build','--network','none','-t',self.image,str(self.work)],timeout=120);self.built=True
        self.image_digest=run(['docker','image','inspect',self.image,'--format','{{.Id}}']).stdout.strip()
        self.binary_hashes={name:digest(self.work/name) for name in names}
        self.stage="native-identity-start"
        self.runtime.start()
        for sid,binary,role in [('spiffe://poc.internal/money','money','kafka-money'),('spiffe://poc.internal/caller/trusted','trusted-caller','kafka-trusted'),('spiffe://poc.internal/ui','ui','kafka-ui')]:
            self.runtime.register(sid,self.work/binary,self.image_digest,role,ttl=90)
        self.stage='rendered-configuration'
        rendered=json.loads(run(self.compose+['--profile','bootstrap','config','--format','json'],env=self.env).stdout)
        verify_compose(rendered)
        self.up=True
        self.stage='broker-start'
        run(self.compose+['up','-d','kafka'],env=self.env,timeout=180)
        # Authenticated admin topic/ACL creation proves broker TLS and admission.
        def bootstrap():
            result=run(self.compose+['run','--rm','--no-deps','admin'],env=self.env,check=False,timeout=45)
            self.bootstrap_diagnostic=(result.stdout+result.stderr)[-5000:]
            return result.returncode==0
        self.stage='broker-bootstrap'
        wait_for(bootstrap,'Private Kafka TLS/topic/ACL bootstrap',seconds=120)
        self.stage='application-start'
        run(self.compose+['up','-d','money','trusted-caller','untrusted-caller','ui'],env=self.env,timeout=60)
        for name in ['trusted-control','untrusted-control']:
            wait_for(lambda name=name:(self.work/name/'api.sock').is_socket(),'Private caller control',seconds=40)
        def ready():
            try:
                code,raw=self.request('GET','/')
                if code!=200 or b'data-ready="true"' not in raw:return False
                code,value=self.json('GET','/api/session')
                if code!=200:return False
                self.csrf=value['csrf_token']
                code,value=self.json('GET','/api/balances')
                return code==200 and value.get('server_verified') is True
            except (OSError,ValueError,KeyError):return False
        self.stage='complete-readiness'
        wait_for(ready,'Authenticated Money assignment/balances and browser HTTPS',seconds=60)
        private_write(self.metadata,json.dumps({'project':self.project,'work':str(self.work),'origin':self.url,'stream_id':self.stream,'pid':os.getpid(),'ephemeral':True,'human_identity':'assumed demo operator'}))
        return self

    def attempt(self,scenario,key=None):
        key=key or uuid.uuid4().hex
        body={'browser_key':key}
        if scenario.startswith('kafka-'):body.update(scenario=scenario,stream_id=self.stream);path='/api/kafka-attempts'
        else:body['caller']=scenario;path='/api/attempts'
        code,value=self.json('POST',path,body)
        if code!=202:raise RuntimeError('Fixed attempt creation failed: '+str(code))
        deadline=time.monotonic()+40
        while value['status']=='pending' and time.monotonic()<deadline:
            time.sleep(.2);code,value=self.json('GET','/api/attempts/'+value['attempt_id'])
            if code!=200:raise RuntimeError('Attempt lookup failed')
        if value['status']=='pending':raise RuntimeError('Bounded attempt did not finish')
        return value,key

    def close(self):
        if self.closed:
            return {'owned_cleanup_complete':True,'ephemeral_state_removed':not self.work.exists(),'errors':[],'compose_exit_code':0,'image_exit_code':0}
        errors=[];compose_code=0;image_code=0
        if self.up:
            try:
                result=run(self.compose+['down','--volumes','--remove-orphans'],env=self.env,check=False,timeout=60)
                compose_code=result.returncode
                if compose_code:errors.append({'action':'compose-down','reason':'nonzero'})
                else:self.up=False
            except Exception as error:
                compose_code=-1;errors.append({'action':'compose-down','reason':type(error).__name__})
        if self.runtime:
            try:
                self.runtime.close();self.runtime=None
            except Exception as error:
                errors.append({'action':'native-runtime','reason':type(error).__name__})
        if self.built:
            try:
                result=run(['docker','image','rm',self.image],check=False,timeout=30)
                image_code=result.returncode
                if image_code:errors.append({'action':'image-removal','reason':'nonzero'})
                else:self.built=False
            except Exception as error:
                image_code=-1;errors.append({'action':'image-removal','reason':type(error).__name__})
        # Keep private ownership/configuration available for a safe retry if an
        # owned container, child process or image could still remain. Never claim
        # success after an earlier timeout merely because close was called again.
        if not self.up and self.runtime is None and not self.built:
            try:
                shutil.rmtree(self.work)
            except FileNotFoundError:
                pass
            except Exception as error:
                errors.append({'action':'private-state-removal','reason':type(error).__name__})
        self.closed=not errors and not self.up and self.runtime is None and not self.built and not self.work.exists()
        return {'owned_cleanup_complete':self.closed,'compose_exit_code':compose_code,'image_exit_code':image_code,'ephemeral_state_removed':not self.work.exists(),'errors':errors}


def require_complete_cleanup(value):
    if not value.get('owned_cleanup_complete') or value.get('errors') or value.get('compose_exit_code') or value.get('image_exit_code') or not value.get('ephemeral_state_removed'):
        raise RuntimeError('Owned cleanup incomplete; inspect private ownership state and retry')


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--port',type=int,default=18088);args=parser.parse_args()
    stack=KafkaStack(args.port)
    def stop(*_):raise KeyboardInterrupt()
    signal.signal(signal.SIGTERM,stop)
    try:
        stack.start()
        print('Isolated combined HTTPS/Kafka demo ready at '+stack.url,flush=True)
        print('Temporary browser CA: '+str(stack.work/'browser/ca.pem'),flush=True)
        print('Operator is assumed. Ctrl-C removes this run and its fake state.',flush=True)
        while True:
            time.sleep(2)
            if any(p.poll() is not None for p in stack.runtime.processes):raise RuntimeError('Owned SPIRE process stopped')
    except KeyboardInterrupt:pass
    finally:
        cleanup=stack.close()
        require_complete_cleanup(cleanup)

if __name__=='__main__':main()
