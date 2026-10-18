#!/usr/bin/env python3
"""Exact normal application flows. Workload HTTP is plaintext signed payload inside outer ISTIO_MUTUAL; tcp port names keep Envoy from authorizing the JWS. Browser HTTPS stays a loopback exception."""
import json

import yaml
import kubernetes_demo as runtime


def policies(value):
    ns='poc-'+value['run']
    def meta(name):return {'name':name,'namespace':ns}
    items=[{'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':meta('default-deny'),
            'spec':{'podSelector':{},'policyTypes':['Ingress','Egress']}},
           {'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':meta('system-egress'),
            'spec':{'podSelector':{},'policyTypes':['Egress'],'egress':[
                {'to':[{'namespaceSelector':{'matchLabels':{'kubernetes.io/metadata.name':'kube-system'}},'podSelector':{'matchLabels':{'k8s-app':'kube-dns'}}}],
                 'ports':[{'protocol':'UDP','port':53},{'protocol':'TCP','port':53}]},
                {'to':[{'namespaceSelector':{'matchLabels':{'kubernetes.io/metadata.name':'istio-system'}}}],
                 'ports':[{'protocol':'TCP','port':15012}]}]}},
           {'apiVersion':'security.istio.io/v1','kind':'PeerAuthentication','metadata':meta('strict'),'spec':{'mtls':{'mode':'STRICT'}}},
           {'apiVersion':'security.istio.io/v1','kind':'AuthorizationPolicy','metadata':meta('default-deny'),'spec':{}},
           {'apiVersion':'security.istio.io/v1','kind':'PeerAuthentication','metadata':meta('browser-only'),
            'spec':{'selector':{'matchLabels':{'poc-role':'ui'}},'portLevelMtls':{'8443':{'mode':'DISABLE'}}}},
           {'apiVersion':'security.istio.io/v1','kind':'AuthorizationPolicy','metadata':meta('browser-only'),
            'spec':{'selector':{'matchLabels':{'poc-role':'ui'}},'rules':[{'to':[{'operation':{'ports':['8443']}}]}]}}]
    flows=[('money','money',8443,['ui','caller-trusted','caller-unidentified']),
           ('trusted-control','caller-trusted',9001,['ui']),('unidentified-control','caller-unidentified',9001,['ui']),
           ('kafka','kafka',9093,['money','caller-trusted','caller-unidentified','kafka-admin'])]
    for name,destination,port,sources in flows:
        selector={'matchLabels':{'poc-role':destination}}
        peers=[{'podSelector':{'matchLabels':{'poc-role':source}}} for source in sources]
        items.append({'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':meta('in-'+name),
                      'spec':{'podSelector':selector,'policyTypes':['Ingress'],'ingress':[{'from':peers,'ports':[{'protocol':'TCP','port':port}]}]}})
        for source in sources:
            items.append({'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':meta('out-'+source+'-'+name),
                          'spec':{'podSelector':{'matchLabels':{'poc-role':source}},'policyTypes':['Egress'],
                                  'egress':[{'to':[{'podSelector':selector}],'ports':[{'protocol':'TCP','port':port}]}]}})
        principals=['poc.internal/ns/'+ns+'/sa/'+source for source in sources]
        items.append({'apiVersion':'security.istio.io/v1','kind':'AuthorizationPolicy','metadata':meta('allow-'+name),
                      'spec':{'selector':selector,'rules':[{'from':[{'source':{'principals':principals}}],'to':[{'operation':{'ports':[str(port)]}}]}]}})
        items.append({'apiVersion':'networking.istio.io/v1','kind':'DestinationRule','metadata':meta(name),
                      'spec':{'host':name+'.'+ns+'.svc.cluster.local','trafficPolicy':{'portLevelSettings':[{'port':{'number':port},'tls':{'mode':'ISTIO_MUTUAL'}}]}}})
    items.append({'apiVersion':'v1','kind':'ResourceQuota','metadata':meta('owned-app-budget'),
                  'spec':{'hard':{'requests.cpu':'1300m','requests.memory':'1800Mi','limits.memory':'2600Mi','requests.ephemeral-storage':'2Gi','pods':'12'}}})
    return items


def main():
    value=runtime.descriptor()
    runtime.kube(value,'apply','-f','-',data=yaml.safe_dump_all(policies(value),sort_keys=False),timeout=90)
    print('Exact default-deny application policy installed; real positive/negative enforcement remains required.')


if __name__=='__main__':main()
