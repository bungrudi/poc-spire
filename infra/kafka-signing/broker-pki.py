#!/usr/bin/env python3
"""Temporary standard TLS PKI; no SPIFFE policy or host trust-store changes."""
import json
import os
from pathlib import Path
import re
import subprocess


def command(args):
    result = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=20)
    if result.returncode:
        raise RuntimeError('Temporary certificate generation failed')


def generate(work, stream, *, controller_local=False, browser_dns=None):
    if browser_dns is not None and not re.fullmatch(r'[a-z0-9-]+\.cloudfront\.net', browser_dns):
        raise ValueError('browser DNS must be the assigned CloudFront hostname')
    os.umask(0o077)
    work = Path(work)
    ca = work / 'pki-ca'
    ca.mkdir(mode=0o700)
    command(['openssl', 'genpkey', '-algorithm', 'EC', '-pkeyopt', 'ec_paramgen_curve:P-256', '-out', str(ca/'ca.key')])
    command(['openssl', 'req', '-new', '-x509', '-key', str(ca/'ca.key'), '-out', str(ca/'ca.pem'), '-days', '1', '-subj', '/CN=poc-kafka-fixture-ca', '-addext', 'basicConstraints=critical,CA:TRUE', '-addext', 'keyUsage=critical,keyCertSign,cRLSign'])
    browser_ca=work/'browser-ca';browser_ca.mkdir(mode=0o700)
    command(['openssl','genpkey','-algorithm','EC','-pkeyopt','ec_paramgen_curve:P-256','-out',str(browser_ca/'ca.key')])
    command(['openssl','req','-new','-x509','-key',str(browser_ca/'ca.key'),'-out',str(browser_ca/'ca.pem'),'-days','1','-subj','/CN=poc-browser-fixture-ca','-addext','basicConstraints=critical,CA:TRUE','-addext','keyUsage=critical,keyCertSign,cRLSign'])
    roles = [('broker-pki','kafka-broker',True),('admin-pki','kafka-admin',False),('money-pki','kafka-money',False),('trusted-pki','kafka-producer',False),('untrusted-pki','kafka-unidentified',False),('probe-pki','kafka-probe',False),('rogue-pki','kafka-rogue',False),('browser','poc-kafka-browser',True)]
    for index,(directory,cn,server) in enumerate(roles,2):
        target=work/directory;target.mkdir(mode=0o700)
        issuer=browser_ca if directory=='browser' else ca
        command(['openssl','genpkey','-algorithm','EC','-pkeyopt','ec_paramgen_curve:P-256','-out',str(target/'client.key')])
        command(['openssl','req','-new','-key',str(target/'client.key'),'-out',str(target/'request.csr'),'-subj','/CN='+cn])
        browser_san='subjectAltName=DNS:'+browser_dns+'\n' if browser_dns else 'subjectAltName=IP:127.0.0.1,DNS:localhost\n'
        ext=target/'extensions.conf';ext.write_text('basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage='+('serverAuth' if directory=='browser' else 'serverAuth,clientAuth' if server else 'clientAuth')+'\n'+(('subjectAltName=DNS:kafka,DNS:localhost,IP:127.0.0.1\n' if controller_local else 'subjectAltName=DNS:kafka\n') if directory=='broker-pki' else browser_san if directory=='browser' else ''))
        command(['openssl','x509','-req','-in',str(target/'request.csr'),'-CA',str(issuer/'ca.pem'),'-CAkey',str(issuer/'ca.key'),'-set_serial',str(index),'-out',str(target/'client.pem'),'-days','1','-extfile',str(ext)])
        (target/'ca.pem').write_bytes((issuer/'ca.pem').read_bytes())
        if directory in {'broker-pki','browser'}:
            (target/('broker.pem' if directory=='broker-pki' else 'browser.pem')).write_bytes((target/'client.key').read_bytes()+(target/'client.pem').read_bytes())
        if directory not in {'broker-pki','browser','admin-pki'}:
            value={'stream_id':stream,'brokers':['kafka:9093'],'ca':'/kafka/ca.pem','certificate':'/kafka/client.pem','key':'/kafka/client.key'}
            if directory in {'trusted-pki','untrusted-pki'}:value['state_dir']='/kafka-state'
            (target/'client.json').write_text(json.dumps(value))
        for path in target.iterdir():path.chmod(0o600)
    return work/'browser/ca.pem'
