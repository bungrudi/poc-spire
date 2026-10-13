#!/usr/bin/env python3
"""Capture bounded startup metadata without API payloads, credentials, certificates or token values."""
import json
from pathlib import Path

import kubernetes_demo as runtime


def state_metadata(states):
    fields=('reason','exitCode','signal','startedAt','finishedAt','containerID')
    return {kind:{key:payload[key] for key in fields if key in payload}
            for kind,payload in (states or {}).items()}


def capture(value, stage):
    result={'run':value['run'],'stage':stage,'pods':[],'events':[],'capture_limits':'metadata only; no raw application logs or credential payloads'}
    for namespace in ('poc-'+value['run'],'spire-'+value['run']):
        try:
            pods=json.loads(runtime.kube(value,'-n',namespace,'get','pods','-o','json',timeout=8))['items']
            for pod in pods[:16]:
                result['pods'].append({'namespace':namespace,'name':pod['metadata']['name'],'uid':pod['metadata']['uid'],
                    'phase':pod.get('status',{}).get('phase'),'reason':pod.get('status',{}).get('reason'),
                    'regular_containers':[c['name'] for c in pod['spec']['containers']],
                    'init_containers':[c['name'] for c in pod['spec'].get('initContainers',[])],
                    'states':[{'name':s['name'],'ready':s.get('ready'),'restarts':s.get('restartCount'),
                               'state':state_metadata(s.get('state')), 'last_state':state_metadata(s.get('lastState'))}
                              for s in pod.get('status',{}).get('containerStatuses',[])]})
        except Exception:
            result.setdefault('unavailable',[]).append(namespace+' pod metadata')
        try:
            events=json.loads(runtime.kube(value,'-n',namespace,'get','events','-o','json',timeout=8))['items']
            for event in events[-16:]:
                entry={'namespace':namespace,'reason':event.get('reason'),'object':event.get('involvedObject',{}).get('name'),'count':event.get('count')}
                # Only the exact eviction resource category/volume name is retained from messages.
                message=event.get('message','')
                if event.get('reason')=='Evicted':
                    entry['emptydir_limit_exceeded']='exceeds the limit' in message and 'emptydir' in message.lower()
                    entry['guard_helpers_mentioned']='guard-bin' in message
                    entry['broker_helpers_mentioned']='helpers' in message
                    entry['memory_pressure']='memory' in message.lower()
                result['events'].append(entry)
        except Exception:
            result.setdefault('unavailable',[]).append(namespace+' event metadata')
    body=json.dumps(result).encode()
    if len(body)>65536:
        raise RuntimeError('bounded metadata capture exceeded limit')
    runtime.private_json(runtime.RUNTIME/'diagnostics'/('startup-'+value['run']+'.json'),result)
    return result
