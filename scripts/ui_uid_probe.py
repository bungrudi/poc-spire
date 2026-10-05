#!/usr/bin/env python3
"""Measure the RFC's different-UID UI prerequisite without changing privileges."""
import json
import os
from pathlib import Path
import shutil
import tempfile
import uuid
from attestation import ROOT, run
from identity import IdentityRuntime

def main():
    os.umask(0o077)
    work=Path(tempfile.mkdtemp(prefix="ui-uid-",dir=ROOT/".runtime"))
    name="poc-spire-ui-uid-"+uuid.uuid4().hex[:10]
    image=name+":test"
    runtime=None
    report={"phase":"M5-prerequisite","ui_uid":10002,"agent_uid":1000}
    try:
        env=dict(os.environ,GOTOOLCHAIN="local",CGO_ENABLED="0")
        run(["go","build","-trimpath","-o",str(work/"approved"),"./cmd/attestation-fixture"],env=env)
        (work/"approved").chmod(0o555)
        (work/"Dockerfile").write_text('FROM scratch\nCOPY approved /approved\nUSER 10002:10002\nENTRYPOINT ["/approved"]\n')
        run(["docker","build","--network","none","-t",image,str(work)])
        runtime=IdentityRuntime();runtime.start()
        run(["docker","run","-d","--name",name,"--network","none","--user","10002:10002",
             "--read-only","--cap-drop","ALL","--security-opt","no-new-privileges:true",
             "-v",str(runtime.public)+":/workload:ro",image,"-listen",":8443"])
        offset=(runtime.state/"agent.log").stat().st_size
        probe=run(["docker","exec",name,"/approved","-target","127.0.0.1:8443","-expect-denied"],check=False)
        report["workload_api_outcome"]=json.loads(probe.stdout)
        errors=[]
        with (runtime.state/"agent.log").open() as stream:
            stream.seek(offset)
            for line in stream:
                try:event=json.loads(line)
                except json.JSONDecodeError:continue
                error=event.get("error","") or ""
                if 'workload attestor "unix" failed' in error:
                    errors.append(error)
        report["unix_attestor_errors"]=errors
        report["different_uid_executable_measurement_blocked"]=any("permission denied" in error for error in errors)
        print("Different-UID UI executable measurement blocked:",report["different_uid_executable_measurement_blocked"],flush=True)
    finally:
        run(["docker","rm","-f",name],check=False)
        if runtime is not None:runtime.close()
        run(["docker","image","rm",image],check=False)
        shutil.rmtree(work)
        (ROOT/"docs/evidence/ui-uid-prerequisite.json").write_text(json.dumps(report,indent=2)+"\n")
    return 0

if __name__=="__main__":raise SystemExit(main())
