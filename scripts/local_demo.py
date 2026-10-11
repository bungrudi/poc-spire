#!/usr/bin/env python3
"""Foreground loopback UI; own ephemeral trust/containers removed on exit."""
import argparse
import json
import os
from pathlib import Path
import select
import shutil
import socket
import socketserver
import threading
import tempfile
import time
import urllib.request
import uuid

from attestation import ROOT, run, digest, wait_for
from identity import IdentityRuntime
from backend_suite import retained_fingerprint


class Bridge(socketserver.ThreadingTCPServer):
    allow_reuse_address = False
    daemon_threads = True
    def __init__(self, port, path):
        self.path=path
        self.slots=threading.BoundedSemaphore(16)
        super().__init__(('127.0.0.1',port),BridgeConnection)
        self.connections=set();self.lock=threading.Lock()
    def close_connections(self):
        with self.lock:
            for s in list(self.connections):
                try:s.shutdown(socket.SHUT_RDWR)
                except OSError:pass
                s.close()


class BridgeConnection(socketserver.BaseRequestHandler):
    def handle(self):
        if not self.server.slots.acquire(blocking=False):return
        upstream=socket.socket(socket.AF_UNIX)
        pair=[self.request,upstream]
        try:
            upstream.settimeout(2);upstream.connect(self.server.path)
            with self.server.lock:self.server.connections.update(pair)
            deadline=time.monotonic()+15
            for s in pair:s.setblocking(False)
            while time.monotonic()<deadline:
                ready,_,_=select.select(pair,[],[],1)
                for source in ready:
                    data=source.recv(16384)
                    if not data:return
                    target=pair[1] if source is pair[0] else pair[0]
                    target.settimeout(2);target.sendall(data);target.setblocking(False)
        except OSError:pass
        finally:
            with self.server.lock:
                for s in pair:self.server.connections.discard(s)
            upstream.close();self.server.slots.release()


class LocalStack:
    def __init__(self,port=8080):
        if not 1024<=port<=65535:raise ValueError('Local demo requires an unprivileged port')
        self.before=retained_fingerprint()
        self.work=Path(tempfile.mkdtemp(prefix='backend-ui-',dir=ROOT/'.runtime'))
        self.project='poc-spire-local-ui-'+uuid.uuid4().hex[:10]
        self.image=self.project+':demo';self.port=port
        self.env=os.environ.copy();self.env.update(GOTOOLCHAIN='local',CGO_ENABLED='0',POC_PROJECT=self.project,
            POC_IMAGE=self.image,POC_WORK=str(self.work),POC_SOCKET_DIR=str(self.work/'workload'),POC_PORT=str(port))
        self.compose=['docker','compose','-f',str(ROOT/'infra/local-ui/compose.yaml')]
        self.runtime=None;self.bridge=None;self.thread=None;self.built=False;self.up=False
        self.metadata=ROOT/'.runtime/local-demo.json'
        self.url=f'http://127.0.0.1:{port}'
        for name in ['trusted-control','untrusted-control','ui-state']:
            (self.work/name).mkdir(mode=0o700)
    def start(self):
        self.runtime=IdentityRuntime(ephemeral_root=self.work,ca_ttl=600)
        # Acquire the substrate lock before any shared metadata or resources.
        if self.metadata.exists():raise RuntimeError('Another demo descriptor exists; inspect its owner before starting')
        names=['money','trusted-caller','untrusted-caller','ui','security-client','control-client']
        for name in names:
            run(['go','build','-trimpath','-o',str(self.work/name),'./cmd/'+name],env=self.env)
            (self.work/name).chmod(0o555)
        shutil.copy(ROOT/'infra/local-ui/Dockerfile',self.work/'Dockerfile')
        run(['docker','build','--network','none','-t',self.image,str(self.work)]);self.built=True
        image_digest=run(['docker','image','inspect',self.image,'--format','{{.Id}}']).stdout.strip()
        self.runtime.start()
        for identity,binary,role in [('spiffe://poc.internal/money','money','local-money'),
            ('spiffe://poc.internal/caller/trusted','trusted-caller','local-trusted'),
            ('spiffe://poc.internal/ui','ui','local-ui')]:
            self.runtime.register(identity,self.work/binary,image_digest,role,ttl=90)
        self.binary_hashes={name:digest(self.work/name) for name in names};self.image_digest=image_digest
        self.up=True;run(self.compose+['up','-d'],env=self.env)
        self.bridge=Bridge(self.port,str(self.work/'ui-state/browser.sock'))
        self.thread=threading.Thread(target=self.bridge.serve_forever,daemon=True);self.thread.start()
        def ready():
            try:
                with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(self.url+'/healthz',timeout=2) as r:return r.status==200
            except (OSError,ValueError):return False
        wait_for(ready,'Local UI listener',seconds=35)
        for service in ['trusted-caller','untrusted-caller']:
            path=self.work/('trusted-control' if service=='trusted-caller' else 'untrusted-control')/'api.sock'
            wait_for(lambda:path.is_socket(),'Private caller socket',seconds=30)
        wait_for(lambda:(self.work/'ui-state/admin.sock').is_socket(),'OS-owner login socket',seconds=10)
        info={'project':self.project,'work':str(self.work),'origin':self.url,'admin_socket':str(self.work/'ui-state/admin.sock'),
            'ephemeral':True,'uid':1000,'pid':os.getpid()}
        self.metadata.write_text(json.dumps(info));self.metadata.chmod(0o600)
        return self
    def close(self):
        if self.bridge:
            self.bridge.shutdown();self.bridge.close_connections();self.bridge.server_close()
            if self.thread:self.thread.join(timeout=3)
        cleanup=run(self.compose+['down','--volumes'],env=self.env,check=False) if self.up else None
        if self.runtime:self.runtime.close()
        image_cleanup=run(['docker','image','rm',self.image],check=False) if self.built else None
        if self.metadata.exists():
            info=json.loads(self.metadata.read_text())
            if info.get('project')==self.project:self.metadata.unlink()
        shutil.rmtree(self.work)
        return {'compose_exit_code':cleanup.returncode if cleanup else 0,'image_exit_code':image_cleanup.returncode if image_cleanup else 0,
            'ephemeral_state_removed':not self.work.exists(),'retained_identity_unchanged':retained_fingerprint()==self.before}


def main():
    os.umask(0o077)
    parser=argparse.ArgumentParser();parser.add_argument('--port',type=int,default=8080);args=parser.parse_args()
    stack=LocalStack(args.port)
    try:
        stack.start()
        print('Local-only demo ready at '+stack.url,flush=True)
        print('In another owner terminal run: python3 scripts/local_login.py',flush=True)
        print('Ctrl-C stops the demo and removes its own temporary trust/state/resources.',flush=True)
        while True:
            time.sleep(1)
            if any(p.poll() is not None for p in stack.runtime.processes):raise RuntimeError('Temporary SPIRE process stopped; inspect protected diagnostics')
    except KeyboardInterrupt:pass
    finally:
        cleanup=stack.close()
        if not cleanup['retained_identity_unchanged'] or cleanup['compose_exit_code'] or cleanup['image_exit_code']:raise RuntimeError('Local demo cleanup requires inspection')


if __name__=='__main__':main()
