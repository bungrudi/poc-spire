#!/usr/bin/env python3
"""Approved, project-local SPIRE state. Foreground processes; no system service."""
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import socket
import ssl
import stat
import threading
import subprocess
import time
import urllib.request

from attestation import ROOT, VERSION, ARCHIVE_SHA256, run, digest, wait_for, spiffe_ids


def private_write(path, text, mode=0o600):
    path = Path(path)
    temporary = path.with_suffix(path.suffix + ".new")
    temporary.write_text(text)
    temporary.chmod(mode)
    temporary.replace(path)


class BundlePublisher:
    """Only publish validated, complete bundles from the private server API."""
    def __init__(self, private_path, public_path, fetch, interval=2):
        self.private_path, self.public_path = Path(private_path), Path(public_path)
        self.fetch, self.interval = fetch, interval
        self.stop = threading.Event()
        self.thread = None
        self.digest = None
        self.available = False

    def refresh(self):
        bundle = self.fetch()
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        context.load_verify_locations(cadata=bundle)
        if not context.get_ca_certs(binary_form=True):
            raise RuntimeError("SPIRE returned no CA certificates")
        fingerprint = hashlib.sha256(bundle.encode()).hexdigest()
        if fingerprint != self.digest:
            private_write(self.private_path, bundle)
            # Mode is correct before atomic rename; readers never see partial PEM
            # or an intermediate owner-only public file during rollover.
            private_write(self.public_path, bundle, mode=0o444)
            self.digest = fingerprint
        self.available = True
        return fingerprint

    def start(self):
        def watch():
            while not self.stop.wait(self.interval):
                try:
                    self.refresh()
                except Exception:
                    # An outage keeps the last verified bundle, never an empty or
                    # fabricated trust root. Do not disclose command diagnostics.
                    self.available = False
        self.thread = threading.Thread(target=watch, name="spire-public-bundle", daemon=True)
        self.thread.start()

    def close(self):
        self.stop.set()
        if self.thread is not None:
            self.thread.join(timeout=5)
            if self.thread.is_alive():
                raise RuntimeError("Bundle publication did not stop within its command deadline")


def validate_isolated_root(root):
    root = Path(root)
    if root.parent != Path("/tmp") or not root.name.startswith("poc-kafka-") or len(str(root / "identity/server/api.sock").encode()) > 103:
        raise RuntimeError("Isolated root requires an owned short /tmp/poc-kafka- directory")
    info = root.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
        raise RuntimeError("Isolated root ownership/permissions differ")
    marker = root / "isolated-runtime.json"
    info = marker.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600 or info.st_size > 1024:
        raise RuntimeError("Isolated root requires its runner's private initialization marker")
    value = json.loads(marker.read_text())
    if value.get("version") != 1 or value.get("purpose") != "poc-spire-kafka" or value.get("ephemeral") is not True or value.get("root") != str(root) or not isinstance(value.get("instance"), str) or len(value["instance"]) != 32 or any(c not in "0123456789abcdef" for c in value["instance"]):
        raise RuntimeError("Isolated initialization marker does not match the run")
    for name in ["identity", "workload"]:
        path = root / name
        if path.is_symlink() or (path.exists() and not path.is_dir()):
            raise RuntimeError("Isolated state/socket path collision")
    return value


class IdentityRuntime:
    def __init__(self, *, ephemeral_root=None, ca_ttl=None, isolated=False):
        if os.getuid() != 1000 or os.getgid() != 1000:
            raise RuntimeError("Measured identity runtime requires existing UID/GID 1000")
        os.umask(0o077)
        # Short-TTL rollover tests never overwrite approved retained state.
        retained = ROOT / ".runtime/identity"
        self.ca_ttl = None
        isolated_marker = None
        if isolated and ephemeral_root is None:
            raise RuntimeError("Isolated identity requires a validated fully ephemeral root")
        if ephemeral_root is not None:
            raw_root = Path(ephemeral_root)
            if isolated and (raw_root.is_symlink() or raw_root.absolute() != raw_root.resolve()):
                raise RuntimeError("Isolated root cannot use path aliases or symlinks")
            ephemeral_root = raw_root.resolve()
            if isolated:
                isolated_marker = validate_isolated_root(ephemeral_root)
            elif ephemeral_root.parent != (ROOT / ".runtime").resolve() or not ephemeral_root.name.startswith("backend-") or not ephemeral_root.is_dir():
                raise RuntimeError("Ephemeral identity root must be this suite's project-local temporary directory")
            if ca_ttl is not None and (not isinstance(ca_ttl, int) or not 120 <= ca_ttl <= 600):
                raise RuntimeError("Ephemeral CA TTL must be 120–600 seconds")
            self.ca_ttl = ca_ttl
        elif ca_ttl is not None:
            raise RuntimeError("Retained PoC CA policy cannot be changed by a test option")
        self.state = retained if ephemeral_root is None else ephemeral_root / "identity"
        self.state.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.state.chmod(0o700)
        # Default/retained modes preserve the global substrate guard. Only the
        # explicit validated ephemeral mode owns a separate lock; busy locks
        # are never ignored and protected retained state is never reused.
        lock_root = ephemeral_root if isolated else retained
        lock_root.mkdir(parents=True, exist_ok=True, mode=0o700)
        descriptor = os.open(lock_root / "runtime.lock", os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        self.lock = os.fdopen(descriptor, "a")
        try:
            fcntl.flock(self.lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BaseException:
            self.lock.close()
            raise
        self.public = ROOT / ".runtime/workload" if ephemeral_root is None else ephemeral_root / "workload"
        self.public.mkdir(exist_ok=True, mode=0o755)
        self.public.chmod(0o755)
        self.processes = []
        self.streams = []
        self.created_entries = []
        self.publisher = None
        self.metadata_path = self.state / "runtime.json"
        self.metadata = json.loads(self.metadata_path.read_text()) if self.metadata_path.exists() else None
        if isolated_marker is not None:
            if self.metadata is not None and self.metadata.get("isolated_instance") != isolated_marker["instance"]:
                self.lock.close()
                raise RuntimeError("Existing ephemeral state belongs to another initialized run")
            self.isolated_instance = isolated_marker["instance"]
        else:
            self.isolated_instance = None
        for name in ["server", "agent"]:
            (self.state / name).mkdir(exist_ok=True, mode=0o700)
            (self.state / name).chmod(0o700)
        self.server_socket = str(self.state / "server/api.sock")

    def binaries(self):
        cache = ROOT / ".cache/spire"
        cache.mkdir(parents=True, exist_ok=True, mode=0o700)
        archive = cache / f"spire-{VERSION}.tar.gz"
        if not archive.exists():
            temporary = archive.with_suffix(".download")
            url = f"https://github.com/spiffe/spire/releases/download/v{VERSION}/spire-{VERSION}-linux-amd64-musl.tar.gz"
            with urllib.request.urlopen(url, timeout=60) as response, temporary.open("wb") as output:
                shutil.copyfileobj(response, output)
            if digest(temporary) != ARCHIVE_SHA256:
                temporary.unlink()
                raise RuntimeError("SPIRE release checksum mismatch")
            temporary.replace(archive)
        if digest(archive) != ARCHIVE_SHA256:
            raise RuntimeError("Cached SPIRE release checksum mismatch")
        # Re-extract from verified bytes, rather than trusting a mutable executable cache.
        run(["tar", "-xzf", str(archive), "-C", str(cache)])
        self.server = str(cache / f"spire-{VERSION}/bin/spire-server")
        self.agent = str(cache / f"spire-{VERSION}/bin/spire-agent")

    def spawn(self, args, name):
        # Private diagnostic logs remain in the approved private state. Do not
        # expose raw SPIRE logs: canonical join-token agent IDs contain a token.
        output = (self.state / (name + ".log")).open("w")
        self.streams.append(output)
        process = subprocess.Popen(args, cwd=ROOT, stdout=output, stderr=output)
        self.processes.append(process)
        return process

    def start(self):
        self.binaries()
        if self.metadata is None:
            with socket.socket() as listener:
                listener.bind(("127.0.0.1", 0))
                port = listener.getsockname()[1]
            self.metadata = {"port": port, "trust_domain": "poc.internal", "version": VERSION}
            if self.isolated_instance is not None:
                self.metadata["isolated_instance"] = self.isolated_instance
        if self.metadata["trust_domain"] != "poc.internal" or self.metadata["version"] != VERSION:
            raise RuntimeError("Existing identity state has a different trust domain/version; no automatic reset")
        server_config = self.state / "server.conf"
        test_policy = "" if self.ca_ttl is None else f'  ca_ttl = "{self.ca_ttl}s"\n  default_x509_svid_ttl = "30s"\n  agent_ttl = "30s"\n  disable_jwt_svids = true\n'
        private_write(server_config, f'''server {{
  bind_address = "127.0.0.1"
  bind_port = "{self.metadata['port']}"
  socket_path = "{self.server_socket}"
  trust_domain = "poc.internal"
  data_dir = "{self.state}/server"
  log_level = "INFO"
  log_format = "json"
{test_policy}}}
plugins {{
  DataStore "sql" {{ plugin_data {{ database_type = "sqlite3" connection_string = "{self.state}/server/database.sqlite3" }} }}
  NodeAttestor "join_token" {{ plugin_data {{}} }}
  KeyManager "disk" {{ plugin_data {{ keys_path = "{self.state}/server/keys.json" }} }}
}}
''')
        server_process = self.spawn([self.server, "run", "-config", str(server_config)], "server")
        wait_for(lambda: server_process.poll() is None and run([self.server, "healthcheck", "-socketPath", self.server_socket], check=False).returncode == 0,
                 "Persistent SPIRE server")
        self.publisher = BundlePublisher(self.state / "bundle.pem", self.public / "bundle.pem",
            lambda: run([self.server, "bundle", "show", "-socketPath", self.server_socket, "-format", "pem"], timeout=3).stdout)
        self.bundle_digest = self.publisher.refresh()
        self.publisher.start()
        token_file = self.state / "join-token.once"
        args = [self.agent, "run", "-config", str(self.state / "agent.conf")]
        if "parent" not in self.metadata:
            token_response = json.loads(run([self.server, "token", "generate", "-socketPath", self.server_socket,
                                            "-ttl", "60", "-output", "json"]).stdout)
            token = token_response["value"]
            self.metadata["parent"] = "spiffe://poc.internal/spire/agent/join_token/" + token
            private_write(token_file, token)
            args += ["-joinTokenFile", str(token_file)]
            del token, token_response
        private_write(self.state / "agent.conf", f'''agent {{
  server_address = "127.0.0.1"
  server_port = "{self.metadata['port']}"
  socket_path = "{self.public}/api.sock"
  trust_bundle_path = "{self.state}/bundle.pem"
  trust_domain = "poc.internal"
  data_dir = "{self.state}/agent"
  log_level = "DEBUG"
  log_format = "json"
}}
plugins {{
  NodeAttestor "join_token" {{ plugin_data {{}} }}
  KeyManager "disk" {{ plugin_data {{ directory = "{self.state}/agent" }} }}
  WorkloadAttestor "unix" {{ plugin_data {{ discover_workload_path = true workload_size_limit = 134217728 }} }}
  WorkloadAttestor "docker" {{ plugin_data {{}} }}
}}
''')
        agent_process = self.spawn(args, "agent")
        wait_for(lambda: agent_process.poll() is None and run([self.agent, "healthcheck", "-socketPath", str(self.public / "api.sock")], check=False).returncode == 0,
                 "Persistent SPIRE agent", seconds=30)
        agents = json.loads(run([self.server, "agent", "list", "-socketPath", self.server_socket, "-output", "json"]).stdout)
        if self.metadata["parent"] not in set(spiffe_ids(agents)):
            raise RuntimeError("Persisted parent does not match the attested agent")
        private_write(self.metadata_path, json.dumps(self.metadata))
        if token_file.exists():
            token_file.unlink()
        (self.public / "api.sock").chmod(0o666)
        return self

    def register(self, spiffe_id, binary, image_digest, role, ttl=3600):
        selectors = ["unix:uid:1000", "unix:sha256:" + digest(binary),
                     "docker:label:poc-spire.role:" + role, "docker:image_config_digest:" + image_digest]
        entries = json.loads(run([self.server, "entry", "show", "-socketPath", self.server_socket, "-output", "json"]).stdout)
        matching = [e for e in entries.get("entries", []) if spiffe_id in set(spiffe_ids(e.get("spiffe_id", {})))]
        # No broad duplicate grants and no silent registration broadening.
        for entry in matching:
            if entry.get("x509_svid_ttl") != ttl:
                raise RuntimeError("Existing registration TTL differs; explicit lifetime update required")
            previous = sorted(s["type"] + ":" + s["value"] for s in entry["selectors"])
            parent_ids = set(spiffe_ids(entry.get("parent_id", {})))
            if previous != sorted(selectors) or parent_ids != {self.metadata["parent"]}:
                raise RuntimeError("Existing registration differs; explicit digest update required")
        if len(matching) > 1:
            raise RuntimeError("Duplicate identity registrations are forbidden")
        if matching:
            return matching[0]["id"]
        args = [self.server, "entry", "create", "-socketPath", self.server_socket,
                "-parentID", self.metadata["parent"], "-spiffeID", spiffe_id,
                "-x509SVIDTTL", str(ttl), "-output", "json"]
        for selector in selectors:
            args += ["-selector", selector]
        response = json.loads(run(args).stdout)
        entry_id = response["results"][0]["entry"]["id"]
        self.created_entries.append(entry_id)
        return entry_id

    def close(self):
        errors=[]
        if self.publisher is not None:
            try:
                self.publisher.close();self.publisher=None
            except Exception:
                errors.append('bundle-publisher')
        alive=[]
        for process in reversed(self.processes):
            try:
                if process.poll() is None:
                    process.terminate()
                    try:process.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        process.kill();process.wait(timeout=5)
            except Exception:
                errors.append('owned-process')
            if process.poll() is None:alive.append(process)
        self.processes=list(reversed(alive))
        remaining=[]
        for stream in self.streams:
            try:stream.close()
            except Exception:
                errors.append('private-stream');remaining.append(stream)
        self.streams=remaining
        if not self.processes:
            try:
                token=self.state/'join-token.once'
                if token.exists():token.unlink()
                for path in self.state.rglob('*'):
                    if path.is_dir():path.chmod(0o700)
                    elif path.is_file():path.chmod(0o600)
            except Exception:errors.append('state-permissions')
            try:self.lock.close()
            except Exception:errors.append('runtime-lock')
        if errors:
            raise RuntimeError('Owned native cleanup incomplete: '+','.join(errors))


def main():
    runtime = IdentityRuntime()
    try:
        runtime.start()
        print("SPIRE ready: localhost/Unix sockets; private PoC identity state retained", flush=True)
        while all(p.poll() is None for p in runtime.processes):
            time.sleep(1)
        raise RuntimeError("SPIRE process stopped unexpectedly; see owner-only state logs")
    except KeyboardInterrupt:
        return 0
    finally:
        runtime.close()


if __name__ == "__main__":
    raise SystemExit(main())
