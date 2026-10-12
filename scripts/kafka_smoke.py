#!/usr/bin/env python3
"""First isolated real-SPIRE/Kafka UI checkpoint, preserving the public stack."""
import json
import time
from attestation import ROOT
from kafka_demo import KafkaStack,require_complete_cleanup

def main():
    report={'phase':'isolated-kafka-smoke','status':'running','checks':{},'public_deployment_changed':False,'full_acceptance':False}
    stack=KafkaStack()
    try:
        stack.start()
        report['checks']['complete_readiness']=True
        for scenario in ['trusted','unidentified','kafka-trusted','kafka-unidentified','kafka-tampered']:
            value,key=stack.attempt(scenario)
            if scenario.startswith('kafka-'):
                expected='transfer_succeeded' if scenario=='kafka-trusted' else 'money_rejected'
                evidence=value['kafka_evidence']
                if value['status']!='done' or evidence['outcome']!=expected:
                    raise RuntimeError('Unexpected fixed Kafka outcome: '+scenario)
                if scenario!='kafka-trusted' and any(q.get('receipt',{}).get('verified_producer_id') for q in evidence['money_queries']):
                    raise RuntimeError('Invalid record acquired producer attribution')
                report['checks'][scenario]={'status':value['status'],'outcome':evidence['outcome'],'query_http_statuses':[q['query_http_status'] for q in evidence['money_queries']],'receipt_reasons':[q.get('receipt',{}).get('reason') for q in evidence['money_queries']]}
            else:
                expected='transfer_succeeded' if scenario=='trusted' else 'tls_rejected'
                if value['status']!='done' or value['caller_evidence']['outcome']!=expected:raise RuntimeError('Unexpected Sync regression')
                report['checks'][scenario]={'status':value['status'],'outcome':expected}
        report['status']='passed_bounded_smoke'
    except Exception as error:
        report['status']='failed'
        report['failed_stage']=stack.stage
        report['error_type']=type(error).__name__
        report['reason']=str(error)[-2000:]
        if stack.bootstrap_diagnostic:report['broker_bootstrap_diagnostic']=stack.bootstrap_diagnostic
        if stack.up:
            from attestation import run
            logs=run(stack.compose+['logs','--no-color','--tail','80','kafka'],env=stack.env,check=False).stdout
            report['broker_errors']=[line[-300:] for line in logs.splitlines() if any(word in line for word in ['ERROR','Exception','Permission','Caused by'])][-20:]
        # Raw SPIRE diagnostics and native parent IDs may contain a join token.
        # Keep them private; public evidence contains no exception diagnostics.
        raise RuntimeError('Isolated smoke failed; inspect protected run diagnostics') from None
    finally:
        report['cleanup']=stack.close()
        if not report['cleanup'].get('owned_cleanup_complete') or report['cleanup'].get('errors'):
            report['status']='failed_cleanup'
        report['finished_at_utc']=time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime())
        (ROOT/'docs/evidence/kafka-smoke-current.json').write_text(json.dumps(report,indent=2)+'\n')
        require_complete_cleanup(report['cleanup'])

if __name__=='__main__':main()
