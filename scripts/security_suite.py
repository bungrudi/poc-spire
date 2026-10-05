#!/usr/bin/env python3
"""M3 direct mTLS/API tests against real SPIRE; no remote edge exposure."""
from concurrent.futures import ThreadPoolExecutor
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
    work = Path(tempfile.mkdtemp(prefix="security-", dir=ROOT / ".runtime"))
    project = "poc-spire-security-" + uuid.uuid4().hex[:10]
    image = project + ":test"
    env = os.environ.copy()
    env.update(GOTOOLCHAIN="local", CGO_ENABLED="0", POC_PROJECT=project,
               POC_IMAGE=image, POC_SOCKET_DIR=str(ROOT / ".runtime/workload"))
    compose = ["docker", "compose", "-f", str(ROOT / "infra/demo/security-compose.yaml")]
    report = {"phase": "M3", "status": "failed", "tests": [], "persistent_identity_state_retained": True}
    runtime = None
    try:
        print("Building direct-mTLS money service and security instrumentation", flush=True)
        for name in ["money", "security-client"]:
            run(["go", "build", "-trimpath", "-o", str(work / name), "./cmd/" + name], env=env)
            (work / name).chmod(0o555)
        shutil.copy(ROOT / "infra/demo/security.Dockerfile", work / "Dockerfile")
        run(["docker", "build", "--network", "none", "-t", image, str(work)])
        image_digest = run(["docker", "image", "inspect", image, "--format", "{{.Id}}"] ).stdout.strip()
        runtime = IdentityRuntime()
        runtime.start()
        report["binary_sha256"] = {name: digest(work / name) for name in ["money", "security-client"]}
        report["image_config_digest"] = image_digest
        specs = [("spiffe://poc.internal/money", "money", "money", 120),
                 ("spiffe://poc.internal/caller/trusted", "security-client", "trusted-probe", 30),
                 ("spiffe://poc.internal/ui", "security-client", "observer-probe", 120),
                 ("spiffe://poc.internal/test/wrong-id", "security-client", "wrong-id", 120),
                 ("spiffe://poc.internal/test/impostor", "security-client", "impostor", 120)]
        for identity, binary, role, ttl in specs:
            runtime.register(identity, work / binary, image_digest, role, ttl=ttl)
        run(compose + ["up", "-d"], env=env)

        def probe(service, flags, check=True, timeout=30):
            p = run(compose + ["exec", "-T", service, "/security-client"] + flags, env=env, check=False, timeout=timeout)
            try:
                value = json.loads(p.stdout)
            except json.JSONDecodeError:
                raise RuntimeError("Security instrumentation failed without a structured outcome")
            if check and p.returncode:
                raise RuntimeError(service + ": " + value.get("outcome", "unknown failure"))
            return p.returncode, value

        def ready():
            code, _ = probe("observer-probe", ["-action", "balances"], check=False)
            return code == 0
        wait_for(ready, "Money and observer identity readiness", seconds=35)
        initial = probe("observer-probe", ["-action", "balances"])[1]["body"]
        cases = [("no-cert-forged-identity-headers", "unidentified", ["-mode", "no-cert"]),
                 ("self-signed-allowed-uri", "unidentified", ["-mode", "self-signed"]),
                 ("valid-wrong-id", "wrong-id", ["-status", "403"]),
                 ("observer-cannot-transfer", "observer-probe", ["-status", "403"]),
                 ("wrong-server-identity", "trusted-probe", ["-target", "https://impostor:8443", "-status", "0"])]
        for case, service, flags in cases:
            _, value = probe(service, flags)
            after = probe("observer-probe", ["-action", "balances"])[1]["body"]
            if after != initial:
                raise RuntimeError(case + " changed the ledger")
            report["tests"].append({"case": case, "result": value, "ledger_unchanged": True})
            print(case + ": passed", flush=True)
        with ThreadPoolExecutor(max_workers=8) as pool:
            results = list(pool.map(lambda _: probe("trusted-probe", ["-id", "concurrent-one"])[1], range(16)))
        after = probe("observer-probe", ["-action", "balances"])[1]["body"]
        if after["version"] != 1 or after["balances"] != {"demo-a":99900,"demo-b":100100}:
            raise RuntimeError("Concurrent duplicate transfer mutated more than once")
        if len({r["body"]["transfer_id"] for r in results}) != 1:
            raise RuntimeError("Concurrent retry results were not stable")
        report["tests"].append({"case":"concurrent-duplicates", "requests":16, "ledger_version":1})
        _, replay = probe("trusted-probe", ["-id", "concurrent-one"])
        if not replay["body"]["replay"]:
            raise RuntimeError("Lost-response retry was not idempotent")
        probe("trusted-probe", ["-id","concurrent-one","-amount","200","-status","409"])
        probe("trusted-probe", ["-id","invalid-fixed","-amount","-1","-status","400"])
        if probe("observer-probe", ["-action","balances"])[1]["body"] != after:
            raise RuntimeError("Conflict/invalid/retry changed balances")
        report["tests"].append({"case":"lost-response-conflict-invalid-input", "ledger_unchanged":True})
        print("Concurrent duplicates, stable retries and conflicts: passed", flush=True)
        _, rotation = probe("trusted-probe", ["-mode","rotation"], timeout=90)
        report["tests"].append({"case":"rotation-expiry-and-connection-reuse", "result":rotation})
        print("Real rotation, expired client and attempted session/connection reuse: passed", flush=True)
        final = probe("observer-probe", ["-action","balances"])[1]["body"]
        if final["version"] != 3:
            raise RuntimeError("Expired credential or reuse unexpectedly mutated the ledger")
        report["final_ledger"] = final
        container = run(compose + ["ps", "-q", "money"], env=env).stdout.strip()
        audit = []
        for line in run(["docker", "logs", container]).stdout.splitlines():
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if event.get("event") == "money_request":
                audit.append(event)
        if not any(e["verified_peer"] == "spiffe://poc.internal/test/wrong-id" and e["status"]==403 for e in audit):
            raise RuntimeError("Missing verified wrong-identity money audit evidence")
        report["money_audit"] = audit
        report["status"] = "passed"
    except Exception as exc:
        report["failure"] = str(exc)
        print("M3 failed: " + str(exc), flush=True)
    finally:
        cleanup = run(compose + ["down", "--volumes"], env=env, check=False)
        removed = True
        if runtime is not None:
            for entry in runtime.created_entries:
                removed &= run([runtime.server,"entry","delete","-socketPath",runtime.server_socket,"-entryID",entry],check=False).returncode==0
            runtime.close()
        image_cleanup = run(["docker","image","rm",image],check=False)
        shutil.rmtree(work)
        report["cleanup"] = {"compose_exit_code":cleanup.returncode,"image_exit_code":image_cleanup.returncode,
                             "new_test_registrations_removed":removed}
        if cleanup.returncode or image_cleanup.returncode or not removed:
            report["status"] = "failed"
        (ROOT / "docs/evidence/money-security.json").write_text(json.dumps(report,indent=2)+"\n")
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
