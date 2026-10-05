# Implementation checkpoint — 5 October 2026

Project: `/home/ubuntu/Workspace/poc-spire` on `personal-vps`. Rudi authorized
implementation and specifically approved retaining PoC CA/database and agent
state under `.runtime/identity/` until an explicit reset/teardown. There is no
public deployment, system service, new SSH access, or host security-policy change.

## Implemented and verified

| Increment | Evidence and limits |
| --- | --- |
| M1 executable/deployment attestation | [P1 evidence](attestation-feasibility.md): real executable hashes, deployment/image conjunction, same UID, wrong-executable/deployment and rebuild denial. Native UID-1000 agent layout only. |
| M2 identity substrate | [Restart result](evidence/identity-restart.json): disk CA and agent keys, protected SQLite/state, unchanged trust and agent identity across restart, issuance before/after restart without a new token, owner-only permissions and measured loopback listener. |
| M3 money service | [Security result](evidence/money-security.json): Go terminates mandatory SPIFFE TLS 1.3, exact route authorization, no cert/forged-header/self-signed rejection, authenticated wrong ID/observer denied, impostor server rejected. Atomic integer ledger, bounded identity-bound deduplication, concurrent retry/conflict/rejection tests. Real SDK rotation, frozen expired credential rejection and attempted connection/session reuse pass. |
| M4 callers/control | [Caller result](evidence/callers-control.json): separate same-UID caller binaries and private control volumes, trusted success and stable retry, unidentified Workload API denial followed by verified-server no-cert TLS rejection, no money HTTP status for that rejection, cross-volume isolation, kernel peer-UID enforcement and fixed bounded command protocol. |

An additional [backend lifecycle checkpoint](backend-acceptance.md) verifies real
CA rollover/public-bundle refresh, outage/expiry, registration removal, an actual
post-commit connection drop and Money restart. See its sanitized
[evidence](evidence/backend-lifecycle.json). All short-TTL CA/fault state was
ephemeral; the suite confirms retained CA/agent state was unchanged.

The [extended lifecycle checkpoint](backend-extended-acceptance.md) adds old-root
taint/revocation propagation, explicit executable-policy updates, strict lifetime
reuse checks and node-expiring outage recovery. Recovery was explicit and
confined to temporary agent state; the same CA, live Money process, ledger and
retry history were preserved. Retained PoC state was unchanged in the fixture.

The [final backend status](backend-final-status.md) reconciles every RFC case and
adds opaque application-proxy TLS-stream loss without TLS termination, mounted
credentials, host ports or privileges. Real commit/unknown-outcome/same-ID retry
passes. Automatic expired-root sweeping has no supported bounded acceleration
in SPIRE 1.14.0 and remains unrun; no long-duration job was initiated.

Go formatting, `go test ./...`, `go test -race ./...`, `go vet ./...`, Python
syntax/evidence-verifier tests, and real-SPIRE Compose suites pass. Early test
parser/classifier issues were repaired; final evidence is passing. Ledger
insufficient-funds, overflow, capacity and conservation checks are unit tested.
The M3 lost-result retry test discards a completed response. The additional backend
suite closes a real TLS connection after the Money handler commits and before
returning its response; the caller reports unknown commit, then the same ID
resolves the committed result. The final proxy suite independently drops encrypted response records after real commit. TCP/IP retransmission/arbitrary packet-loss injection remains unrun.
Money rechecks peer validity before route authorization and immediately before
ledger mutation, including expiry while reading a body. Boundary regression
cases pass. A certificate expired before connection is rejected at TLS with no
HTTP status; expiry after a valid handshake is denied at the handler without
mutation. The rotation aggregate reports observed server verification explicitly.

The local control harness runs as UID 10002, has only the two read-only control
mounts, and has no network or Workload API mount. It is privileged test tooling,
not a remotely accessible UI or a substitute for OIDC. Actual callers run as
1000:1000. Control-volume initialization temporarily uses a root container with
only CHOWN/FOWNER/FSETID capabilities and only the two fresh control-volume
mounts; applications drop all capabilities and have read-only root filesystems.

## State, lifecycle and commands

Retained private state is `.runtime/identity/server/` and
`.runtime/identity/agent/`, with metadata/config/private diagnostic logs in the
same owner-only identity directory. Directories are 0700, private files 0600,
owned by the existing host user. Runtime/cache paths are ignored. Workloads
mount only `.runtime/workload/`, containing the public socket/bundle; they cannot
mount CA/database/admin state or the Docker socket. Canonical join-token agent
IDs appear only in protected runtime state/logs, never evidence. The active
one-use token file is deleted after bootstrap.

`python3 scripts/identity.py` starts the substrate in the foreground. Ctrl-C
stops only its child processes and retains state. No autostart is installed.
Normal restart retains CA/agent identity; if an offline agent credential expires,
re-enrollment is a separate explicit operation. There is no automatic CA reset.
A bounded background publisher polls the private server API every two seconds,
validates complete CA PEM, and atomically publishes the public bundle as 0444.
Fetch/parse failure preserves the last verified bundle; it adds no fallback trust.
Real short-TTL rollover and outage/expiry are tested in isolated state. Production
24-hour rollover duration and automatic expired-root sweeping remain unrun.
Native taint/admin root removal and a node-expiring outage/recovery now pass.

```sh
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go vet ./...
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/test_identity_restart.py
python3 scripts/security_suite.py
python3 scripts/caller_suite.py
python3 scripts/backend_suite.py
python3 scripts/backend_extended_suite.py
python3 scripts/backend_proxy_suite.py
```

Run one identity/test command at a time; an advisory lock prevents simultaneous
state writers. Integration commands start temporary private containers, then
remove their own containers, volumes, images and newly created test grants.
They retain the approved identity state. Existing different registrations cause
a stop rather than a silent policy/digest rewrite. No browser/OIDC secrets or
workload private keys have been retained. Mac metadata files are left untouched.

## M5 blockers and remaining work

1. **UI process UID/agent layout:** the [different-UID probe](evidence/ui-uid-prerequisite.json)
   confirms the current UID-1000 agent cannot hash a UID-10002 executable:
   `readlink /proc/<pid>/exe: permission denied`. Socket/network reachability is
   not the cause. Keep the RFC's different-UID UI and review agent privileges,
   or approve a separately isolated UID-1000 UI. The latter changes the RFC
   hardening recommendation while retaining executable/deployment attestation
   and private volume isolation; control peer policy/tests must then be adapted.
   No privileges were raised and the executable requirement was not weakened.
2. **Remote configuration:** choose hostname/exposure, browser certificate
   provisioning, OIDC issuer/client ID and explicit operator/viewer mapping.
   Only non-secret configuration is needed in conversation. Private TLS/OIDC
   material belongs in protected runtime files. A private test issuer is an
   alternative if explicitly selected; there is no anonymous fallback.

The remote UI, HTTPS/OIDC/session/CSRF controls, full T01–T23 acceptance, CA-bundle
automatic expired-root sweeping/default-duration rollover, end-to-end UI restart/re-login
retry proof, remote exposure and owner acceptance remain unimplemented/unrun.
T01/T02 are verified only through their internal caller path. T03 uses actual
same-UID callers in M4, not an inference from a UI button.
