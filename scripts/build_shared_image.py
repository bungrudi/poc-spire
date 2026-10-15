#!/usr/bin/env python3
"""Build the normal executable inventory and publish an immutable application image."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sys

from kubernetes_images import components, file_inventory
from shared_target import private_json, run

ROOT=Path(__file__).resolve().parents[1]


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--repository',required=True);parser.add_argument('--output',required=True)
    args=parser.parse_args();revision=run(['git','rev-parse','HEAD'],cwd=ROOT).strip()
    if run(['git','status','--porcelain'],cwd=ROOT).strip():raise RuntimeError('image build requires a clean committed source checkout')
    directory=ROOT/'.runtime/shared-build'/revision;directory.mkdir(parents=True,exist_ok=True,mode=0o700)
    names=components('normal')
    for name in names:
        result=__import__('subprocess').run(['go','build','-trimpath','-buildvcs=false','-o',str(directory/name),'./cmd/'+name],
            cwd=ROOT,env={**os.environ,'CGO_ENABLED':'0','GOOS':'linux','GOARCH':'amd64','GOTOOLCHAIN':'local','GOMAXPROCS':'2'},capture_output=True)
        if result.returncode:raise RuntimeError('Go build failed for '+name)
    (directory/'Dockerfile').write_text('FROM scratch\nCOPY '+' '.join(names)+' /\nUSER 1000:1000\nENTRYPOINT ["/money"]\n')
    (directory/'.dockerignore').write_text('*\n!Dockerfile\n'+''.join('!'+name+'\n' for name in names))
    tag=args.repository+':'+revision
    run(['docker','build','--platform','linux/amd64','--network','none','--label','poc-platform/source='+revision,'-t',tag,directory],timeout=600)
    archive=directory/'image.tar';run(['docker','image','save','--output',archive,tag],timeout=180)
    inventory=file_inventory(archive,names)
    hashes={name:hashlib.sha256((directory/name).read_bytes()).hexdigest() for name in names}
    if any(inventory[n]['sha256']!=hashes[n] for n in names):raise RuntimeError('exported executable inventory differs from source build')
    run(['docker','push',tag],timeout=600)
    info=json.loads(run(['docker','image','inspect',tag]))[0]
    references=[r for r in info.get('RepoDigests',[]) if r.startswith(args.repository+'@sha256:')]
    if len(references)!=1:raise RuntimeError('unique published repository digest unavailable')
    private_json(args.output,{'immutable_reference':references[0],'binary_sha256':hashes,'source_revision':revision,'profile':'normal',
                             'verified_file_inventory':inventory})


if __name__=='__main__':
    try:main()
    except (RuntimeError,ValueError) as exc:sys.exit(str(exc))
