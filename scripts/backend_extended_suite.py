#!/usr/bin/env python3
"""Isolated root removal, reviewed registration change, expired-node recovery."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import uuid

from attestation import ROOT, VERSION, run, wait_for, digest
from backend_suite import Client, stop_own, retained_fingerprint
from identity import IdentityRuntime, private_write


class FixtureClient(Client):
    def __init__(self, compose, env, service, expected=None, executable='/backend-client'):
        args=compose+['exec','-T',service,executable]
        args+=['-expected',expected] if expected else ['-unidentified']
        self.process=subprocess.Popen(args,cwd=ROOT,env=env,text=True,stdin=subprocess.PIPE,
                                      stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)


def main():
    os.umask(0o077)
    before=retained_fingerprint()
    work=Path(tempfile.mkdtemp(prefix='backend-extended-',dir=ROOT/'.runtime'))
    project='poc-spire-extended-'+uuid.uuid4().hex[:10];image=project+':test'
    env=os.environ.copy();env.update(GOTOOLCHAIN='local',CGO_ENABLED='0',POC_PROJECT=project,
        POC_IMAGE=image,POC_SOCKET_DIR=str(work/'workload'))
    compose=['docker','compose','-f',str(ROOT/'infra/backend/compose.yaml')]
    runtime=None;clients=[]
    report={'phase':'backend-extended','status':'failed','spire_version':VERSION,'ca_ttl_seconds':600,
            'started_at':datetime.now(timezone.utc).isoformat(),'test_identity_state':'ephemeral','tests':[],
            'unrun':['automatic expired-root sweep: 24-hour post-expiry safety threshold and six-hour timer',
                     'production 24-hour CA rollover duration','arbitrary packet-loss injection',
                     'upstream-authority and federated/multi-agent layouts','UI/full T01–T23 acceptance']}
    def passed(case,evidence):
        report['tests'].append({'case':case,**evidence});print(case+': passed',flush=True)
    try:
        print('Building isolated root/registration/recovery fixtures',flush=True)
        for name in ['backend-money','backend-client','backend-client-v2']:
            cmd='backend-client' if name.endswith('-v2') else name
            args=['go','build','-trimpath','-o',str(work/name)]
            if name.endswith('-v2'):args+=['-ldflags','-X main.buildMarker=v2']
            run(args+['./cmd/'+cmd],env=env);(work/name).chmod(0o555)
        if digest(work/'backend-client')==digest(work/'backend-client-v2'):
            raise RuntimeError('Reviewed rebuild fixture has identical executable digest')
        shutil.copy(ROOT/'infra/backend/extended.Dockerfile',work/'Dockerfile')
        run(['docker','build','--network','none','-t',image,str(work)])
        image_digest=run(['docker','image','inspect',image,'--format','{{.Id}}']).stdout.strip()
        report['binary_sha256']={name:digest(work/name) for name in ['backend-money','backend-client','backend-client-v2']}
        report['image_config_digest']=image_digest;report['compose_sha256']=digest(ROOT/'infra/backend/compose.yaml')
        report['versions']={'go':run(['go','version']).stdout.strip(),
            'go_spiffe_sdk':run(['go','list','-m','github.com/spiffe/go-spiffe/v2'],env=env).stdout.strip(),
            'docker_engine':run(['docker','version','--format','{{.Server.Version}}']).stdout.strip(),
            'compose':run(['docker','compose','version','--short']).stdout.strip()}
        runtime=IdentityRuntime(ephemeral_root=work,ca_ttl=600);runtime.start()
        specs={'spiffe://poc.internal/money':('backend-money','backend-money',90),
               'spiffe://poc.internal/caller/trusted':('backend-client','backend-trusted',90),
               'spiffe://poc.internal/ui':('backend-client','backend-observer',90)}
        entries={identity:runtime.register(identity,work/binary,image_digest,role,ttl=ttl)
                 for identity,(binary,role,ttl) in specs.items()}
        # A real exact-entry reuse proves lifetime parsing against this release.
        if runtime.register('spiffe://poc.internal/money',work/'backend-money',image_digest,'backend-money',ttl=90)!=entries['spiffe://poc.internal/money']:
            raise RuntimeError('Exact native entry reuse changed ID')
        run(compose+['up','-d'],env=env)
        observer=FixtureClient(compose,env,'observer','spiffe://poc.internal/ui');clients.append(observer)
        v1=FixtureClient(compose,env,'trusted','spiffe://poc.internal/caller/trusted');clients.append(v1)
        v2_probe=FixtureClient(compose,env,'trusted',executable='/backend-client-v2');clients.append(v2_probe)
        unidentified=FixtureClient(compose,env,'unidentified');clients.append(unidentified)
        def balances():
            value=observer.ask('balances')
            if value.get('money_http_status')!=200 or not value.get('server_verified'):
                raise RuntimeError('Observer cannot verify/read Money')
            return value
        initial=balances()['money_result']
        if initial['version']!=0:raise RuntimeError('Fresh Money ledger was not zero')
        def show_entries():return json.loads(run([runtime.server,'entry','show','-socketPath',runtime.server_socket,'-output','json']).stdout)['entries']
        def entry_snapshot():return next(e for e in show_entries() if e['id']==entries['spiffe://poc.internal/caller/trusted'])
        snapshot=entry_snapshot()
        denied=v2_probe.ask('fresh')
        if denied['outcome']!='no_matching_identity' or denied.get('grpc_code')!='PermissionDenied':
            raise RuntimeError('Rebuild received identity under old executable registration')
        try:runtime.register('spiffe://poc.internal/caller/trusted',work/'backend-client-v2',image_digest,'backend-trusted',ttl=90)
        except RuntimeError as exc:
            if 'explicit digest update required' not in str(exc):raise
        else:raise RuntimeError('Runtime silently rewrote executable policy')
        try:runtime.register('spiffe://poc.internal/caller/trusted',work/'backend-client',image_digest,'backend-trusted',ttl=30)
        except RuntimeError as exc:
            if 'explicit lifetime update required' not in str(exc):raise
        else:raise RuntimeError('Runtime silently reused wrong lifetime policy')
        if entry_snapshot()!=snapshot:raise RuntimeError('Rejected policy update changed native entry')

        def update(identity,binary,role,ttl,parent):
            args=[runtime.server,'entry','update','-socketPath',runtime.server_socket,'-entryID',entries[identity],
                  '-parentID',parent,'-spiffeID',identity,'-x509SVIDTTL',str(ttl),'-output','json']
            selectors=['unix:uid:1000','unix:sha256:'+digest(work/binary),
                       'docker:label:poc-spire.role:'+role,'docker:image_config_digest:'+image_digest]
            for selector in selectors:args+=['-selector',selector]
            run(args)
        update('spiffe://poc.internal/caller/trusted','backend-client-v2','backend-trusted',90,runtime.metadata['parent'])
        new_entry=entry_snapshot()
        before_nonhash=[s for s in snapshot['selectors'] if not(s['type']=='unix' and s['value'].startswith('sha256:'))]
        after_nonhash=[s for s in new_entry['selectors'] if not(s['type']=='unix' and s['value'].startswith('sha256:'))]
        if before_nonhash!=after_nonhash or snapshot['parent_id']!=new_entry['parent_id'] or snapshot['x509_svid_ttl']!=new_entry['x509_svid_ttl']:
            raise RuntimeError('Reviewed digest update broadened deployment/parent/lifetime')
        observation={}
        def updated():
            observation.update(v2_probe.ask('fresh'));return observation.get('outcome')=='identity_issued'
        wait_for(updated,'Reviewed binary update propagation',seconds=25)
        wait_for(lambda:v1.ask('fresh')['outcome']=='no_matching_identity','Old executable no longer eligible',seconds=25)
        v2=FixtureClient(compose,env,'trusted','spiffe://poc.internal/caller/trusted',executable='/backend-client-v2');clients.append(v2)
        if v2.ask('status').get('build_marker')!='v2':raise RuntimeError('Expected rebuilt executable did not run')
        transfer=v2.ask('transfer','reviewed-rebuild')
        if transfer['outcome']!='transfer_succeeded' or transfer['money_result']['version']!=1:
            raise RuntimeError('Reviewed binary could not transfer exactly once')
        passed('T21-explicit-digest-update-and-no-silent-lifetime-reuse',{'old_policy_rebuild_denied':denied,
            'policy_guard_read_only':True,'reviewed_update_preserves_other_selectors_parent_ttl':True,
            'old_executable_fresh_issuance_denied':True,'new_executable_verified_transfer':transfer,
            'instant_cached_certificate_revocation_claimed':False})

        captured=v2.ask('capture');old=captured['authority_key_id']
        root_base=[runtime.server,'localauthority','x509']
        def authority(action,authority_id=None,check=True):
            args=root_base+[action,'-socketPath',runtime.server_socket,'-output','json']
            if authority_id:args+=['-authorityID',authority_id]
            return run(args,check=check)
        states=json.loads(authority('show').stdout)
        # Protobuf JSON uses active/prepared/old states, each with authority_id.
        active=states['active']['authority_id']
        if active.lower()!=old:raise RuntimeError('Frozen credential did not use current native authority')
        original_bundle=digest(runtime.public/'bundle.pem')
        if authority('taint',active,check=False).returncode==0 or authority('revoke',active,check=False).returncode==0:
            raise RuntimeError('Native API permitted active root taint/revocation')
        if digest(runtime.public/'bundle.pem')!=original_bundle:raise RuntimeError('Rejected active-root operation changed bundle')
        prepared=json.loads(authority('prepare').stdout)['prepared_authority']['authority_id']
        wait_for(lambda:prepared.lower() in observer.ask('status').get('bundle_authority_ids',[]),
                 'Prepared authority distributed before activation',seconds=20)
        authority('activate',prepared)
        authority('taint',active)
        renewed={}
        def renewed_on_new_root():
            state=v2.ask('status');server=balances()
            ok=state['authority_key_id']==prepared.lower() and server['server_authority_key_id']==prepared.lower()
            if ok:renewed.update(source=state,server_authority_key_id=server['server_authority_key_id'])
            return ok
        wait_for(renewed_on_new_root,'Tainted old authority workload rotation',seconds=35)
        if captured['expires_at']<=time.time():raise RuntimeError('Frozen credential expired before root-removal test')
        frozen_before=v2.ask('frozen-transfer','old-root-before-revoke')
        if frozen_before.get('money_http_status')!=200:raise RuntimeError('Taint was incorrectly treated as instant certificate revocation')
        before_removal=balances()['money_result']
        authority('revoke',active)
        def root_removed():
            state=observer.ask('status')
            pem=run([runtime.server,'bundle','show','-socketPath',runtime.server_socket,'-format','pem'],timeout=3).stdout
            return old not in state.get('bundle_authority_ids',[]) and state.get('bundle_roots')==1 and hashlib.sha256(pem.encode()).hexdigest()==digest(runtime.public/'bundle.pem')
        wait_for(root_removed,'Revoked old root removed from SDK/public bundles',seconds=20)
        frozen_after=v2.ask('frozen-transfer','old-root-after-revoke')
        if captured['expires_at']<=time.time() or frozen_after.get('money_http_status')!=401 or not frozen_after['server_verified'] or balances()['money_result']!=before_removal:
            raise RuntimeError('Still-valid old-root credential survived native root revocation')
        passed('old-root-taint-rotation-and-revocation-propagation',{'active_root_operations_refused':True,
            'captured_credential':captured,'rotated_workload':renewed,'taint_alone_does_not_revoke':True,
            'root_removed_from_sdk_and_atomic_public_file':True,'still_valid_old_root_peer_rejected':frozen_after,
            'ledger_unchanged_after_removal':True,'automatic_expired_root_pruning_tested':False})

        # Hold the server down beyond every cached workload and node credential.
        # This is a real clock wait; no host clock or binary is modified.
        trusted_expiry=v2.ask('status')['expires_at'];observer_expiry=observer.ask('status')['expires_at']
        before_outage=balances()['money_result'];parent_before=runtime.metadata['parent']
        ca_keys_before=digest(runtime.state/'server/keys.json');bundle_before=digest(runtime.public/'bundle.pem')
        server_args=[runtime.server,'run','-config',str(runtime.state/'server.conf')]
        stop_own(runtime.processes[0]);started=time.time()
        wait_for(lambda:not runtime.publisher.available,'Publisher observes extended server outage',seconds=8)
        deadline=max(trusted_expiry,observer_expiry,started+95)+2
        wait_for(lambda:time.time()>deadline,'All cached credentials expire during extended outage',seconds=135)
        expired=v2.ask('transfer','long-outage-rejected')
        if expired['outcome']=='transfer_succeeded' or 'money_http_status' in expired:
            raise RuntimeError('Expired workload credentials succeeded during extended outage')
        outage_seconds=int(time.time()-started)
        # Restart only our server using the same disk CA, then wait for the
        # running agent to demonstrate whether expired node renewal works.
        restored_server=runtime.spawn(server_args,'server-recovery')
        wait_for(lambda:restored_server.poll() is None and run([runtime.server,'healthcheck','-socketPath',runtime.server_socket],check=False).returncode==0,
                 'Same-CA server restoration',seconds=20)
        wait_for(lambda:runtime.publisher.available,'Publisher recovers after server restart',seconds=8)
        time.sleep(3)
        recovered_without_enrollment=v2.ask('fresh')
        if recovered_without_enrollment['outcome']=='identity_issued':
            raise RuntimeError('Pinned expired join-token agent recovered automatically; revise the recovery expectation')
        # A normal runtime restart must fail closed, not silently issue a token.
        runtime.close();runtime=None
        attempt=IdentityRuntime(ephemeral_root=work,ca_ttl=600)
        runtime=attempt
        automatic_failed=False
        try:attempt.start()
        except RuntimeError:automatic_failed=True
        if not automatic_failed:raise RuntimeError('Expired-node normal restart did not require explicit recovery')
        if (attempt.state/'join-token.once').exists() or digest(attempt.state/'server/keys.json')!=ca_keys_before:
            raise RuntimeError('Normal expired-node restart reset CA or minted a join token')
        attempt.close();runtime=None
        passed('extended-outage-expires-node-and-workload-no-automatic-reenrollment',{
            'outage_seconds':outage_seconds,'expired_attempt':expired,
            'fresh_attempt_before_reenrollment':recovered_without_enrollment,
            'normal_restart_failed_closed':True,'new_join_token_automatically_created':False,'ca_keys_unchanged':True})

        # This recovery is explicit and test-only. It deletes only this suite's
        # own ephemeral agent state, never the approved retained agent directory.
        state=work/'identity';agent=state/'agent'
        if state.parent!=work or work.parent!=ROOT/'.runtime' or not work.name.startswith('backend-extended-'):
            raise RuntimeError('Ephemeral recovery path ownership guard failed')
        metadata=json.loads((state/'runtime.json').read_text());metadata.pop('parent');private_write(state/'runtime.json',json.dumps(metadata))
        shutil.rmtree(agent);agent.mkdir(mode=0o700)
        runtime=IdentityRuntime(ephemeral_root=work,ca_ttl=600);runtime.start()
        new_parent=runtime.metadata['parent']
        if new_parent==parent_before:raise RuntimeError('Explicit one-use re-enrollment retained the expired node parent')
        specs['spiffe://poc.internal/caller/trusted']=('backend-client-v2','backend-trusted',90)
        for identity,(binary,role,ttl) in specs.items():update(identity,binary,role,ttl,new_parent)
        recovered={}
        def same_ledger_recovered():
            value=observer.ask('balances')
            if value.get('money_http_status')==200 and value.get('server_verified'):
                recovered.update(value['money_result']);return True
            return False
        wait_for(same_ledger_recovered,'Live workload source recovery after explicit exact-parent updates',seconds=35)
        if recovered!=before_outage:raise RuntimeError('Outage recovery changed Money ledger or restarted it')
        retry=v2.ask('transfer','reviewed-rebuild')
        if retry['outcome']!='transfer_succeeded' or not retry['money_result']['replay'] or balances()['money_result']!=before_outage:
            raise RuntimeError('Post-recovery retry duplicated a pre-outage transfer')
        if digest(runtime.state/'server/keys.json')!=ca_keys_before or digest(runtime.public/'bundle.pem')!=bundle_before or (runtime.state/'join-token.once').exists():
            raise RuntimeError('Explicit node recovery reset CA/trust or retained bootstrap token')
        passed('explicit-ephemeral-node-reenrollment-and-live-source-recovery',{
            'same_ca_and_bundle_preserved':True,'new_parent_exactly_applied_to_all_existing_entries':True,
            'one_use_token_removed':True,'live_sources_recovered_without_app_restart':True,
            'money_ledger_and_history_preserved':True,'retry':retry,'only_fixture_agent_state_replaced':True})
        report['final_ledger']=before_outage;report['status']='passed'
    except Exception as exc:
        report['failure']=str(exc);print('Extended lifecycle failed: '+str(exc),flush=True)
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
        evidence=ROOT/'docs/evidence/backend-extended.json';evidence.write_text(json.dumps(report,indent=2)+'\n');evidence.chmod(0o644)
        print('Extended lifecycle checkpoint: '+report['status'],flush=True)
    return 0 if report['status']=='passed' else 1


if __name__=='__main__':raise SystemExit(main())
