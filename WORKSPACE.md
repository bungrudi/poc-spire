# Workspace brief

## Owner scope update — 11 October 2026

The owner requested deployment at `spire.adianto.dev` and then explicitly
removed OIDC, asking to assume human authentication to focus on SPIRE. The
current HTTPS demo labels its operator as **assumed, not authenticated**;
anyone with browser access may trigger fake transfers. The original OIDC and
verified-human acceptance requirements below are historical and superseded
for this demo. Workload attestation, direct mandatory mTLS, exact-ID Money
authorization, separate caller control mounts, HTTPS and CSRF checks remain.
The isolated UI uses the measured UID-1000/native-agent layout. See
[the current demo and deployment plan](docs/remote-demo.md). The owner specifically approved the HTTPS certificate/key, Nginx route,
renewal hook, persistent service and public access without human login. Those
changes are installed and verified at `https://spire.adianto.dev/`.


## 1. Current status

Implementation was separately authorized on 5 October 2026 in this existing `personal-vps` project. The P1 real-SPIRE executable/deployment attestation suite passes for UID 1000 with an unprivileged native host agent. See [feasibility evidence](docs/attestation-feasibility.md). The money service, actual callers and simplified HTTPS UI are implemented and verified. The owner-approved demo is live at `https://spire.adianto.dev/` with an explicitly assumed human operator. See [implementation checkpoint](docs/implementation-status.md). Persistent credentials/access and host/network security changes still require specific approval.

The technical architecture and security rationale are in [RFC 0001](docs/rfcs/0001-process-identity.md). Current implemented behavior is documented in [OpenSpec contracts](openspec/specs/), with source/test/evidence limits in [the baseline index](docs/spec-baseline.md). Future behavior changes require owner review of a spec delta and short plan before coding; see [the workflow](docs/spec-workflow.md). Historical recommendations are not implementation or acceptance claims.

## 2. Confirmed requirements

| ID | Requirement |
| --- | --- |
| R1 | Go applications, deployed with Docker Compose; no Kubernetes requirement. |
| R2 | One money service owns a fake, in-memory ledger and a transfer endpoint. |
| R3 | The money service itself verifies the calling workload's identity; no mTLS-terminating sidecar. |
| R4 | Two caller processes: one SPIRE-identified and authorized, one unidentified. UI buttons trigger the respective processes, not direct browser transfers. |
| R5 | Identity must distinguish executables running as the same user; UID-only or container-only identity is insufficient. |
| R6 | The web UI must support remote access, not just localhost. |
| R7 | The design must resist forged network requests and unauthorized network peers, including peers that can reach the money service. |
| R8 | Produce a clear workspace brief, TODO list, technical RFC, and agent guidance before coding. |

## 3. Current design decisions and measured scope

| Decision | Current decision and rationale |
| --- | --- |
| Process topology | Separate unprivileged caller containers, same numeric UID and same UID mapping. This preserves useful isolation; a shared container would introduce same-user memory/credential attacks that executable attestation alone does not solve. |
| Proof of process attestation | Combine deployment selectors with an executable digest derived from the Workload API peer PID. Include a controlled wrong-executable test in the trusted deployment. Both callers get socket access; access denial alone does not count as the proof. |
| Scope of 'process identity' | Approved executable in an approved deployment, not a unique PID identity or continuous runtime-integrity guarantee. |
| TLS failure semantics | Missing/untrusted certificate: reject TLS. Valid SPIFFE identity but unauthorized role: HTTP `403`. |
| Browser edge | Existing VPS Nginx serves browser HTTPS and verifies re-encrypted HTTPS to Go UI. Money workload TLS still terminates only in Go Money; browser and SPIFFE PKI remain separate. |
| Human access | Owner explicitly requested an assumed operator and removed OIDC. Anyone may trigger fake transfers; disclosure, secure sessions and CSRF remain. |
| Trigger channel | Private, per-caller Unix-domain sockets mounted into the UI and the corresponding caller only. Callers expose no network trigger endpoint. |
| UI balance reads | A distinct UI SPIFFE identity can read balances but cannot transfer. The UI never borrows the trusted caller's credentials. |
| Demo ledger | Integer minor units, fixed transfer amount, atomic updates, in-memory idempotency. Restart resets the ledger and retry history. |

The measured layout uses UID 1000 with an unprivileged native agent and separate application containers. The originally proposed UID 10001/containerized agent is unverified. Coverage is bounded by the [baseline index](docs/spec-baseline.md); adopting a contract does not create a new test pass or owner acceptance.

## 4. What the demo will and will not establish

The demo should prove both **attestation** (which executable/deployment can obtain a credential) and **authorization** (which verified identity can transfer). Successful TLS alone is not permission to transfer.

It should reject forged headers, arbitrary certificates, an unregistered same-UID executable, and a valid but unauthorized workload identity. Human rejection was explicitly retired for the owner-requested learning demo.

It intentionally does not restrict browser users. It will not prove resistance to host/root/Docker/SPIRE compromise, stolen authorized credentials, arbitrary code execution in the trusted caller, or a compromised UI exercising its legitimate trigger capability. No claim of real-money readiness, tamper-proof runtime execution, or availability against an unbounded denial-of-service attack is intended.

## 5. Document ownership

- [OpenSpec specs](openspec/specs/): source-derived current behavior contracts; owner baseline acceptance pending.
- [RFC 0001](docs/rfcs/0001-process-identity.md): authoritative architecture/security rationale and original T01–T23 matrix; reconcile current sections with approved contract changes.
- [Spec workflow](docs/spec-workflow.md): document authority, pinned tooling and explicit owner review before future behavior changes.
- [Baseline index](docs/spec-baseline.md): sources, named checks, recorded evidence, discrepancies and verification limits.
- [TODO.md](TODO.md): task status and acceptance gates; `[x]` means actually completed, not merely specified.
- [docs/design.md](docs/design.md): short overview; refer to the RFC for details.
- [docs/plan.md](docs/plan.md): milestone sequence, not a second independent checklist.
- [AGENTS.md](AGENTS.md): current authorization, security guardrails and behavior-change review gate.
- [README.md](README.md): entry point; add real run instructions only when a tested implementation exists.

## 6. Original proposed layout — historical

```text
cmd/
  money/                  Direct SPIFFE mTLS server and ledger API
  trusted-caller/         Attested transfer client; private trigger socket
  untrusted-caller/       No-identity transfer probe; private trigger socket
  ui/                     Remote HTTPS, OIDC, roles, sessions, result display
internal/
  identity/               Go SPIFFE source and TLS policy helpers
  ledger/                 Atomic balances and idempotency
  control/                Bounded private trigger protocol
  web/                    Browser authentication, authorization, CSRF, UI
  audit/                  Structured events and evidence labels
web/                      Minimal embedded templates/assets; no SPA required
infra/
  compose/                Stack and test profiles
  spire/                  Server/agent configuration and registration templates
scripts/                  Bootstrap, inspect, integration-test, reset
tests/                    Integration/security fixtures
docs/
  rfcs/                   Design proposals
  evidence/               Versioned test results, never keys/tokens
```

Future paths are suggestions, not existing files. Avoid adding a custom SPIRE plugin unless release-specific verification demonstrates it is necessary and the added scope is explicitly approved.

## 7. Original deployment decisions — historical; current demo choices recorded above

These do not block writing the design. They do block final configuration and deployment:

1. Target Linux host/runtime and CPU architecture; native Linux Docker Engine is the baseline. Docker Desktop/rootless compatibility must be tested rather than assumed.
2. Remote DNS name, certificate provisioning method, and intended exposure: internet or private network/VPN. Recommend private-network access where available, still with HTTPS and OIDC.
3. OIDC issuer, client registration, callback URI, and explicit authorized operator subjects/groups. If none exists, choose a dev identity provider in Compose with non-default credentials, not an unauthenticated fallback.
4. Pinned SPIRE/Go/SDK versions and verified executable attestor behavior. Exact selectors are intentionally not invented in these documents.
5. Formal owner review of remaining recommendations; implementation is now explicitly authorized. Protected PoC SPIRE CA/agent state retention was approved and implemented. M5's different-UID UI/agent privilege layout and browser/identity-provider configuration remain owner decisions.
