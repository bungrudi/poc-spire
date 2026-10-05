#!/usr/bin/env python3
"""Ephemeral real-SPIRE P1 proof; no public listeners or retained credentials."""
import hashlib
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
VERSION = "1.14.0"
ARCHIVE_SHA256 = "dd5238274a6f4c52610b008efe67464f22bf30ac9841ab57d07a7a1087904fac"
TRUSTED = "spiffe://poc.internal/caller/trusted"
PARENT_REDACTED = "spiffe://poc.internal/spire/agent/join_token/[redacted]"


def run(args, *, env=None, check=True, timeout=120):
    # Never echo arguments: the one-use join token is passed to SPIRE in memory.
    try:
        p = subprocess.run(args, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout)
    except subprocess.TimeoutExpired:
        # Command arguments can include a canonical join-token parent ID.
        raise RuntimeError(f"{Path(args[0]).name} {args[1]} timed out") from None
    if check and p.returncode:
        detail = ": " + p.stderr.strip()[-2000:] if args[0] in ("docker", "go") else ""
        raise RuntimeError(f"{Path(args[0]).name} {args[1]} failed (exit {p.returncode})" + detail)
    return p


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def observed_selectors(log):
    observations = []
    for line in log.read_text().splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if event.get("msg", event.get("@message")) != "PID attested to have selectors":
            continue
        raw = event.get("selectors", [])
        if isinstance(raw, str):
            raw = raw.strip("[]").split()
        values = [s["type"] + ":" + s["value"] if isinstance(s, dict) else str(s) for s in raw]
        observations.append({"pid": event.get("pid"), "selectors": sorted(s for s in values if s.startswith((
            "unix:uid:", "unix:sha256:", "docker:label:poc-spire.attestation.role:", "docker:image_config_digest:")))})
    return observations


def verify_observation(selectors, uid, image_digest, binary_digest, deployment):
    expected = {f"unix:uid:{uid}", "unix:sha256:" + binary_digest,
                "docker:image_config_digest:" + image_digest,
                "docker:label:poc-spire.attestation.role:" + deployment}
    if set(selectors) != expected:
        raise RuntimeError("Observed selectors do not establish the intended executable/deployment test")


def spiffe_ids(value):
    if isinstance(value, dict):
        if "trust_domain" in value and "path" in value:
            yield "spiffe://" + value["trust_domain"] + value["path"]
        for item in value.values():
            yield from spiffe_ids(item)
    elif isinstance(value, list):
        for item in value:
            yield from spiffe_ids(item)
    elif isinstance(value, str) and value.startswith("spiffe://"):
        yield value


def wait_for(action, description, seconds=20):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        if action():
            return
        time.sleep(0.2)
    raise RuntimeError(description + " not ready")


def main():
    if os.getuid() == 0:
        raise RuntimeError("Run as the existing unprivileged Docker-enabled user, not root")
    os.umask(0o077)
    runtime_root = ROOT / ".runtime"
    runtime_root.mkdir(exist_ok=True, mode=0o700)
    work = Path(tempfile.mkdtemp(prefix="attestation-", dir=runtime_root))
    project = "poc-spire-proof-" + work.name.split("-")[-1]
    image = project + ":fixture"
    processes = []
    streams = []
    compose = ["docker", "compose", "-f", str(ROOT / "infra/attestation/compose.yaml")]
    env = os.environ.copy()
    env.update(POC_PROJECT=project, POC_IMAGE=image, POC_UID=str(os.getuid()),
               POC_GID=str(os.getgid()), POC_SOCKET_DIR=str(work / "workload"),
               GOTOOLCHAIN="local", CGO_ENABLED="0")
    result = {"phase": "P1", "status": "failed", "tests": [], "versions": {},
              "started_at": datetime.now(timezone.utc).isoformat(),
              "limitations": ["Network target is a test-only TCP sentinel, not an implemented money API",
                              "Host-user agent and caller UID 1000 are a feasibility layout; application UID 10001/containerized agent remain unverified",
                              "No proof of continuous runtime integrity or resistance to host/authorized-process compromise"]}
    evidence = ROOT / "docs/evidence/attestation-latest.json"
    try:
        result["versions"] = {
            "spire": VERSION, "spire_archive_sha256": ARCHIVE_SHA256,
            "go": run(["go", "version"]).stdout.strip(),
            "docker": run(["docker", "version", "--format", "{{.Server.Version}}"] ).stdout.strip(),
            "compose": run(["docker", "compose", "version"]).stdout.strip(),
            "kernel": run(["uname", "-srmo"]).stdout.strip(), "sdk": "v2.6.0"}
        if not result["versions"]["go"].startswith("go version go1.24.2 linux/amd64"):
            raise RuntimeError("This proof is pinned to installed Go 1.24.2 linux/amd64")
        print("Building pinned Go feasibility fixtures", flush=True)
        run(["go", "mod", "download"], env=env)
        build = work / "build"
        build.mkdir()
        for name, role in [("approved", "approved"), ("probe", "unidentified"), ("approved-v2", "approved-rebuild")]:
            run(["go", "build", "-trimpath", "-ldflags", "-X main.buildRole=" + role,
                 "-o", str(build / name), "./cmd/attestation-fixture"], env=env)
            (build / name).chmod(0o555)
        shutil.copy(ROOT / "infra/attestation/Dockerfile", build / "Dockerfile")
        result["binary_sha256"] = {name: digest(build / name) for name in ["approved", "probe", "approved-v2"]}
        if len(set(result["binary_sha256"].values())) != 3:
            raise RuntimeError("Fixture executable digests must differ")
        run(["docker", "build", "--network", "none", "-t", image, str(build)], env=env)
        result["image_config_digest"] = run(["docker", "image", "inspect", image,
                                              "--format", "{{.Id}}"] ).stdout.strip()
        archive = work / "spire.tar.gz"
        url = f"https://github.com/spiffe/spire/releases/download/v{VERSION}/spire-{VERSION}-linux-amd64-musl.tar.gz"
        print("Fetching checksum-pinned SPIRE test binaries", flush=True)
        with urllib.request.urlopen(url, timeout=60) as response, archive.open("wb") as output:
            shutil.copyfileobj(response, output)
        if digest(archive) != ARCHIVE_SHA256:
            raise RuntimeError("SPIRE release archive checksum mismatch")
        run(["tar", "-xzf", str(archive), "-C", str(work)])
        server = str(next(work.glob("spire-*/bin/spire-server")))
        agent = str(next(work.glob("spire-*/bin/spire-agent")))
        for directory in ["server", "agent", "workload"]:
            (work / directory).mkdir()
        (work / "workload").chmod(0o755)
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            port = listener.getsockname()[1]
        server_socket = str(work / "server/api.sock")
        server_config = work / "server.conf"
        server_config.write_text(f'''server {{
  bind_address = "127.0.0.1"
  bind_port = "{port}"
  socket_path = "{server_socket}"
  trust_domain = "poc.internal"
  data_dir = "{work}/server"
  log_level = "DEBUG"
  log_format = "json"
}}
plugins {{
  DataStore "sql" {{ plugin_data {{ database_type = "sqlite3" connection_string = "{work}/server/database.sqlite3" }} }}
  NodeAttestor "join_token" {{ plugin_data {{}} }}
  KeyManager "memory" {{ plugin_data {{}} }}
}}
''')
        def start(args, log):
            stream = (work / log).open("w")
            streams.append(stream)
            process = subprocess.Popen(args, cwd=ROOT, stdout=stream, stderr=stream)
            processes.append(process)
            return process
        start([server, "run", "-config", str(server_config)], "server.log")
        wait_for(lambda: run([server, "healthcheck", "-socketPath", server_socket], check=False).returncode == 0,
                 "SPIRE server")
        run([server, "bundle", "show", "-socketPath", server_socket], check=True)
        bundle = run([server, "bundle", "show", "-socketPath", server_socket, "-format", "pem"]).stdout
        (work / "bundle.pem").write_text(bundle)
        token_output = run([server, "token", "generate", "-socketPath", server_socket,
                            "-ttl", "60", "-output", "json"]).stdout
        token_data = json.loads(token_output)
        token = token_data["value"]
        # The optional CLI spiffeID is an additional alias, not the canonical
        # parent. Never export the canonical ID: it contains the one-use token.
        parent = "spiffe://poc.internal/spire/agent/join_token/" + token
        agent_config = work / "agent.conf"
        agent_config.write_text(f'''agent {{
  server_address = "127.0.0.1"
  server_port = "{port}"
  socket_path = "{work}/workload/api.sock"
  trust_bundle_path = "{work}/bundle.pem"
  trust_domain = "poc.internal"
  data_dir = "{work}/agent"
  log_level = "DEBUG"
  log_format = "json"
}}
plugins {{
  NodeAttestor "join_token" {{ plugin_data {{}} }}
  KeyManager "memory" {{ plugin_data {{}} }}
  WorkloadAttestor "unix" {{ plugin_data {{ discover_workload_path = true workload_size_limit = 134217728 }} }}
  WorkloadAttestor "docker" {{ plugin_data {{}} }}
}}
''')
        result["registration"] = {"parent": PARENT_REDACTED, "identity": TRUSTED, "selectors": [
            f"unix:uid:{os.getuid()}", "unix:sha256:" + result["binary_sha256"]["approved"],
            "docker:label:poc-spire.attestation.role:approved",
            "docker:image_config_digest:" + result["image_config_digest"]]}
        registration = [server, "entry", "create", "-socketPath", server_socket, "-parentID", parent,
                        "-spiffeID", TRUSTED]
        for selector in result["registration"]["selectors"]:
            registration += ["-selector", selector]
        run(registration)
        # Install the narrow entry before initial agent synchronization, rather
        # than mistaking eventual entry propagation for an attestation denial.
        start([agent, "run", "-config", str(agent_config), "-joinToken", token], "agent.log")
        del token, token_output, token_data, registration
        wait_for(lambda: (work / "workload/api.sock").is_socket(), "SPIRE agent socket")
        agents = json.loads(run([server, "agent", "list", "-socketPath", server_socket, "-output", "json"]).stdout)
        result["canonical_parent_matches_server_agent"] = parent in set(spiffe_ids(agents))
        if not result["canonical_parent_matches_server_agent"]:
            raise RuntimeError("Canonical parent does not match the attested server agent")
        del parent, agents
        # A public API socket grants only permission to request attestation; private
        # server/admin/agent state is outside this sole read-only workload mount.
        (work / "workload/api.sock").chmod(0o666)
        print("Starting private same-UID containers; no host ports", flush=True)
        run(compose + ["up", "-d"], env=env)
        result["runtime_users"] = {}
        for service in ["trusted", "unidentified", "wrong-deployment"]:
            container = run(compose + ["ps", "-q", service], env=env).stdout.strip()
            inspect = json.loads(run(["docker", "inspect", container]).stdout)[0]
            pid = inspect["State"]["Pid"]
            result["runtime_users"][service] = {
                "user": inspect["Config"]["User"],
                "uid_map": Path(f"/proc/{pid}/uid_map").read_text().strip(),
                "host_uid": Path(f"/proc/{pid}/status").read_text().split("Uid:\t")[1].splitlines()[0],
                "published_ports": inspect["NetworkSettings"]["Ports"],
                "cap_drop": inspect["HostConfig"]["CapDrop"],
                "read_only": inspect["HostConfig"]["ReadonlyRootfs"]}
        # Prove issuance works before any negative assertion. Socket readiness is
        # not registration/SVID cache readiness; the eventual sync is bounded.
        result["readiness_attempts"] = 0
        def identity_ready():
            result["readiness_attempts"] += 1
            return run(compose + ["exec", "-T", "trusted", "/approved"], env=env,
                       check=False, timeout=15).returncode == 0
        wait_for(identity_ready, "Approved identity issuance", seconds=25)
        cases = [("approved", "trusted", "/approved", False),
                 ("unidentified", "unidentified", "/probe", True),
                 ("wrong-executable-same-deployment", "trusted", "/probe", True),
                 ("approved-wrong-deployment", "wrong-deployment", "/approved", True),
                 ("rebuilt-executable-old-registration", "trusted", "/approved-v2", True)]
        for name, service, executable, negative in cases:
            before = len(observed_selectors(work / "agent.log"))
            args = compose + ["exec", "-T", service, executable]
            if negative:
                args.append("-expect-denied")
            attempt = run(args, env=env, check=False, timeout=15)
            try:
                outcome = json.loads(attempt.stdout)
            except json.JSONDecodeError:
                outcome = {"result": "fixture_infrastructure_failure"}
            observations = observed_selectors(work / "agent.log")[before:]
            result["tests"].append({"case": name, "exit_code": attempt.returncode, "outcome": outcome,
                                     "observations": observations})
            print(name + ": " + outcome["result"], flush=True)
            if attempt.returncode == 0:
                if len(observations) != 1:
                    raise RuntimeError("Each case must have exactly one fresh agent attestation observation")
                deployment = {"trusted": "approved", "unidentified": "unidentified", "wrong-deployment": "wrong-deployment"}[service]
                verify_observation(observations[0]["selectors"], os.getuid(), result["image_config_digest"],
                                   result["binary_sha256"][executable[1:]], deployment)
        # Export only the relevant selector fields, never raw debug logs/env.
        result["observed_selectors"] = observed_selectors(work / "agent.log")
        if any(test["exit_code"] for test in result["tests"]):
            # Safe diagnostic: error messages, not token/request/data fields.
            result["agent_errors"] = []
            for line in (work / "agent.log").read_text().splitlines():
                try:
                    event = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if event.get("level") == "error":
                    result["agent_errors"].append({"message": event.get("msg"), "error": event.get("error")})
            raise RuntimeError("One or more real-SPIRE attestation cases failed")
        if not result["observed_selectors"]:
            raise RuntimeError("No observed selector evidence")
        result["status"] = "passed"
    except Exception as exc:
        result["failure"] = str(exc)
        print("P1 failed: " + str(exc), flush=True)
    finally:
        # Teardown only this uniquely named proof project and our own child PIDs.
        cleanup = run(compose + ["down", "--volumes", "--remove-orphans"], env=env, check=False)
        for process in reversed(processes):
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill(); process.wait()
        for stream in streams:
            stream.close()
        image_cleanup = run(["docker", "image", "rm", image], check=False)
        shutil.rmtree(work)
        result["cleanup"] = {"compose_exit_code": cleanup.returncode,
                             "image_exit_code": image_cleanup.returncode,
                             "private_runtime_removed": not work.exists()}
        if result["status"] == "passed" and (cleanup.returncode or image_cleanup.returncode):
            result["status"] = "failed"
            result["failure"] = "Identity cases passed but complete test-resource cleanup failed"
        result["finished_at"] = datetime.now(timezone.utc).isoformat()
        evidence.parent.mkdir(parents=True, exist_ok=True)
        evidence.write_text(json.dumps(result, indent=2) + "\n")
        evidence.chmod(0o644)
    return 0 if result["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
