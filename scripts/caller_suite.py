#!/usr/bin/env python3
"""M4 caller/UDS proof. Test harness is local infrastructure, not a remote UI."""
import json
import os
from pathlib import Path
import shutil
import tempfile
import uuid

from attestation import ROOT, run, wait_for, digest
from identity import IdentityRuntime


def main():
    os.umask(0o077)
    work = Path(tempfile.mkdtemp(prefix="caller-", dir=ROOT / ".runtime"))
    project = "poc-spire-callers-" + uuid.uuid4().hex[:10]
    image = project + ":test"
    env = os.environ.copy()
    env.update(GOTOOLCHAIN="local", CGO_ENABLED="0", POC_PROJECT=project,
               POC_IMAGE=image, POC_SOCKET_DIR=str(ROOT / ".runtime/workload"))
    compose = ["docker","compose","-f",str(ROOT / "infra/demo/caller-compose.yaml")]
    runtime = None
    report = {"phase":"M4","status":"failed","tests":[],"persistent_identity_state_retained":True}
    try:
        print("Building separate callers and private-control harness",flush=True)
        binaries = ["money","trusted-caller","untrusted-caller","security-client","control-init","control-client"]
        for name in binaries:
            run(["go","build","-trimpath","-o",str(work/name),"./cmd/"+name],env=env)
            (work/name).chmod(0o555)
        if digest(work/"trusted-caller")==digest(work/"untrusted-caller"):
            raise RuntimeError("Caller executables must differ")
        shutil.copy(ROOT/"infra/demo/caller.Dockerfile",work/"Dockerfile")
        run(["docker","build","--network","none","-t",image,str(work)])
        image_digest = run(["docker","image","inspect",image,"--format","{{.Id}}"] ).stdout.strip()
        runtime=IdentityRuntime();runtime.start()
        for spiffe_id,binary,role in [("spiffe://poc.internal/money","money","money"),
            ("spiffe://poc.internal/caller/trusted","trusted-caller","trusted-caller"),
            ("spiffe://poc.internal/ui","security-client","observer-probe")]:
            runtime.register(spiffe_id,work/binary,image_digest,role)
        run(compose+["up","-d"],env=env)

        def control(socket,id="logical-transfer",service="harness",raw=None,user=None):
            args=compose+["exec","-T"]
            if user:args += ["--user",user]
            args += [service,"/control-client","-socket",socket,"-id",id]
            if raw:args += ["-raw",raw]
            p=run(args,env=env,check=False,timeout=20)
            try:value=json.loads(p.stdout)
            except json.JSONDecodeError:raise RuntimeError("Unstructured control fixture failure")
            return p.returncode,value

        def balances():
            p=run(compose+["exec","-T","observer","/security-client","-action","balances"],env=env,check=False)
            if p.returncode:return None
            return json.loads(p.stdout)["body"]

        wait_for(lambda:balances() is not None,"Money/observer ready",seconds=35)
        wait_for(lambda:control("/trusted-control/api.sock",raw="{}")[1].get("control_status")==400,
                 "Trusted caller ready",seconds=30)
        initial=balances()
        code,trusted=control("/trusted-control/api.sock")
        if code or trusted["outcome"]!="transfer_succeeded" or trusted["money_http_status"]!=200 or not trusted["server_verified"]:
            raise RuntimeError("Trusted caller transfer failed")
        if trusted["money_result"]["verified_caller_id"]!="spiffe://poc.internal/caller/trusted":
            raise RuntimeError("Money did not record the verified trusted caller")
        after=balances()
        if after["version"]!=initial["version"]+1:
            raise RuntimeError("Trusted caller did not transfer exactly once")
        report["tests"].append({"case":"trusted-caller","result":trusted})
        _,replay=control("/trusted-control/api.sock")
        if not replay["money_result"]["replay"] or balances()!=after:
            raise RuntimeError("Control retry did not preserve transfer ID")
        report["tests"].append({"case":"stable-caller-retry","replay":True})
        code,unidentified=control("/untrusted-control/api.sock",id="unidentified-attempt")
        if code or unidentified["outcome"]!="signature_rejected" or unidentified["identity_eligibility"]!="no_matching_identity" or unidentified.get("money_http_status")!=401 or not unidentified["server_verified"]:
            raise RuntimeError("Unidentified caller did not demonstrate verified no-SVID signature rejection")
        if balances()!=after:raise RuntimeError("Unidentified caller changed balances")
        report["tests"].append({"case":"unidentified-caller","result":unidentified,"ledger_unchanged":True})
        code,blocked=control("/trusted-control/api.sock",service="untrusted-caller")
        if code==0 or blocked.get("outcome")!="control_unreachable":
            raise RuntimeError("Unidentified caller reached the trusted control volume")
        report["tests"].append({"case":"cross-caller-control-isolation","result":blocked})
        code,wrong_peer=control("/trusted-control/api.sock",user="1000:1000")
        if code==0 or wrong_peer.get("control_status")!=403:
            raise RuntimeError("Wrong Unix peer UID was not rejected")
        report["tests"].append({"case":"kernel-peer-uid-enforcement","control_status":403})
        code,bad=control("/trusted-control/api.sock",raw='{"request_id":"arbitrary","url":"http://attacker","amount_minor":999}')
        if code==0 or bad.get("control_status")!=400 or balances()!=after:
            raise RuntimeError("Private protocol accepted arbitrary network/business instructions")
        report["tests"].append({"case":"bounded-fixed-control-protocol","control_status":400})
        report["runtime"]={}
        for service in ["trusted-caller","untrusted-caller","harness"]:
            container=run(compose+["ps","-q",service],env=env).stdout.strip()
            info=json.loads(run(["docker","inspect",container]).stdout)[0]
            report["runtime"][service]={"user":info["Config"]["User"],"published_ports":info["NetworkSettings"]["Ports"],
                "mount_destinations":[m["Destination"] for m in info["Mounts"]],"cap_drop":info["HostConfig"]["CapDrop"],
                "uid_map":Path("/proc/"+str(info["State"]["Pid"])+"/uid_map").read_text().strip()}
        if report["runtime"]["trusted-caller"]["user"]!=report["runtime"]["untrusted-caller"]["user"]:
            raise RuntimeError("Caller UIDs differ")
        if any(x["published_ports"] for x in report["runtime"].values()):
            raise RuntimeError("Unexpected published caller/control port")
        report["final_ledger"]=after
        report["status"]="passed"
        print("Trusted transfer, unidentified no-SVID/TLS denial, stable retry and private-control isolation: passed",flush=True)
    except Exception as exc:
        report["failure"]=str(exc);print("M4 failed: "+str(exc),flush=True)
    finally:
        cleanup=run(compose+["down","--volumes"],env=env,check=False)
        removed=True
        if runtime is not None:
            for entry in runtime.created_entries:
                removed &= run([runtime.server,"entry","delete","-socketPath",runtime.server_socket,"-entryID",entry],check=False).returncode==0
            runtime.close()
        image_cleanup=run(["docker","image","rm",image],check=False)
        shutil.rmtree(work)
        report["cleanup"]={"compose_exit_code":cleanup.returncode,"image_exit_code":image_cleanup.returncode,"new_test_registrations_removed":removed}
        if cleanup.returncode or image_cleanup.returncode or not removed:report["status"]="failed"
        (ROOT/"docs/evidence/callers-control.json").write_text(json.dumps(report,indent=2)+"\n")
    return 0 if report["status"]=="passed" else 1


if __name__=="__main__":raise SystemExit(main())
