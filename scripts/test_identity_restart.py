#!/usr/bin/env python3
"""Real-SPIRE restart test. Retains approved CA/agent state, no test grants."""
import json
import os
from pathlib import Path
import shutil
import tempfile
import uuid

from attestation import ROOT, run, wait_for
from identity import IdentityRuntime


def main():
    os.umask(0o077)
    work = Path(tempfile.mkdtemp(prefix="identity-proof-", dir=ROOT / ".runtime"))
    project = "poc-spire-restart-" + uuid.uuid4().hex[:10]
    image = project + ":fixture"
    identity = "spiffe://poc.internal/test/restart/" + uuid.uuid4().hex
    env = os.environ.copy()
    env.update(GOTOOLCHAIN="local", CGO_ENABLED="0", POC_PROJECT=project,
               POC_IMAGE=image, POC_UID="1000", POC_GID="1000",
               POC_SOCKET_DIR=str(ROOT / ".runtime/workload"))
    compose = ["docker", "compose", "-f", str(ROOT / "infra/attestation/compose.yaml")]
    report = {"status": "failed", "phase": "M2", "checks": {}}
    runtime = None
    entry_id = None
    try:
        print("Building same-UID restart fixture", flush=True)
        for name in ["approved", "probe", "approved-v2"]:
            run(["go", "build", "-trimpath", "-o", str(work / name), "./cmd/attestation-fixture"], env=env)
            (work / name).chmod(0o555)
        shutil.copy(ROOT / "infra/attestation/Dockerfile", work / "Dockerfile")
        run(["docker", "build", "--network", "none", "-t", image, str(work)], env=env)
        image_digest = run(["docker", "image", "inspect", image, "--format", "{{.Id}}"] ).stdout.strip()
        runtime = IdentityRuntime()
        runtime.start()
        entry_id = runtime.register(identity, work / "approved", image_digest, "approved")
        # A private, network-less fixture is enough for issuance/restart proof.
        container = project + "-client"
        run(["docker", "run", "-d", "--name", container, "--user", "1000:1000",
             "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
             "--network", "none", "--label", "poc-spire.role=approved", "-v",
             str(runtime.public) + ":/workload:ro", image, "-listen", ":8443"])
        probe = ["docker", "exec", container, "/approved", "-target", "127.0.0.1:8443", "-expected", identity]
        wait_for(lambda: run(probe, check=False).returncode == 0, "First issuance", seconds=30)
        bundle_before = runtime.bundle_digest
        parent_before = runtime.metadata["parent"]
        runtime.close()
        runtime = None
        print("Restarting retained server and agent state without a join token", flush=True)
        runtime = IdentityRuntime()
        runtime.start()
        wait_for(lambda: run(probe, check=False).returncode == 0, "Post-restart issuance", seconds=30)
        listeners = run(["ss", "-lntp"]).stdout.splitlines()
        own_listeners = [line for line in listeners if "pid=" + str(runtime.processes[0].pid) + "," in line]
        report["checks"] = {"issuance_before_and_after_restart": True,
                            "ca_bundle_unchanged": runtime.bundle_digest == bundle_before,
                            "agent_identity_unchanged": runtime.metadata["parent"] == parent_before,
                            "one_use_join_token_removed": not (runtime.state / "join-token.once").exists(),
                            "no_public_listener": bool(own_listeners) and all(line.split()[3].startswith("127.0.0.1:") for line in own_listeners)}
        if not all(report["checks"].values()):
            raise RuntimeError("Identity continuity check failed")
        report["status"] = "passed"
    except Exception as exc:
        report["failure"] = str(exc)
        print("M2 failed: " + str(exc), flush=True)
    finally:
        run(compose + ["down", "--volumes"], env=env, check=False)
        run(["docker", "rm", "-f", project + "-client"], check=False)
        if runtime is not None:
            if entry_id is not None:
                deletion = run([runtime.server, "entry", "delete", "-socketPath", runtime.server_socket, "-entryID", entry_id], check=False)
                report["checks"]["test_registration_removed"] = deletion.returncode == 0
                if deletion.returncode:
                    report["status"] = "failed"
            runtime.close()
        run(["docker", "image", "rm", image], check=False)
        shutil.rmtree(work)
        state = ROOT / ".runtime/identity"
        permission_failures = []
        for path in [state] + list(state.rglob("*")):
            if path.is_file() or path.is_dir():
                if path.stat().st_mode & 0o077:
                    permission_failures.append(str(path.relative_to(state)))
        report["checks"]["private_state_permissions"] = not permission_failures
        if permission_failures:
            report["status"] = "failed"
        report["private_state_retained"] = True
        (ROOT / "docs/evidence/identity-restart.json").write_text(json.dumps(report, indent=2) + "\n")
        print("M2 restart proof: " + report["status"], flush=True)
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
