#!/usr/bin/env python3
"""Serial real-SPIRE/Kafka acceptance with private fixed-case test builds."""
import base64
import hashlib
import http.client
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import ssl
import subprocess
import time
import uuid

from attestation import ROOT,run,wait_for
from kafka_demo import KafkaStack,require_complete_cleanup

PROTECTED=Path('/home/ubuntu/Workspace/poc-spire')

def private_fingerprint(root):
    values={}
    for path in sorted(root.rglob('*')):
        if path.is_file() and not path.is_symlink():values[str(path.relative_to(root))]=(path.stat().st_mode&0o777,hashlib.sha256(path.read_bytes()).hexdigest())
    return values

def observe_live():
    state=run(['systemctl','is-active','poc-spire-demo.service'],check=False).stdout.strip()
    pid=run(['systemctl','show','poc-spire-demo.service','--property','MainPID','--value'],check=False).stdout.strip()
    context=ssl.create_default_context();connection=http.client.HTTPSConnection('spire.adianto.dev',context=context,timeout=8)
    try:connection.request('GET','/healthz');response=connection.getresponse();response.read(1024);health=response.status
    finally:connection.close()
    return {'service_state':state,'service_pid':pid,'https_health_status':health}

class Acceptance:
    def __init__(self,stack):self.stack=stack;self.checks=[]
    def pass_case(self,name,details):self.checks.append({'check':name,'mode':'real-spire-kafka','result':'passed','details':details})
    def fixed(self,case,id=None,context=None):
        id=id or uuid.uuid4().hex
        args=self.stack.compose+['exec','-T','trusted-caller','/trusted-caller','-fixture-case',case,'-fixture-config','/kafka/client.json','-fixture-id',id]
        if context:args+=['-fixture-created',str(context['created_at']),'-fixture-deadline',str(context['deadline'])]
        result=run(args,env=self.stack.env,timeout=30)
        return json.loads(result.stdout)
    def read(self,digest):
        result=run(self.stack.compose+['exec','-T','ui','/ui','-fixture-case','receipt','-fixture-digest',digest],env=self.stack.env,timeout=15)
        return json.loads(result.stdout)
    def metadata(self):
        result=run(self.stack.compose+['exec','-T','ui','/ui','-fixture-case','metadata'],env=self.stack.env,timeout=15)
        return json.loads(result.stdout)
    def receipt(self,digest,timeout=12):
        deadline=time.monotonic()+timeout
        while time.monotonic()<deadline:
            value=self.read(digest)
            if value['query_http_status']==200:return value['receipt']
            time.sleep(.2)
        raise RuntimeError('No retained authenticated receipt within acceptance deadline')
    def balances(self):
        code,value=self.stack.json('GET','/api/balances')
        if code!=200 or not value.get('server_verified'):raise RuntimeError('Authenticated balance read unavailable')
        return value
    def publish_ui_signature(self,id):
        # This signing process has only the UI workload socket and Money network.
        # It receives a public policy JSON, never a broker key/network or admin key.
        policy=self.stack.work/'fixture-policy.json'
        policy.write_text(json.dumps({'stream_id':self.stack.stream,'brokers':['kafka:9093'],'ca':'unmounted','certificate':'unmounted','key':'unmounted'}));policy.chmod(0o600)
        args=['docker','run','--rm','--network',self.stack.project+'_money','--user','1000:1000','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true','--label','poc-spire.role=kafka-ui','-v',str(self.stack.work/'workload')+':/workload:ro','-v',str(policy)+':/fixture/policy.json:ro','--entrypoint','/ui',self.stack.image,'-fixture-case','prepare-valid','-fixture-config','/fixture/policy.json','-fixture-id',id]
        prepared=run(args,env=self.stack.env,timeout=25).stdout
        command=self.stack.compose+['exec','-T','trusted-caller','/trusted-caller','-fixture-case','publish-prepared','-fixture-config','/kafka/client.json']
        result=subprocess.run(command,cwd=ROOT,env=self.stack.env,input=prepared,text=True,capture_output=True,timeout=25)
        if result.returncode:raise RuntimeError('Private prepared-record publication failed')
        return json.loads(result.stdout)
    def broker_case(self,case,credential='untrusted-pki'):
        # A disposable host-controlled fixture has only its own conventional
        # broker client key. No SPIFFE key or application admin key is borrowed.
        args=['docker','run','--rm','--network',self.stack.project+'_kafka','--user','1000:1000','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true','-v',str(self.stack.work/credential)+':/kafka:ro','--entrypoint','/trusted-caller',self.stack.image,'-fixture-case',case,'-fixture-config','/kafka/client.json']
        return json.loads(run(args,env=self.stack.env,timeout=20).stdout)
    def offset(self):
        result=run(self.stack.compose+['run','--rm','--no-deps','--entrypoint','/opt/kafka/bin/kafka-consumer-groups.sh','admin','--bootstrap-server','kafka:9093','--command-config','/config/admin.properties','--describe','--group','poc.spire.money.'+self.stack.stream],env=self.stack.env,timeout=20)
        for line in result.stdout.splitlines():
            columns=line.split()
            if len(columns)>=4 and columns[1]=='poc.spire.commands.'+self.stack.stream:
                try:return int(columns[3])
                except ValueError:pass
        raise RuntimeError('No exact committed partition offset')
    def caller_control(self,method,path,body=None,role='trusted'):
        connection=http.client.HTTPConnection('localhost',timeout=12)
        connection.sock=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM);connection.sock.settimeout(12)
        connection.sock.connect(str(self.stack.work/(role+'-control')/'api.sock'))
        try:
            connection.request(method,path,None if body is None else json.dumps(body),{'Content-Type':'application/json'})
            response=connection.getresponse();raw=response.read(8193)
            if len(raw)>8192:raise RuntimeError('Private fixture response exceeded limit')
            return response.status,json.loads(raw)
        finally:connection.close()
    def private_leaf(self,id):
        # Compare two public leaf encodings in private memory only. Never emit
        # the JWS/chain from bounded caller run state as acceptance evidence.
        disk=json.loads((self.stack.work/'trusted-state/state.json').read_text())
        record=disk['payload']['entries'][id]['variants'][-1]['record']
        compact=base64.b64decode(record['Value']).decode();header=compact.split('.')[0]
        return json.loads(base64.urlsafe_b64decode(header+'='*(-len(header)%4)))['x5c'][0]
    def rotation_and_cached_outage(self):
        s=self.stack;id=uuid.uuid4().hex;instance=self.metadata()['instance_id'];before=self.balances()['version']
        body={'request_id':id,'scenario':'kafka-trusted','money_instance_id':instance}
        code,first=self.caller_control('POST','/attempt/kafka',body)
        if code!=200 or first['publication']!='queued':raise RuntimeError('Initial private rotation send failed')
        self.receipt(first['variants'][0]['digest']);first_leaf=self.private_leaf(id)
        # The real registered workload TTL is 90 seconds. A bounded wait beyond
        # half its lifetime lets SPIRE's live source rotate while preserving the
        # original 120-second command context and already decided logical ID.
        until=time.monotonic()+47
        while time.monotonic()<until:time.sleep(min(1,until-time.monotonic()))
        code,second=self.caller_control('POST','/attempt/kafka',body)
        if code!=200 or len(second['variants'])!=2 or self.private_leaf(id)==first_leaf:raise RuntimeError('Real matched workload rotation was not observed')
        current=self.receipt(second['variants'][-1]['digest'])
        if current['disposition']!='replayed' or second['created_at']!=first['created_at'] or second['deadline']!=first['deadline'] or second['money_instance_id']!=instance or self.balances()['version']!=before+1:raise RuntimeError('Rotated retry changed context or duplicated effect')
        self.pass_case('real-svid-rotation-private-retry',{'matched_leaf_changed':True,'same_context_deadline_nonce':True,'variants':2,'one_effect':True,'workload_ttl_seconds':90})
        # Pause only this run's native SPIRE children. Existing application
        # sources retain currently valid SVIDs and independent cached bundles.
        outage_id=uuid.uuid4().hex;paused=[]
        try:
            for process in s.runtime.processes:
                if process.poll() is not None:raise RuntimeError('Owned SPIRE process already stopped')
                os.kill(process.pid,signal.SIGSTOP);paused.append(process.pid)
            time.sleep(1)
            code,queued=self.caller_control('POST','/attempt/kafka',{'request_id':outage_id,'scenario':'kafka-trusted','money_instance_id':instance})
            if code!=200 or queued['publication']!='queued':raise RuntimeError('Cached signing source failed during bounded outage')
            wait_for(lambda:self.balances()['version']==before+2,'Cached independent trust decision during outage',seconds=5)
        finally:
            for pid in paused:os.kill(pid,signal.SIGCONT)
        receipt=self.receipt(queued['variants'][0]['digest'])
        if receipt['disposition']!='applied':raise RuntimeError('Cached outage had no authenticated terminal receipt')
        self.pass_case('cached-valid-sources-during-spire-outage',{'owned_native_children_paused':True,'cached_signing_and_independent_trust_valid':True,'authenticated_receipt':'applied','live_money_instance_preserved':True})
    def browser_and_private_boundaries(self):
        s=self.stack
        for body in [{'scenario':'kafka-trusted','browser_key':uuid.uuid4().hex,'stream_id':s.stream,'amount_minor':200},{'scenario':'arbitrary','browser_key':uuid.uuid4().hex,'stream_id':s.stream},{'scenario':'kafka-trusted','browser_key':uuid.uuid4().hex,'stream_id':'0'*32}]:
            if s.request('POST','/api/kafka-attempts',body)[0]!=400:raise RuntimeError('Arbitrary or stale browser command accepted')
        body={'scenario':'kafka-trusted','browser_key':uuid.uuid4().hex,'stream_id':s.stream}
        # Bypass the runner's convenience headers to exercise actual HTTPS gates.
        for bad,expected in [({'Origin':s.url,'X-CSRF-Token':'invalid'},403),({'Origin':'https://wrong.invalid','X-CSRF-Token':s.csrf},403),({'Origin':s.url,'X-CSRF-Token':s.csrf,'Host':'wrong.invalid'},403)]:
            connection=http.client.HTTPSConnection('127.0.0.1',s.port,context=s.context,timeout=5)
            try:
                headers={'Cookie':s.cookie,'Content-Type':'application/json',**bad};connection.request('POST','/api/kafka-attempts',json.dumps(body),headers);reply=connection.getresponse();reply.read(8192)
                if reply.status!=expected:raise RuntimeError('Actual browser boundary classification failed: '+str(reply.status))
            finally:connection.close()
        code,_=self.caller_control('POST','/attempt/kafka',{'request_id':'bad-shape','scenario':'kafka-trusted','money_instance_id':self.metadata()['instance_id'],'topic':'arbitrary'})
        if code!=400:raise RuntimeError('Private route accepted arbitrary topic')
        value=json.loads(run(s.compose+['exec','-T','untrusted-caller','/control-client','-socket','/trusted-control/api.sock'],env=s.env,check=False,timeout=15).stdout)
        if value.get('outcome')!='control_unreachable':raise RuntimeError('Unidentified container reached trusted control')
        self.pass_case('actual-browser-csrf-origin-host-and-fixed-schema',{'csrf_status':403,'origin_status':403,'host_status':403,'arbitrary_or_stale_context_status':400})
        self.pass_case('actual-private-fixed-schema-and-cross-caller-isolation',{'extra_destination_status':400,'unidentified_to_trusted_control':'unreachable'})
        collision=KafkaStack(port=s.port)
        try:
            try:collision.start()
            except OSError:pass
            else:raise RuntimeError('Busy browser port was not rejected')
        finally:require_complete_cleanup(collision.close())
        if self.balances().get('server_verified') is not True or any(p.poll() is not None for p in s.runtime.processes):raise RuntimeError('Collision cleanup damaged active owned stack')
        self.pass_case('actual-port-collision-owned-cleanup',{'collision_rejected_before_start':True,'existing_stack_still_verified':True,'colliding_state_removed':True})

    def missing_caller_state(self):
        s=self.stack;key=uuid.uuid4().hex;first,_=s.attempt('kafka-trusted',key);before=self.balances()['version']
        (s.work/'trusted-state/state.json').unlink()
        run(s.compose+['restart','ui'],env=s.env,timeout=30);s.cookie=None;s.csrf=None
        def ready():
            try:
                if s.request('GET','/')[0]!=200:return False
                code,value=s.json('GET','/api/session');s.csrf=value.get('csrf_token');return code==200
            except (OSError,ValueError):return False
        wait_for(ready,'UI after expected sender-state loss',seconds=30)
        recovered,_=s.attempt('kafka-trusted',key)
        code,value=self.caller_control('POST','/attempt',{'request_id':'sync-after-kafka-state-loss'})
        if recovered['status']!='retryable' or recovered['kafka_evidence']['outcome']!='unknown_commit' or (s.work/'trusted-state/state.json').exists() or code!=200 or value['outcome']!='transfer_succeeded' or self.balances()['version']!=before+1:raise RuntimeError('State loss recreated/re-published or disabled Sync')
        self.pass_case('actual-caller-state-loss-ui-recovery',{'missing_state_not_recreated':True,'same_browser_attempt':'unknown_commit','no_auto_kafka_republication':True,'sync_still_works':True})

    def crash_redelivery(self):
        s=self.stack;fault=s.work/'money-fault';before=self.balances()['version'];committed=self.offset()
        (fault/'block-ack').write_text('blocked');(fault/'block-ack').chmod(0o600)
        published=self.fixed('valid');old=self.receipt(published['digest'])
        if self.offset()!=committed or self.balances()['version']!=before+1:raise RuntimeError('Pre-ack point was not controlled')
        run(s.compose+['kill','-s','SIGKILL','money'],env=s.env,timeout=10)
        run(s.compose+['start','money'],env=s.env,timeout=15)
        def recovered():
            try:
                value=self.read(published['digest'])
                return value['query_http_status']==200 and value['receipt']['instance_id']!=old['instance_id']
            except (RuntimeError,ValueError):return False
        wait_for(recovered,'Exact still-valid crash redelivery',seconds=20)
        new=self.receipt(published['digest'])
        if new['disposition']!='applied' or new['business_result']['version']!=1 or new['digest']!=old['digest'] or self.offset()!=committed:raise RuntimeError('Exact retained record redelivery not proved')
        (fault/'block-ack').unlink()
        wait_for(lambda:self.offset()==committed+1,'Captured-generation next-offset acknowledgement',seconds=12)
        self.pass_case('sigkill-after-ledger-before-ack',{'exact_original_digest':published['digest'],'old_and_new_money_instances_differ':True,'new_lifetime_version':1,'committed_offset_before':committed,'committed_offset_after':committed+1,'same_record_redelivered_without_republish':True})
        # A crash before any terminal decision leaves the same exact record pending.
        committed=self.offset();(fault/'block-processing').write_text('blocked');(fault/'block-processing').chmod(0o600)
        before=self.balances()['version'];published=self.fixed('valid');time.sleep(.4)
        if self.read(published['digest'])['query_http_status']!=404 or self.offset()!=committed or self.balances()['version']!=before:raise RuntimeError('Pre-decision gate allowed an effect/offset')
        run(s.compose+['kill','-s','SIGKILL','money'],env=s.env,timeout=10);(fault/'block-processing').unlink();run(s.compose+['start','money'],env=s.env,timeout=15)
        receipt=self.receipt(published['digest'],timeout=20)
        if receipt['disposition']!='applied' or receipt['business_result']['version']!=1:raise RuntimeError('Pre-decision retained redelivery failed')
        wait_for(lambda:self.offset()==committed+1,'Pre-decision crash offset',seconds=12)
        self.pass_case('sigkill-before-decision',{'exact_original_digest':published['digest'],'no_precrash_effect':True,'new_lifetime_version':1,'committed_offset_before':committed,'committed_offset_after':committed+1})

    def run(self):
        s=self.stack
        for scenario in ['trusted','unidentified','kafka-trusted','kafka-unidentified','kafka-tampered']:
            attempt,key=s.attempt(scenario)
            result=attempt['kafka_evidence'] if scenario.startswith('kafka-') else attempt['caller_evidence']
            expected='money_rejected' if scenario in ['kafka-unidentified','kafka-tampered'] else 'tls_rejected' if scenario=='unidentified' else 'transfer_succeeded'
            if attempt['status']!='done' or result['outcome']!=expected:raise RuntimeError('Fixed scenario failed')
            self.pass_case('fixed-'+scenario,{'outcome':expected})
        for service,binary in [('trusted-caller','/ui'),('untrusted-caller','/trusted-caller')]:
            value=json.loads(run(s.compose+['exec','-T',service,binary,'-fixture-case','attestation-denied'],env=s.env,timeout=15).stdout)
            if value.get('identity_eligibility')!='no_matching_identity' or not value.get('socket_reachable') or not value.get('money_reachable'):raise RuntimeError('Attestation denial substituted infrastructure failure')
            self.pass_case('attestation-'+('wrong-executable' if service=='trusted-caller' else 'wrong-deployment'),value)
        for case,credential in [('broker-principal-denied','rogue-pki'),('broker-topic-denied','untrusted-pki'),('broker-group-denied','untrusted-pki'),('broker-wrong-host','untrusted-pki')]:
            value=self.broker_case(case,credential);self.pass_case(case,value)
        wrong_reader=json.loads(run(s.compose+['exec','-T','trusted-caller','/trusted-caller','-fixture-case','wrong-reader'],env=s.env,timeout=15).stdout)
        self.pass_case('observer-resource-wrong-reader',wrong_reader)
        no_cert=json.loads(run(s.compose+['exec','-T','trusted-caller','/trusted-caller','-fixture-case','no-certificate-reader'],env=s.env,timeout=15).stdout)
        self.pass_case('observer-resource-no-certificate',no_cert)
        before=self.balances()['version']
        for case,reason in [('tampered','invalid_signature'),('fake-root','untrusted_svid'),('wrong-type','context_mismatch'),('wrong-environment','context_mismatch'),('wrong-audience','context_mismatch'),('wrong-stream','context_mismatch'),('wrong-topic','context_mismatch'),('wrong-key','context_mismatch'),('headers','unsupported_kafka_headers'),('stripped','malformed_signature'),('none','unsupported_header'),('hs256','unsupported_header'),('es384','unsupported_header'),('der','malformed_signature'),('unknown-header','malformed_header'),('expired-message','message_expired')]:
            published=self.fixed(case);receipt=self.receipt(published['digest'])
            if receipt['disposition']!='rejected' or receipt['reason']!=reason or receipt.get('business_result'):raise RuntimeError('Negative record classification failed: '+case)
            if reason in ['invalid_signature','untrusted_svid','malformed_jws','unsupported_header','malformed_signature','malformed_header'] and receipt.get('verified_producer_id'):raise RuntimeError('Invalid signature/trust was attributed')
            if self.balances()['version']!=before:raise RuntimeError('Negative record changed ledger')
            self.pass_case('negative-'+case,{'receipt_reason':reason,'no_ledger_effect':True})
        wrong=self.publish_ui_signature(uuid.uuid4().hex);receipt=self.receipt(wrong['digest'])
        if receipt['reason']!='unauthorized_signer' or receipt.get('verified_producer_id')!='spiffe://poc.internal/ui' or self.balances()['version']!=before:raise RuntimeError('Valid wrong signer was not denied after authentication')
        self.pass_case('wrong-valid-original-signer',{'authenticated_id':'spiffe://poc.internal/ui','reason':'unauthorized_signer','broker_admitted':True})
        # Reuse the same deterministic logical ID across HTTP and Kafka ingresses.
        key=uuid.uuid4().hex;sync,_=s.attempt('trusted',key);id=sync['attempt_id'];version=self.balances()['version'];copy=self.fixed('valid',id);receipt=self.receipt(copy['digest'])
        if receipt['disposition']!='replayed' or self.balances()['version']!=version:raise RuntimeError('Sync to Kafka duplicated effect')
        conflict=self.fixed('business-invalid',id,copy['context']);receipt=self.receipt(conflict['digest'])
        if receipt['business_code']!=409 or receipt['reason']!='idempotency_conflict' or self.balances()['version']!=version:raise RuntimeError('Changed business payload did not conflict')
        self.pass_case('sync-kafka-shared-id-and-conflict',{'one_effect':True,'changed_payload_business_code':409})
        # UI/caller restart must recover descriptor and receipts without refresh.
        key=uuid.uuid4().hex;first,_=s.attempt('kafka-trusted',key);descriptor=first['kafka_evidence']['caller_diagnostics'];version=self.balances()['version']
        run(s.compose+['restart','ui','trusted-caller'],env=s.env,timeout=45)
        s.cookie=None;s.csrf=None
        def browser_ready():
            try:
                if s.request('GET','/')[0]!=200:return False
                code,value=s.json('GET','/api/session');s.csrf=value.get('csrf_token');return code==200
            except (OSError,ValueError):return False
        wait_for(browser_ready,'Restarted UI source',seconds=30)
        recovered,_=s.attempt('kafka-trusted',key);current=recovered['kafka_evidence']['caller_diagnostics']
        if recovered['kafka_evidence']['outcome']!='transfer_succeeded' or current['created_at']!=descriptor['created_at'] or current['deadline']!=descriptor['deadline'] or current['money_instance_id']!=descriptor['money_instance_id'] or current['variants']!=descriptor['variants'] or self.balances()['version']!=version:raise RuntimeError('Caller/UI restart lost bounded retry state')
        self.pass_case('ui-caller-restart-recovery',{'original_deadline_and_variants_preserved':True,'no_second_effect':True})
        # Group offsets do not reconstruct a restarted in-memory Money ledger.
        old=self.metadata();run(s.compose+['restart','money'],env=s.env,timeout=45)
        new={}
        def money_ready():
            nonlocal new
            try:new=self.metadata();return new['consumer_ready'] and new['instance_id']!=old['instance_id']
            except (RuntimeError,ValueError):return False
        wait_for(money_ready,'Restarted Money assignment',seconds=40)
        if self.balances()['version']!=0:raise RuntimeError('Broker offsets restored fake ledger')
        run(s.compose+['restart','ui'],env=s.env,timeout=45)
        s.cookie=None;s.csrf=None;wait_for(browser_ready,'UI after Money reset',seconds=30)
        reset,_=s.attempt('kafka-trusted',key)
        if reset['status']!='retryable' or reset['kafka_evidence']['outcome']!='unknown_commit' or self.balances()['version']!=0:raise RuntimeError('Detected Money reset automatically republished')
        self.pass_case('money-restart-offsets-and-ui-nonce',{'new_instance':True,'ledger_reset':True,'no_automatic_republication':True})
        # Still-valid prior operation resent in the new Money lifetime applies
        # anew, demonstrating the documented lack of durable replay protection.
        replay=self.fixed('valid',id);receipt=self.receipt(replay['digest'])
        if receipt['disposition']!='applied' or self.balances()['version']!=1:raise RuntimeError('Documented process-memory reset was not demonstrated')
        self.pass_case('reused-stream-new-money-lifetime',{'same_original_id_can_apply_anew':True,'durable_replay_guarantee':False})
        self.crash_redelivery()
        self.browser_and_private_boundaries()
        self.rotation_and_cached_outage()
        self.missing_caller_state()
        # Runtime mount/network checks cover actual configured containers.
        rows=[json.loads(line) for line in run(s.compose+['ps','--format','json'],env=s.env).stdout.splitlines() if line.strip()]
        if sum(row['Service']=='money' for row in rows)!=1:raise RuntimeError('Multiple Money replicas are unsupported')
        for row in rows:
            info=json.loads(run(['docker','inspect',row['ID']]).stdout)[0];name=row['Service'];host=info['HostConfig'];targets={m['Destination'] for m in info['Mounts']}
            if name=='ui' and (s.project+'_kafka' in info['NetworkSettings']['Networks'] or any('kafka' in target or 'pki' in target for target in targets)):raise RuntimeError('UI broker boundary failed')
            if any('docker.sock' in target or '/identity' in target for target in targets):raise RuntimeError('Application received admin/other identity state')
            if name!='kafka' and (not host['ReadonlyRootfs'] or 'ALL' not in host['CapDrop']):raise RuntimeError('Application hardening failed')
        self.pass_case('actual-container-boundaries',{'ui_no_kafka_network_or_credential':True,'app_no_admin_mount':True})
        return self.checks

def main():
    report={'full_acceptance':False,'phase':'serial-kafka-acceptance','status':'running','build_profile':'kafkatest','source_revision':run(['git','rev-parse','HEAD']).stdout.strip(),'started_at_utc':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),'checks':[]}
    paths=run(['git','ls-files','--cached','--others','--exclude-standard','cmd','internal','scripts','infra','go.mod','go.sum','package.json','package-lock.json']).stdout.splitlines()
    report['source_sha256']={path:hashlib.sha256((ROOT/path).read_bytes()).hexdigest() for path in sorted(set(paths)) if (ROOT/path).is_file()}
    report['source_files_changed_from_revision']=bool(run(['git','diff','HEAD','--name-only','--','cmd','internal','scripts','infra','go.mod','go.sum','package.json','package-lock.json']).stdout.strip() or run(['git','ls-files','--others','--exclude-standard','cmd','internal','scripts','infra']).stdout.strip())
    report['versions']={'go':run(['go','version']).stdout.strip(),'docker':run(['docker','version','--format','{{.Server.Version}}']).stdout.strip(),'compose':run(['docker','compose','version','--short']).stdout.strip(),'kernel':run(['uname','-srmo']).stdout.strip(),'spire':'1.14.0','go_spiffe':'2.6.0','go_jose':'4.1.5','franz_go':'1.19.5','kmsg':'1.11.2','kafka':'4.0.2'}
    protected=private_fingerprint(PROTECTED/'.runtime/identity');live=observe_live();report['live_before']=live
    stack=KafkaStack(test_build=True)
    try:
        stack.start();report['image_config_digest']=stack.image_digest;report['binary_sha256']=stack.binary_hashes
        acceptance=Acceptance(stack);report['checks']=acceptance.checks
        acceptance.run();report['status']='passed_real_integration_increment'
    except Exception as error:
        report['status']='failed';report['failed_stage']=stack.stage;report['error_type']=type(error).__name__;report['reason']=str(error)[-2000:]
        raise RuntimeError('Serial acceptance increment failed; see sanitized evidence') from None
    finally:
        report['cleanup']=stack.close();report['live_after']=observe_live();report['retained_identity_unchanged']=private_fingerprint(PROTECTED/'.runtime/identity')==protected;report['live_service_continuity']=report['live_after']==live
        if not report['cleanup']['owned_cleanup_complete'] or not report['retained_identity_unchanged'] or not report['live_service_continuity']:report['status']='failed_cleanup_or_coexistence'
        report['finished_at_utc']=time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime());(ROOT/'docs/evidence/kafka-acceptance-current.json').write_text(json.dumps(report,indent=2)+'\n');require_complete_cleanup(report['cleanup'])
        if report['status']=='failed_cleanup_or_coexistence':raise RuntimeError('Protected resource continuity or cleanup failed')

if __name__=='__main__':main()
