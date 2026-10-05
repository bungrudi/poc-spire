#!/usr/bin/env python3
"""Bounded backend lifecycle tests; all fault state/credentials are ephemeral."""
import hashlib
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import select
import shutil
import subprocess
import tempfile
import time
import uuid

from attestation import ROOT, VERSION, run, wait_for, digest
from identity import IdentityRuntime


class Client:
    def __init__(self, compose, env, service, expected=None):
        args = compose + ['exec', '-T', service, '/backend-client']
        args += ['-expected', expected] if expected else ['-unidentified']
        self.process = subprocess.Popen(args, cwd=ROOT, env=env, text=True,
                                        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)

    def ask(self, action, request_id='backend-fixed'):
        if self.process.poll() is not None: raise RuntimeError('Backend fixture process stopped')
        self.process.stdin.write(json.dumps({'action': action, 'request_id': request_id})+'\n')
        self.process.stdin.flush()
        if not select.select([self.process.stdout], [], [], 28)[0]:
            raise RuntimeError('Backend fixture response deadline exceeded')
        line = self.process.stdout.readline()
        try: value = json.loads(line)
        except json.JSONDecodeError: raise RuntimeError('Backend fixture returned no bounded structured evidence')
        return value

    def close(self):
        if self.process.poll() is None:
            self.process.stdin.close()
            try: self.process.wait(timeout=6)
            except subprocess.TimeoutExpired: self.process.terminate(); self.process.wait(timeout=6)
        self.process.stdout.close()


def retained_fingerprint():
    # Compare locally; publish only the equality result, never credential bytes/hashes.
    base = ROOT/'.runtime/identity'
    return {str(p.relative_to(base)): (digest(p), p.stat().st_mode & 0o777)
            for p in base.rglob('*') if p.is_file()}


def stop_own(process):
    if process.poll() is None:
        process.terminate()
        try: process.wait(timeout=8)
        except subprocess.TimeoutExpired: process.kill(); process.wait()


def main():
    os.umask(0o077)
    before = retained_fingerprint()
    work = Path(tempfile.mkdtemp(prefix='backend-', dir=ROOT/'.runtime'))
    project = 'poc-spire-backend-'+uuid.uuid4().hex[:10]
    image = project+':test'
    env = os.environ.copy(); env.update(GOTOOLCHAIN='local', CGO_ENABLED='0', POC_PROJECT=project,
        POC_IMAGE=image, POC_SOCKET_DIR=str(work/'workload'))
    compose = ['docker','compose','-f',str(ROOT/'infra/backend/compose.yaml')]
    runtime=None; clients=[]
    report={'phase':'backend-lifecycle','status':'failed','spire_version':VERSION,
            'ca_ttl_seconds':120,'test_identity_state':'ephemeral','tests':[], 'started_at':datetime.now(timezone.utc).isoformat()}
    def passed(case, evidence):
        report['tests'].append({'case':case,**evidence});print(case+': passed',flush=True)
    try:
        print('Building isolated real-SPIRE backend lifecycle fixtures',flush=True)
        for name in ['backend-money','backend-client']:
            run(['go','build','-trimpath','-o',str(work/name),'./cmd/'+name],env=env);(work/name).chmod(0o555)
        shutil.copy(ROOT/'infra/backend/Dockerfile',work/'Dockerfile')
        run(['docker','build','--network','none','-t',image,str(work)])
        image_digest=run(['docker','image','inspect',image,'--format','{{.Id}}']).stdout.strip()
        report['binary_sha256']={name:digest(work/name) for name in ['backend-money','backend-client']}
        report['image_config_digest']=image_digest
        runtime=IdentityRuntime(ephemeral_root=work,ca_ttl=120);runtime.start()
        specs=[('spiffe://poc.internal/money','backend-money','backend-money',90),
               ('spiffe://poc.internal/caller/trusted','backend-client','backend-trusted',30),
               ('spiffe://poc.internal/ui','backend-client','backend-observer',120)]
        report['versions']={'go':run(['go','version']).stdout.strip(),
            'go_spiffe_sdk':run(['go','list','-m','github.com/spiffe/go-spiffe/v2'],env=env).stdout.strip(),
            'docker_engine':run(['docker','version','--format','{{.Server.Version}}']).stdout.strip(),
            'compose':run(['docker','compose','version','--short']).stdout.strip()}
        report['compose_sha256']=digest(ROOT/'infra/backend/compose.yaml')
        report['registration_policy']=[{'identity':identity,'uid':1000,'binary_sha256':digest(work/binary),
            'image_config_digest':image_digest,'deployment_role':role,'ttl_seconds':ttl,'parent':'exact attested parent (redacted)'}
            for identity,binary,role,ttl in specs]
        entries={}
        for identity,binary,role,ttl in specs:
            entries[identity]=runtime.register(identity,work/binary,image_digest,role,ttl=ttl)
        run(compose+['up','-d'],env=env)
        observer=Client(compose,env,'observer','spiffe://poc.internal/ui');clients.append(observer)
        trusted=Client(compose,env,'trusted','spiffe://poc.internal/caller/trusted');clients.append(trusted)
        unidentified=Client(compose,env,'unidentified');clients.append(unidentified)
        def balances():
            value=observer.ask('balances')
            if value.get('money_http_status')!=200 or not value.get('server_verified'):
                raise RuntimeError('Observer could not verify/read Money')
            return value
        initial=balances()['money_result']
        if initial['version']!=0:raise RuntimeError('Fresh Money fixture ledger was not empty')
        first=observer.ask('status');first_server=balances()['server_authority_key_id']
        public_before=digest(runtime.public/'bundle.pem')
        wrong=trusted.ask('wrong-bundle')
        if wrong['outcome']!='server_trust_failure' or wrong['server_verified'] or 'money_http_status' in wrong:
            raise RuntimeError('Wrong server bundle was not rejected at verification')
        missing_bundle=unidentified.ask('missing-bundle')
        if missing_bundle['outcome']!='infrastructure_failure' or missing_bundle['stage']!='server_trust_bundle' or missing_bundle.get('identity_eligibility')!='no_matching_identity' or 'money_http_status' in missing_bundle:
            raise RuntimeError('Missing bundle was misclassified as identity/TLS rejection')
        missing_socket=unidentified.ask('missing-socket')
        if missing_socket['outcome']!='infrastructure_failure' or 'identity_eligibility' in missing_socket or 'money_http_status' in missing_socket:
            raise RuntimeError('Missing socket was misclassified as identity rejection')
        if balances()['money_result']!=initial:raise RuntimeError('Trust/socket fault changed ledger')
        passed('T18-wrong-bundle-missing-bundle-and-socket',{'unrelated_ca_bundle_roots':1,'wrong_bundle':wrong,'missing_bundle':missing_bundle,'missing_socket':missing_socket,'ledger_unchanged':True})

        lost=trusted.ask('transfer','lost-response')
        if lost['outcome']!='unknown_commit' or not lost['server_verified'] or 'money_http_status' in lost:
            raise RuntimeError('Dropped committed response did not produce unknown outcome')
        after=balances()['money_result']
        retry=trusted.ask('transfer','lost-response')
        if after['version']!=1 or retry['outcome']!='transfer_succeeded' or not retry['money_result']['replay'] or balances()['money_result']!=after:
            raise RuntimeError('Dropped response retry caused duplicate/missing commit')
        passed('T16-real-post-commit-connection-drop',{'unknown_outcome':lost,'retry':retry,'ledger_version':1,'drop_point':'Money fixture closes TLS connection after real handler commit before response'})

        cached=trusted.ask('status');entry=entries['spiffe://poc.internal/caller/trusted']
        run([runtime.server,'entry','delete','-socketPath',runtime.server_socket,'-entryID',entry]);runtime.created_entries.remove(entry)
        denial={}
        def denied():
            denial.update(trusted.ask('fresh'));return denial.get('outcome')=='no_matching_identity'
        wait_for(denied,'Removed registration propagation',seconds=25)
        # No instant certificate revocation is claimed. Cached credentials may
        # work while valid; no further transfer should occur after their expiry.
        wait_for(lambda:time.time()>cached['expires_at']+1,'Removed registration cached credential expiry',seconds=40)
        expired=trusted.ask('transfer','removed-registration')
        if expired.get('money_http_status')==200 or expired['outcome']=='transfer_succeeded' or balances()['money_result']!=after:
            raise RuntimeError('Removed registration allowed transfer after cached credential expiry')
        passed('registration-removal-and-cached-expiry',{'fresh_workload_api':denial,'expired_attempt':expired,'ledger_unchanged':True,'instant_revocation_claimed':False})
        runtime.register('spiffe://poc.internal/caller/trusted',work/'backend-client',image_digest,'backend-trusted',ttl=30)
        wait_for(lambda:trusted.ask('status').get('expires_at',0)>time.time()+3,'Reinstated exact narrow registration',seconds=30)

        rotation={}
        def rotated():
            state=observer.ask('status');server=balances()
            ok=(state['authority_key_id']!=first['authority_key_id'] and server['server_authority_key_id']!=first_server
                and state['bundle_sha256']!=first['bundle_sha256'] and state['bundle_roots']>=2
                and digest(runtime.public/'bundle.pem')!=public_before and runtime.publisher.available)
            if ok:rotation.update(source=state,server_authority_key_id=server['server_authority_key_id'],public_bundle_sha256=digest(runtime.public/'bundle.pem'))
            return ok
        wait_for(rotated,'Real CA rollover through source/server/public bundle',seconds=190)
        # Compare the complete authenticated server bundle with the atomic public
        # file; if another preparation races this observation, retry the equality.
        wait_for(lambda:hashlib.sha256(run([runtime.server,'bundle','show','-socketPath',runtime.server_socket,'-format','pem'],timeout=3).stdout.encode()).hexdigest()==digest(runtime.public/'bundle.pem'), 'Public bundle catch-up',seconds=8)
        current=trusted.ask('transfer','post-ca-rollover')
        if current['outcome']!='transfer_succeeded':raise RuntimeError('Fresh TLS transfer failed after CA rollover')
        after=balances()['money_result']
        if after['version']!=2:raise RuntimeError('Rollover/revocation changed ledger unexpectedly')
        no_cert=unidentified.ask('unidentified')
        if no_cert['outcome']!='tls_rejected' or not no_cert['server_verified'] or 'money_http_status' in no_cert or balances()['money_result']!=after:
            raise RuntimeError('No-cert client could not verify rotated Money then demonstrate client rejection')
        passed('real-ca-rollover-and-public-bundle-refresh',{'initial_source':first,'rotated':rotation,'money_tls_verified':True,'public_no_cert_probe':no_cert,'atomic_public_file_mode':oct((runtime.public/'bundle.pem').stat().st_mode&0o777),'old_root_pruning_tested':False})

        run(compose+['restart','money'],env=env)
        reset={}
        def reset_ready():
            value=observer.ask('balances')
            if value.get('money_http_status')==200:reset.update(value['money_result']);return True
            return False
        wait_for(reset_ready,'Money restart',seconds=30)
        if reset['version']!=0 or reset['balances']!={'demo-a':100000,'demo-b':100000}:raise RuntimeError('Money restart did not reset ledger/history')
        again=trusted.ask('transfer','post-ca-rollover')
        if again['outcome']!='transfer_succeeded' or again['money_result']['replay'] or again['money_result']['version']!=1:
            raise RuntimeError('Money restart incorrectly retained in-memory deduplication')
        passed('T22-money-restart-process-lifetime-only-history',{'fresh_ledger':reset,'prior_key_is_new_after_restart':True,'spire_state_reset':False})

        # The new root has ample remaining lifetime; only the short-TTL caller
        # should expire during this bounded outage, leaving observer/Money valid.
        before_outage=trusted.ask('status');public_good=digest(runtime.public/'bundle.pem')
        stop_own(runtime.processes[0])
        wait_for(lambda:not runtime.publisher.available,'Publisher recognizes server outage',seconds=8)
        during=trusted.ask('transfer','outage-valid')
        if during['outcome']!='transfer_succeeded':raise RuntimeError('Valid cached identity did not work during server outage')
        after=balances()['money_result']
        wait_for(lambda:time.time()>before_outage['expires_at']+1,'Caller credential expiry during server outage',seconds=40)
        stale=trusted.ask('transfer','outage-expired')
        if stale['outcome']!='tls_rejected' or not stale['server_verified'] or 'money_http_status' in stale or balances()['money_result']!=after:
            raise RuntimeError('Expired source transferred during SPIRE outage')
        if digest(runtime.public/'bundle.pem')!=public_good:raise RuntimeError('Outage replaced the last authenticated public bundle')
        passed('T18-server-outage-valid-cache-then-expiry',{'valid_cached_attempt':during,'expired_attempt':stale,'ledger_unchanged_after_expiry':True,'last_verified_bundle_preserved':True})
        stop_own(runtime.processes[1])
        unavailable=unidentified.ask('unidentified')
        if unavailable['outcome']!='infrastructure_failure' or 'identity_eligibility' in unavailable or 'money_http_status' in unavailable or balances()['money_result']!=after:
            raise RuntimeError('Stopped agent was counted as an identity rejection')
        passed('T18-agent-outage-is-infrastructure-failure',{'result':unavailable,'ledger_unchanged':True})
        report['final_ledger']=after;report['status']='passed'
    except Exception as exc:
        report['failure']=str(exc);print('Backend lifecycle failed: '+str(exc),flush=True)
    finally:
        for client in clients:
            try:client.close()
            except Exception:report['status']='failed';report['cleanup_failure']='Fixture client did not stop'
        cleanup=run(compose+['down','--volumes'],env=env,check=False)
        if runtime is not None:runtime.close()
        image_cleanup=run(['docker','image','rm',image],check=False)
        shutil.rmtree(work)
        unchanged=retained_fingerprint()==before
        report['retained_ca_agent_state_unchanged']=unchanged
        report['cleanup']={'compose_exit_code':cleanup.returncode,'image_exit_code':image_cleanup.returncode,'ephemeral_identity_state_removed':not work.exists()}
        if cleanup.returncode or image_cleanup.returncode or not unchanged:report['status']='failed'
        report['finished_at']=datetime.now(timezone.utc).isoformat()
        evidence=ROOT/'docs/evidence/backend-lifecycle.json';evidence.write_text(json.dumps(report,indent=2)+'\n');evidence.chmod(0o644)
        print('Backend lifecycle checkpoint: '+report['status'],flush=True)
    return 0 if report['status']=='passed' else 1


if __name__=='__main__':raise SystemExit(main())
