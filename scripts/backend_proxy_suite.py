#!/usr/bin/env python3
"""Opaque application-proxy transport faults, not kernel/IP packet emulation."""
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import uuid

from attestation import ROOT, VERSION, run, digest, wait_for
from identity import IdentityRuntime
from backend_suite import Client, retained_fingerprint


class TargetClient(Client):
    def __init__(self, compose, env, service, target, expected=None):
        args = compose + ['exec', '-T', service, '/backend-client', '-target', target]
        args += ['-expected', expected] if expected else ['-unidentified']
        self.process = subprocess.Popen(args, cwd=ROOT, env=env, text=True,
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)


def main():
    os.umask(0o077)
    before = retained_fingerprint()
    work = Path(tempfile.mkdtemp(prefix='backend-proxy-', dir=ROOT/'.runtime'))
    project = 'poc-spire-proxy-'+uuid.uuid4().hex[:10]
    image = project+':test'
    env = os.environ.copy()
    env.update(GOTOOLCHAIN='local', CGO_ENABLED='0', POC_PROJECT=project,
        POC_IMAGE=image, POC_SOCKET_DIR=str(work/'workload'))
    compose = ['docker','compose','-f',str(ROOT/'infra/backend/compose.yaml'),
        '-f',str(ROOT/'infra/backend/proxy.compose.yaml')]
    runtime=None; clients=[]
    report={'phase':'backend-proxy-loss', 'status':'failed', 'spire_version':VERSION,
        'started_at':datetime.now(timezone.utc).isoformat(), 'tests':[],
        'fault_scope':'opaque TLS-record loss inside an application TCP proxy',
        'kernel_ip_packet_loss_tested':False, 'test_identity_state':'ephemeral',
        'automatic_expired_root_sweep':{'status':'unrun',
            'reason':'SPIRE 1.14.0 fixes 24-hour post-expiry grace and six-hour sweep independently of CA TTL; no supported accelerated configuration',
            'source':'https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/ca/manager/manager.go',
            'timer_source':'https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/ca/rotator/rotator.go'}}
    def passed(case, evidence):
        report['tests'].append({'case':case,**evidence})
        print(case+': passed',flush=True)
    try:
        print('Building isolated opaque TLS proxy fixtures',flush=True)
        names=['backend-money','backend-client','backend-proxy']
        for name in names:
            run(['go','build','-trimpath','-o',str(work/name),'./cmd/'+name],env=env)
            (work/name).chmod(0o555)
        shutil.copy(ROOT/'infra/backend/proxy.Dockerfile',work/'Dockerfile')
        run(['docker','build','--network','none','-t',image,str(work)])
        image_digest=run(['docker','image','inspect',image,'--format','{{.Id}}']).stdout.strip()
        report.update(binary_sha256={name:digest(work/name) for name in names},
            image_config_digest=image_digest,
            compose_sha256={name:digest(ROOT/'infra/backend'/name) for name in ['compose.yaml','proxy.compose.yaml']},
            versions={'go':run(['go','version']).stdout.strip(),
                'go_spiffe_sdk':run(['go','list','-m','github.com/spiffe/go-spiffe/v2'],env=env).stdout.strip(),
                'docker_engine':run(['docker','version','--format','{{.Server.Version}}']).stdout.strip(),
                'compose':run(['docker','compose','version','--short']).stdout.strip()})
        runtime=IdentityRuntime(ephemeral_root=work,ca_ttl=600);runtime.start()
        specs=[('spiffe://poc.internal/money','backend-money','backend-money'),
            ('spiffe://poc.internal/caller/trusted','backend-client','backend-trusted'),
            ('spiffe://poc.internal/ui','backend-client','backend-observer')]
        for identity,binary,role in specs:
            runtime.register(identity,work/binary,image_digest,role,ttl=90)
        report['registration_policy']=[{'identity':identity,'uid':1000,
            'binary_sha256':digest(work/binary),'image_config_digest':image_digest,
            'deployment_role':role,'ttl_seconds':90,'parent':'exact attested parent (redacted)'}
            for identity,binary,role in specs]
        run(compose+['up','-d'],env=env)
        observer=Client(compose,env,'observer','spiffe://poc.internal/ui');clients.append(observer)
        def balance():
            value=observer.ask('balances')
            if value.get('money_http_status')!=200 or not value.get('server_verified'):
                raise RuntimeError('Observer cannot verify/read Money')
            return value['money_result']
        initial=balance()
        def proxy_events():
            output=run(compose+['logs','--no-log-prefix','proxy'],env=env).stdout
            values=[]
            for line in output.splitlines():
                try:values.append(json.loads(line))
                except json.JSONDecodeError:pass
            return values
        wait_for(lambda:any(e.get('event')=='ready' for e in proxy_events()),'Opaque proxy ready',seconds=15)
        cid=run(compose+['ps','-q','proxy'],env=env).stdout.strip()
        config=json.loads(run(['docker','inspect',cid]).stdout)[0]
        if config['Mounts'] or config['HostConfig']['PortBindings'] or config['HostConfig']['Privileged'] or config['HostConfig']['CapAdd']:
            raise RuntimeError('Proxy gained mounts, public ports or privileges')
        if not config['HostConfig']['ReadonlyRootfs'] or config['Config']['User']!='1000:1000' or config['HostConfig']['CapDrop']!=['ALL']:
            raise RuntimeError('Proxy hardening differs from restricted fixture')
        report['proxy_boundaries']={'workload_or_admin_socket_mounts':False,'published_ports':False,
            'privileged':False,'capabilities_added':False,'read_only':True,'uid':1000,
            'tls_terminated':False,'maximum_connections':16,'connection_deadline_seconds':12}
        def client(port, identified=True):
            c=TargetClient(compose,env,'trusted' if identified else 'unidentified',f'https://proxy:{port}',
                'spiffe://poc.internal/caller/trusted' if identified else None)
            clients.append(c);return c
        good=client(9443)
        positive=good.ask('transfer','proxy-positive')
        if positive['outcome']!='transfer_succeeded' or not positive['server_verified'] or positive['money_result']['version']!=1:
            raise RuntimeError('Opaque pass-through did not preserve direct mutual identity verification')
        after=balance()
        negative=client(9443,False).ask('unidentified','proxy-no-cert')
        if negative['outcome']!='tls_rejected' or not negative['server_verified'] or negative.get('identity_eligibility')!='no_matching_identity' or 'money_http_status' in negative or balance()!=after:
            raise RuntimeError('Opaque pass-through bypassed client identity or changed ledger')
        passed('opaque-pass-through-preserves-money-mtls',{'positive':positive,'unidentified':negative,'ledger_unchanged_after_no_cert':True})
        for port,mode,direction in [(9444,'discard-client','client'),(9445,'discard-server','server')]:
            key='proxy-'+mode
            lost=client(port).ask('transfer',key)
            if lost['outcome']!='infrastructure_failure' or lost['server_verified'] or 'money_http_status' in lost or balance()!=after:
                raise RuntimeError('Handshake loss misclassified or changed ledger: '+mode)
            def dropped():
                return any(e.get('event')=='connection_finished' and e.get('mode')==mode and e.get(direction,{}).get('dropped_records',0)>0 for e in proxy_events())
            wait_for(dropped,'Directional drop evidence',seconds=15)
            retry=good.ask('transfer',key)
            if retry['outcome']!='transfer_succeeded' or retry['money_result']['replay'] or retry['money_result']['version']!=after['version']+1:
                raise RuntimeError('Retry after handshake loss did not commit exactly once')
            after=balance()
            replay=good.ask('transfer',key)
            if replay['outcome']!='transfer_succeeded' or not replay['money_result']['replay'] or balance()!=after:
                raise RuntimeError('Same key retry duplicated a handshake-loss transfer')
            passed(mode+'-handshake-loss',{'fault':lost,'discarded_tls_records_observed':True,
                'fault_ledger_unchanged':True,'restored_same_key_retry':retry,'stable_replay':True})
        lost=client(9446).ask('transfer','proxy-response-loss')
        if lost['outcome']!='unknown_commit' or not lost['server_verified'] or 'money_http_status' in lost:
            raise RuntimeError('Opaque response-record loss did not preserve server verification/unknown commit')
        committed=balance()
        if committed['version']!=after['version']+1:
            raise RuntimeError('Response-record loss did not follow an actual Money commit')
        mode='discard-server-after-handshake'
        wait_for(lambda:any(e.get('event')=='connection_finished' and e.get('mode')==mode and e.get('server',{}).get('dropped_records',0)>0 for e in proxy_events()),'Post-commit opaque drop evidence',seconds=15)
        retry=good.ask('transfer','proxy-response-loss')
        if retry['outcome']!='transfer_succeeded' or not retry['money_result']['replay'] or balance()!=committed:
            raise RuntimeError('Retry duplicated the response-lost committed transfer')
        passed('T16-opaque-encrypted-response-loss-after-commit',{'fault':lost,
            'discarded_tls_records_observed':True,'actual_commit_version':committed['version'],
            'same_key_retry':retry,'ledger_unchanged_on_retry':True,'money_response_fixture_drop_used':False})
        report['final_ledger']=committed
        report['proxy_record_counts']=[e for e in proxy_events() if e.get('event')=='connection_finished']
        report['status']='passed'
    except Exception as exc:
        report['failure']=str(exc);print('Proxy suite failed: '+str(exc),flush=True)
    finally:
        for c in clients:
            try:c.close()
            except Exception:report['status']='failed';report['cleanup_failure']='Fixture client did not stop'
        cleanup=run(compose+['down','--volumes'],env=env,check=False)
        if runtime is not None:runtime.close()
        image_cleanup=run(['docker','image','rm',image],check=False)
        shutil.rmtree(work)
        unchanged=retained_fingerprint()==before
        report['retained_ca_agent_state_unchanged']=unchanged
        report['cleanup']={'compose_exit_code':cleanup.returncode,'image_exit_code':image_cleanup.returncode,
            'ephemeral_identity_state_removed':not work.exists()}
        if cleanup.returncode or image_cleanup.returncode or not unchanged:report['status']='failed'
        report['finished_at']=datetime.now(timezone.utc).isoformat()
        evidence=ROOT/'docs/evidence/backend-proxy.json'
        evidence.write_text(json.dumps(report,indent=2)+'\n');evidence.chmod(0o644)
        print('Backend proxy checkpoint: '+report['status'],flush=True)
    return 0 if report['status']=='passed' else 1


if __name__=='__main__':raise SystemExit(main())
