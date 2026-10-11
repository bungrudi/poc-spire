#!/usr/bin/env python3
"""OS-owner login helper. Code is an ephemeral secret: never save in evidence."""
import http.client
import json
import os
from pathlib import Path
import socket
import stat

from attestation import ROOT


def login_code(path):
    path=Path(path)
    info=path.lstat()
    if not stat.S_ISSOCK(info.st_mode) or info.st_uid!=os.getuid() or stat.S_IMODE(info.st_mode)!=0o600:
        raise RuntimeError('Owner-only UI login socket required')
    connection=http.client.HTTPConnection('unix',timeout=3)
    connection.sock=socket.socket(socket.AF_UNIX);connection.sock.settimeout(3);connection.sock.connect(str(path))
    try:
        connection.request('POST','/login-code',body=b'')
        response=connection.getresponse();data=response.read(1025)
        if response.status!=200 or len(data)>1024:raise RuntimeError('OS-owner login denied')
        return json.loads(data)['code']
    finally:connection.close()


def main():
    info=json.loads((ROOT/'.runtime/local-demo.json').read_text())
    print('Enter this one-use code at '+info['origin']+' (expires in one minute):')
    print(login_code(info['admin_socket']))


if __name__=='__main__':main()
