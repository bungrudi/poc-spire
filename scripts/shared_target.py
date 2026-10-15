"""Version 1 PoC adapter contract. Keep identical copies in the owning repositories."""
import contextlib
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import uuid

LAB = 'poc-platform/lab'
OWNER = 'poc-platform/owner'
CLUSTER_KINDS = {'ClusterRole', 'ClusterRoleBinding', 'CSIDriver'}


def run(args, *, data=None, timeout=300, cwd=None):
    result = subprocess.run([str(x) for x in args], input=data, text=True,
                            capture_output=True, timeout=timeout, cwd=cwd)
    if result.returncode:
        # Inputs or output can contain credentials, manifests or a certificate bundle.
        raise RuntimeError(f'{Path(str(args[0])).name} {args[1] if len(args)>1 else ""} failed ({result.returncode}); private output withheld')
    return result.stdout


def private_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    if path.is_symlink():
        raise RuntimeError('refuse symlink state path')
    fd, tmp = tempfile.mkstemp(dir=path.parent)
    try:
        with os.fdopen(fd, 'w') as f:
            json.dump(value, f, indent=2)
            f.write('\n')
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp): os.unlink(tmp)


def immutable(reference):
    return isinstance(reference, str) and bool(re.fullmatch(r'[^\s@]+@sha256:[a-f0-9]{64}', reference))


class Target:
    def __init__(self, path, owner):
        self.path = Path(path).resolve()
        self.config = json.loads(self.path.read_text())
        self.owner = owner
        c = self.config
        if c.get('schema_version') != 1 or not re.fullmatch(r'[a-z][a-z0-9-]{2,30}', c.get('lab_id', '')):
            raise ValueError('invalid shared target schema/lab identifier')
        if c.get('account_id')!='581695527661' or c.get('region')!='ap-southeast-2':
            raise ValueError('shared profile is restricted to the approved AWS account and Sydney region')
        self.target = c['target']
        if not self.target.get('context') or not self.target.get('kubeconfig'):
            raise ValueError('explicit context and kubeconfig are required')
        if not re.fullmatch(r'[a-f0-9]{64}', self.target.get('ca_sha256', '')):
            raise ValueError('expected CA fingerprint required')
        if not self.target.get('kube_system_uid') or not self.target.get('server', '').startswith('https://'):
            raise ValueError('expected cluster endpoint and UID required')
        self.base = ['kubectl', '--kubeconfig', str(Path(self.target['kubeconfig']).expanduser().resolve()), '--context', self.target['context']]
        self.runtime = Path(c['runtime_directory']).expanduser().resolve() / owner
        self.runtime.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.receipt_path = self.runtime / 'ownership.json'
        self.receipt = json.loads(self.receipt_path.read_text()) if self.receipt_path.exists() else None

    def kube(self, *args, data=None, timeout=300):
        return run([*self.base, *args], data=data, timeout=timeout)

    def get(self, kind, name, namespace=None):
        args = (['-n', namespace] if namespace else []) + ['get', kind, name, '--ignore-not-found', '-o', 'json']
        raw = self.kube(*args)
        return json.loads(raw) if raw.strip() else None

    def check(self):
        import base64
        actual = json.loads(self.kube('config', 'view', '--minify', '--raw', '-o', 'json'))
        if len(actual.get('clusters', [])) != 1:
            raise RuntimeError('explicit context must resolve to exactly one cluster')
        cluster = actual['clusters'][0]['cluster']
        if cluster.get('insecure-skip-tls-verify'):
            raise RuntimeError('TLS verification bypass refused')
        if 'certificate-authority-data' in cluster:
            ca = base64.b64decode(cluster['certificate-authority-data'], validate=True)
        else:
            ca_path = Path(cluster['certificate-authority'])
            if not ca_path.is_absolute(): ca_path = Path(self.base[2]).parent / ca_path
            ca = ca_path.read_bytes()
        if cluster['server'] != self.target['server'] or hashlib.sha256(ca).hexdigest() != self.target['ca_sha256']:
            raise RuntimeError('cluster endpoint or CA mismatch')
        ns = self.get('namespace', 'kube-system')
        if not ns or ns['metadata']['uid'] != self.target['kube_system_uid']:
            raise RuntimeError('kube-system UID mismatch')
        marker = self.get('configmap', 'identity', 'poc-platform')
        expected = {k: self.config[k] for k in ('lab_id', 'account_id', 'region')}
        if not marker or any(marker.get('data', {}).get(k) != v for k, v in expected.items()):
            raise RuntimeError('platform identity marker mismatch')
        if self.receipt and self.receipt.get('target') != self.target:
            raise RuntimeError('receipt belongs to another cluster incarnation')
        if self.receipt:
            for name,uid in self.receipt['namespaces'].items():
                current=self.get('namespace',name)
                if current and (current['metadata']['uid']!=uid or current['metadata'].get('labels',{}).get(OWNER)!=self.owner):
                    raise RuntimeError('owned namespace was replaced or relabelled')
        return self

    @contextlib.contextmanager
    def lock(self):
        self.check()
        name = self.config['lab_id'] + '-operation'
        token = uuid.uuid4().hex
        value = {'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':name,'namespace':'poc-platform'},
                 'data':{'token':token,'owner':self.owner,'started':str(time.time())}}
        self.kube('create', '-f', '-', data=json.dumps(value))
        try:
            yield
        finally:
            lock = self.get('configmap', name, 'poc-platform')
            if not lock or lock.get('data', {}).get('token') != token:
                raise RuntimeError('operation lock changed; refuse foreign deletion')
            # Delete with API UID/resourceVersion preconditions, rather than a racy name delete.
            delete_exact(self, lock)

    def save(self):
        private_json(self.receipt_path, self.receipt)

    def namespaces(self, names):
        self.check()
        if self.receipt:
            for name in names:
                existing = self.get('namespace', name)
                if not existing or existing['metadata']['uid'] != self.receipt['namespaces'].get(name):
                    raise RuntimeError('owned namespace missing or replaced; explicit recovery required')
            return
        # Check every namespace before creating any of them.
        if any(self.get('namespace', n) for n in names):
            raise RuntimeError('namespace collision; foreign namespaces are never adopted')
        self.receipt = {'owner':self.owner,'lab_id':self.config['lab_id'],'target':self.target,
                        'namespaces':{},'cluster_resources':[],'status':'initializing'}
        self.save()
        for name in names:
            obj = {'apiVersion':'v1','kind':'Namespace','metadata':{'name':name,'labels':{LAB:self.config['lab_id'],OWNER:self.owner}}}
            self.kube('create','-f','-',data=json.dumps(obj))
            self.receipt['namespaces'][name] = self.get('namespace',name)['metadata']['uid']
            self.save()

    def apply(self, objects):
        import yaml
        objects = [o for o in objects if o]
        # Preflight the entire batch before a mutation.
        for obj in objects:
            meta = obj['metadata']; kind = obj['kind']
            if kind == 'Namespace':
                if meta['name'] not in self.receipt['namespaces']: raise RuntimeError('undeclared namespace')
                continue
            if kind in CLUSTER_KINDS:
                existing = self.get(kind, meta['name'])
                recorded = next((r for r in self.receipt['cluster_resources'] if r['kind']==kind and r['name']==meta['name']), None)
                if existing and (not recorded or existing['metadata']['uid'] != recorded['uid'] or existing['metadata'].get('labels',{}).get(OWNER)!=self.owner):
                    raise RuntimeError('cluster resource collision; adoption refused')
            elif meta.get('namespace') not in self.receipt['namespaces']:
                raise RuntimeError('object outside owned namespaces')
            meta.setdefault('labels',{}).update({LAB:self.config['lab_id'],OWNER:self.owner})
        for name,uid in self.receipt['namespaces'].items():
            current=self.get('namespace',name)
            if not current or current['metadata']['uid']!=uid or current['metadata'].get('labels',{}).get(OWNER)!=self.owner:
                raise RuntimeError('owned namespace was lost or changed before apply')
        self.kube('apply','-f','-',data=yaml.safe_dump_all(objects,sort_keys=False))
        for obj in objects:
            if obj['kind'] in CLUSTER_KINDS:
                current=self.get(obj['kind'],obj['metadata']['name'])
                record={'kind':obj['kind'],'name':obj['metadata']['name'],'uid':current['metadata']['uid']}
                if record not in self.receipt['cluster_resources']: self.receipt['cluster_resources'].append(record)
        self.save()

    def remove(self):
        self.check()
        if not self.receipt: raise RuntimeError('no ownership receipt; removal refused')
        # Preflight every object before removing any object.
        objects=[]
        for name,uid in self.receipt['namespaces'].items():
            obj=self.get('namespace',name)
            if obj and (obj['metadata']['uid']!=uid or obj['metadata'].get('labels',{}).get(OWNER)!=self.owner):
                raise RuntimeError('namespace ownership changed; removal refused')
            if obj: objects.append(obj)
        for r in self.receipt['cluster_resources']:
            obj=self.get(r['kind'],r['name'])
            if obj and (obj['metadata']['uid']!=r['uid'] or obj['metadata'].get('labels',{}).get(OWNER)!=self.owner):
                raise RuntimeError('cluster resource ownership changed; removal refused')
            if obj: objects.append(obj)
        namespaces=[obj for obj in objects if obj['kind']=='Namespace']
        for obj in namespaces:delete_exact(self,obj)
        for name in [obj['metadata']['name'] for obj in namespaces]:
            self.kube('wait','--for=delete','namespace/'+name,'--timeout=180s',timeout=190)
        # CSI mounts must be gone before deleting the driver or its remaining RBAC.
        for obj in objects:
            if obj['kind']!='Namespace':delete_exact(self,obj)
        self.receipt['status']='removed'
        self.save()


def delete_exact(target, obj):
    """kubectl proxy on an ephemeral loopback socket, Kubernetes DELETE preconditions."""
    import urllib.request
    meta=obj['metadata']; version=obj['apiVersion']; kind=obj['kind']
    resource={'Namespace':'namespaces','ConfigMap':'configmaps','ClusterRole':'clusterroles',
              'ClusterRoleBinding':'clusterrolebindings','CSIDriver':'csidrivers'}[kind]
    prefix='/api/'+version if '/' not in version else '/apis/'+version
    path=prefix+('/namespaces/'+meta['namespace'] if meta.get('namespace') else '')+'/'+resource+'/'+meta['name']
    # Start the proxy with port 0 and discover its bound address without selecting a racy free port.
    process=subprocess.Popen([*target.base,'proxy','--port=0','--address=127.0.0.1','--accept-hosts=^127\\.0\\.0\\.1$'],
                             stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,text=True)
    try:
        import select
        ready,_,_=select.select([process.stdout],[],[],10)
        if not ready: raise RuntimeError('bounded deletion proxy did not start')
        line=process.stdout.readline()
        match=re.search(r'127\.0\.0\.1:(\d+)',line)
        if not match: raise RuntimeError('unexpected deletion proxy address')
        body=json.dumps({'apiVersion':'v1','kind':'DeleteOptions','preconditions':{'uid':meta['uid'],'resourceVersion':meta['resourceVersion']}}).encode()
        request=urllib.request.Request('http://127.0.0.1:'+match[1]+path,data=body,method='DELETE',headers={'Content-Type':'application/json'})
        with urllib.request.urlopen(request,timeout=30) as response:
            if response.status not in (200,202): raise RuntimeError('preconditioned delete failed')
    finally:
        process.terminate()
        process.wait(timeout=10)
