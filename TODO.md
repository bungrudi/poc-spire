# TODO — SPIRE process-identity PoC

## Current source change — 18 October 2026

- [x] Remove direct application mTLS and use signed requests/responses in every profile.
- [x] Keep Money's independent chain/signature/ID checks and guard freshness under the ledger lock.
- [x] Wire SPIRE SDS/CSI into proxies, narrow proxy grants and matching mesh policies.
- [x] Update deterministic Go/Python checks and current OpenSpec contracts.
- [ ] Observe actual mesh SVID issuer and rotation in a fresh disposable Kubernetes run.
- [ ] Deploy the new source; no live cutover is included in this change.

The [change verification](openspec/changes/remove-direct-app-mtls/verification.md)
separates new checks from the earlier observations below.

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


**Current phase: five-scenario Kubernetes/Istio/SPIRE/Kafka HTTPS demo live and independently verified at spire.adianto.dev. Owner approved public assumed-operator access, browser TLS retention, Nginx, renewal hook and startup service. Original verified-human/full acceptance gates below are historical, not claimed.**

Checked boxes mean work actually completed. Unchecked boxes are planned work, not tests that passed. Technical decisions live in [RFC 0001](docs/rfcs/0001-process-identity.md); workspace requirements live in [WORKSPACE.md](WORKSPACE.md).

## SVID-signed Kubernetes workload HTTP — 14 October 2026

- [x] Owner approved applying `replace-app-mtls-with-svid-signature` at `9f048254a4a89d1be80c6dcf7df88c822ad2cb01`.
- [x] Kubernetes workload HTTP signs REST payloads with the current X.509-SVID and does not use application mTLS. Compose direct mTLS stays.
- [x] Unit tests cover tampered payloads, non-verifying signatures, obsolete signatures, forged chains, route replay, UI denial and a still-valid previous SVID.
- [ ] Re-run the live Kubernetes/SPIRE mesh checks for the signed-HTTP profile. That observation is unrun.

## Current public Kubernetes deployment — 13 October 2026

The owner directly authorized merge/push, named public cutover and cleanup.
[Current runbook/evidence](docs/kubernetes-public.md) records a main-local normal
Kubernetes/Istio/SPIRE/Kafka run, public five outcomes/ledger 0 → 2, verified upstream
TLS, browser defenses, renewal projection and route restart. Eight old public/
preview containers and their owned supervisors/issuers/runtime are removed; the
old public service is masked. Shared apps/access, retained state and local WIP
are preserved. Final independent public-route/source/cleanup review PASS; strict validation 13/13.

## Historical two-scenario Compose demo — 11 October 2026

The original human-login checklist below is historical for this profile. No
verified-human or original full Gate F/G acceptance is claimed.

- [x] Remove OIDC implementation and unused dependencies; declare an assumed operator.
- [x] Serve the demo over Go HTTPS with secure in-memory sessions, CSRF, exact origin/Host checks and public disclosure.
- [x] Use a separate UID-1000 UI container and measured native agent without raising agent/host privileges.
- [x] Exercise real SPIRE browser-to-caller trusted transfer, unidentified no-SVID/TLS rejection and unchanged rejected balances.
- [x] Verify stable request IDs/no duplicate transfer after UI restart and renewed demo session.
- [x] Verify cross-caller control isolation, private backend ports and application mount/capability boundaries.
- [x] Run Go tests/race/vet and Python deterministic checks; retain sanitized `docs/evidence/ui-demo.json`.
- [x] Prepare exact public/persistent host changes in `docs/remote-demo.md`; no deployment changes installed.
- [x] Obtain specific public/persistent deployment approval required by automatic review (owner approved on 11 October 2026).
- [x] Install `spire.adianto.dev` browser certificate, Nginx route, certificate-renewal hook and supervised service.
- [x] Verify deployed HTTPS browser/API, caller outcomes, ports/mounts, protected key permissions, startup/renewal timers and renewal-hook session/ledger continuity; record public evidence.
- [ ] Observe actual scheduled ACME renewal in the future; current issuance and manual hook test pass.

## OpenSpec adoption — owner request, 11 October 2026

- [x] Pin project-local OpenSpec 1.14.1, lock dependencies and document a suitable existing Node runtime without global/user-wide installation.
- [x] Generate the seven official Codex skills, including verification, using the stock spec-driven schema and isolated configuration.
- [x] Document six current capabilities with 26 requirements/60 scenarios and link source, named checks, recorded evidence and R1–R8/T01–T23 coverage/limits.
- [x] Require actual owner approval of each future behavioral spec delta and short plan before code/apply; an agent-written review cannot grant approval.
- [x] Reconcile active assumed-operator/UID-1000/deployment prose and label old OIDC/full-acceptance checkpoints as historical.
- [x] Verify locked installation, reproducible skills, strict spec validation, scoped references/exclusions and deterministic checks; record cached Go results and unrun integration checks in [adoption evidence](docs/evidence/openspec-adoption.json).
- [x] Complete independent review and resolve the three factual wording findings plus health-check clarification; final strict validation passes.
- [ ] Obtain owner acceptance of baseline correctness; source inspection and structural validation do not imply acceptance.
- [x] Parent records the directly authorized standing commit/push permission after the single-writer handoff, separately from future behavior-change review.

## Approved isolated Kafka iteration — 12 October 2026

[Actual owner approval](openspec/changes/add-spiffe-kafka-message-signing/review.md)
identifies the reviewed `735b80f` scope; [tasks](openspec/changes/add-spiffe-kafka-message-signing/tasks.md)
track its 18 implementation checks. [Coverage](docs/kafka-signing-acceptance.md)
distinguishes real integration, injected clock/bundle/capacity faults and unrun
observations. The synchronous public deployment remains unchanged.

- [x] Record actual owner approval before implementation and assess/pin dependencies.
- [x] Implement bounded JWS/SVID validation, shared-ledger guards/context/receipts and ordered captured-generation consumption.
- [x] Add initialized private caller recovery and the fixed combined UI with authenticated all-variant outcomes.
- [x] Implement isolated Compose/native SPIRE, separate broker/browser PKI, exact ACLs, readiness and owned cleanup.
- [x] Complete the frozen-source JS-01–JS-23 acceptance run and sanitized final evidence.
- [x] Complete independent review of the final source/evidence revision; parent [PASS and separate rerun evidence](docs/evidence/kafka-independent-review.json), 12 October 2026.
- [ ] Obtain owner acceptance of the implemented learning profile based on evidence.

## Approved Kubernetes/Istio baseline — 13 October 2026

The isolated complete baseline and retained core security checks pass on clean
`528bc5f`; [runtime/evidence](docs/kubernetes-baseline.md) records five scenarios,
actual double TLS, attestation/authorization/JWS and restored boundaries/resources.
Parent independent source/runtime review passed. Final docs consistency review passed;
owner acceptance remains pending in [tasks](openspec/changes/add-kubernetes-double-mtls-baseline/tasks.md).
Extended reliability is DEFERRED; the public/Compose profiles are unchanged.

## Original milestone checklist — historical

The P0–P6 entries below preserve the 5 October implementation sequence and its
original verified-human acceptance definition. Old unchecked UI/deployment/OIDC
items and gate text are not the current public-demo backlog. The active profile
and remaining scheduled-renewal observation are above; current feature/test
contracts and bounded coverage are in [the baseline index](docs/spec-baseline.md).
Future behavior changes follow [the owner-reviewed workflow](docs/spec-workflow.md).
Do not mark historical requirements passed merely because the simplified demo
was accepted for a different scope.

## P0 — Planning and approval

- [x] Record confirmed requirements and recommended same-UID container topology in `WORKSPACE.md`.
- [x] Write a detailed technical RFC covering attestation, direct mTLS, remote human access, control channels, ledger, and threat model.
- [x] Define TLS rejection versus authenticated HTTP `403` behavior.
- [x] Replace the obsolete localhost-only assumption with mandatory remote HTTPS and human authorization.
- [x] Create a documentation index, milestone plan, and agent guardrails.
- [ ] Owner reviews the recommended topology: separate same-UID containers plus a controlled wrong-executable test in the trusted deployment.
- [ ] Owner reviews RFC scope and limitations, including trust in the UI/authorized caller and no runtime-integrity claim.
- [x] Select target host/runtime/architecture: existing `personal-vps`, native Linux Docker, linux/amd64. P1 uses UID 1000/native agent; production UID/containerized-agent layout remains unverified.
- [ ] Select remote DNS name, network exposure, and HTTPS certificate provisioning.
- [ ] Select OIDC provider/client and explicit operator/viewer mapping; no automatic role for every authenticated user.
- [x] Obtain a separate, explicit instruction to implement: Rudi's 5 October 2026 implementation-loop request and confirmation to use this existing project.

**Gate A:** Coding is authorized. Formal owner-review checkboxes remain unclaimed. No remote deployment is authorized; deployment configuration remains unresolved.

## P1 — Verify executable attestation before app development

Dependencies: Gate A.

- [x] Pin Go, SPIRE, Go SPIFFE SDK, Docker/Compose requirements, and image versions/digests.
- [x] Read pinned-release Unix and Docker workload-attestor docs/source; record supported digest selectors/options and selector-conjunction semantics.
- [x] Record a least-privilege agent deployment: peer PID visibility, procfs access, namespaces, Docker metadata access, and socket ownership.
- [x] Explain any agent host-level privilege; do not call a read-only Docker socket mount a read-only API.
- [x] Define registration policy requiring the expected agent parent, deployment metadata, executable digest, and image identity where supported. Check for broader duplicate grants.
- [x] Confirm actual callers use equal UID 1000/mapping and reach the Workload API and real money API in M4; unidentified rejection is issuance/TLS, not reachability.
- [x] Prove the approved executable in the approved deployment gets the intended SVID.
- [x] Prove the unidentified executable receives no matching SVID, rather than merely failing socket permissions.
- [x] Build a controlled test fixture containing approved and probe executables; hold deployment selectors and UID constant, vary the executable, and prove denial.
- [x] Prove copying the approved executable into an unapproved deployment does not grant the trusted identity.
- [x] Review digest scope, replacement/PID races, selector caching, and binary-update behavior against the chosen implementation.
- [x] Save sanitized attestation evidence, effective selectors, registrations, and versions.
- [x] Verified built-in support suffices for the measured layout; no custom attestor or weaker selector policy was introduced.

**Gate B:** Passed for the measured UID-1000/native-agent feasibility layout. See `docs/attestation-feasibility.md` and sanitized JSON evidence. T04/T05/T21 issuance cases pass; The P1 T03 proof used a network sentinel; M4 additionally verifies actual same-UID caller connectivity to money. Repeat after UID/agent-layout changes. Full application acceptance remains open.

## P2 — Scaffold and identity infrastructure

Dependencies: Gate B. Rudi approved retained project-local CA/database/agent state under `.runtime/identity/`. Restart/permissions/loopback tests pass; see `docs/implementation-status.md`. UI UID/agent layout and browser provisioning remain unresolved.

- [x] Create the Go module, dependency locks, formatting/test conventions, and planned internal service/caller packages.
- [ ] Complete the planned UI binary/packages after M5 configuration/layout decisions.
- [x] Add `.gitignore` rules for credentials, runtime state, generated bundles, OIDC sessions, and local configuration secrets.
- [x] Create Compose infrastructure, network/volume boundaries, health/readiness checks, and minimal application images.
- [x] Bootstrap one SPIRE server/agent with explicit node attestation and protected administration.
- [x] Persist SPIRE state separately; define normal restart versus explicit trust-domain reset.
- [x] Generate narrow registrations after final binary/image digests are known; keep the registration authority outside applications.
- [x] Provide SDK SVID/bundle sources and source shutdown in Go; actual SVID rotation passes. Public no-SVID bundle publication now refreshes atomically; real short-TTL CA rollover and outage/expiry pass. Native taint/admin root removal and node-expiring recovery also pass; automatic expired-root sweeping and production 24-hour rollover duration remain unrun.
- [ ] Establish protected browser TLS/OIDC secret provisioning; never commit tokens or use default shared credentials.
- [ ] Review application hardening: non-root, caller UID equality, dropped capabilities, no-new-privileges, read-only binaries/root filesystem, separate PID namespaces.
- [x] Confirm no app can access Docker/admin sockets, SPIRE server state, or another caller's control volume.

**Gate C:** Measured native UID-1000 substrate/restart passes with protected retained state and no remote exposure. Different-UID UI and browser provisioning remain blocked; no broader privilege claim is made.

## P3 — Money API and in-memory ledger

Dependencies: Gate C.

- [x] Serve SPIFFE TLS directly in the Go money process; require client SVIDs and verify them with the SDK.
- [x] Use TLS 1.3 for the internal API; do not introduce plaintext or optional-client-cert listeners.
- [x] Enforce the exact route policy: trusted ID can transfer; UI ID can read balances; all others denied.
- [x] Verify the expected money SPIFFE ID on client connections; no verification bypass or broad trust-domain server match.
- [x] Implement bounded transfer input with fixed accounts, fixed amount/currency, and integer minor units.
- [x] Implement atomic debit/credit/version updates and insufficient-funds/overflow checks.
- [x] Implement verified-identity-bound idempotency, canonical payload comparison, same-key conflict behavior, and bounded history without unsafe eviction.
- [x] Preserve a request ID on retry after an unknown outcome; document process-lifetime-only guarantees.
- [x] Add structured authorization/transfer audit events without credentials or unverified identities.
- [x] Bound requests, use fresh internal handshakes without connection reuse/session resumption, and reject expired credentials before authorization; test real rotation behavior.
- [x] Unit-test authorization, input handling, ledger invariants, concurrency, and deduplication.
- [x] Integration-test no cert, fabricated identity headers, self-signed cert with allowed URI, wrong valid SPIFFE ID, and wrong server identity.

**Gate D:** Direct mTLS/route and ledger tests pass. T06–T09 and actual T17 rotation/expiry/reuse pass; T13–T15 have integration/unit coverage. The backend T16 fixture now closes the TLS connection after a real Money commit, with unknown-outcome/same-ID retry proof; opaque proxy TLS-stream loss also passes; TCP/IP retransmission/arbitrary packet-loss injection remains unrun. See `docs/backend-final-status.md` for limits.

## P4 — Caller processes and private trigger channels

Dependencies: Gate D.

- [x] Implement separate caller executables; each initiates its own network connection to money.
- [x] Create separate per-caller control UDS volumes and directory/socket ownership; only the local UI-equivalent test harness mounts both; real UI remains M5.
- [x] Reject arbitrary URLs, amounts, identity strings, headers, and commands in the control protocol.
- [x] Trusted caller obtains its own SVID and performs the fixed transfer with a stable request ID.
- [x] Unidentified caller records actual Workload API eligibility failure with a bounded deadline.
- [x] Implement its intentional no-client-certificate HTTPS probe, still verifying the money identity using a protected public trust bundle.
- [x] Flag unexpected SVID issuance to the unidentified process as a security failure.
- [x] Distinguish no SVID, TLS rejection, server-identity mismatch, HTTP denial, success, timeout, and unknown commit outcome.
- [x] Confirm callers expose no network control listener and the unidentified caller cannot mount/reach the trusted control socket.

**Gate E:** Internal caller portions of T01–T02 and T12 pass through the controlled UID-10002 local harness. Remote operator authentication is not implemented or accepted. Infrastructure errors cannot count as identity rejection.

## P5 — Original remote UI and verified-human security (superseded)

Dependencies: Gate E. Blocked: measured agent cannot hash UID-10002 UI executables; owner must choose UI UID/agent privilege layout, and supply non-secret HTTPS/OIDC/operator configuration. Complete all remote controls before permitting remote traffic.

- [ ] Serve remote browser HTTPS with a certificate valid for the configured DNS name; publish only the intended edge port.
- [ ] Implement OIDC Code + PKCE, state/nonce/issuer/audience/signature checks, exact callback URI, and safe redirects.
- [ ] Use opaque server-side sessions, secure HttpOnly cookies, login rotation, idle/absolute expiry, and CSRF-protected logout.
- [ ] Enforce default-deny operator/viewer roles on the backend. Both buttons require operator permission.
- [ ] Add CSRF tokens, exact origin/Host checks, same-origin API policy, and no state-changing GET routes.
- [ ] Add CSP/frame protection, escaped templates, no-store responses, sanitized errors, and no third-party scripts.
- [ ] Add bounded bodies/timeouts, per-operator rate limits, concurrent-attempt limits, and bounded status history.
- [ ] Implement attempt creation/polling, subject-bound browser idempotency, and ownership checks for result access.
- [ ] Derive stable money request IDs from operator identity + browser key; prove a UI restart/re-login does not turn a retry into a second transfer.
- [ ] Read balances with the UI's own observer SVID; verify UI transfer attempts return `403`.
- [ ] Show two buttons, fixed transfer details, attempt stages, evidence provenance, and balances/version.
- [ ] Clearly label expected versus verified SPIFFE IDs and distinguish UI HTTP status from money HTTP status.
- [ ] Test anonymous user, unauthorized logged-in user, bad/expired token/session, CSRF, foreign origin, and wrong Host cases.
- [ ] Verify missing authentication/certificate configuration fails startup/readiness instead of enabling anonymous access.

**Original Gate F (retired for the simplified profile):** Not passed: T10–T11 and T19 are unimplemented/unrun. These remain requirements only for a future verified-human profile. The current owner-approved public demo explicitly assumes the operator; its separate completed checks are above.

## P6 — Full verification and handoff

Dependencies: Gate F.

- [ ] Run the entire RFC T01–T23 matrix with real SPIRE and direct Go TLS connections.
- [x] Verify the bounded backend lifecycle subset: short-TTL CA rollover/public bundle, registration removal/cached expiry, server/agent/socket/trust faults, post-commit connection drop, Money restart, and HTTP history-capacity boundary. See `docs/backend-acceptance.md`. This does not complete the UI/full acceptance gate.
- [x] Verify native old-root taint/revocation propagation, explicit reviewed executable update, unchanged selector/parent/lifetime boundaries, and recovery after a node/workload-expiring outage with explicit fixture-only re-enrollment. See `docs/backend-extended-acceptance.md`.
- [x] Verify bounded application-proxy handshake/response loss with preserved end-to-end mTLS, accurate infrastructure/unknown-commit outcomes and stable same-ID retry; inspect its no-mount/no-port/no-privilege boundaries. See `docs/backend-final-status.md`.
- [ ] Run automatic expired-root sweeping and production-TTL rollover observations only if separately authorized for long duration. SPIRE 1.14.0 fixes a 24-hour post-expiry grace/six-hour sweep independently of accelerated CA TTL; no bounded acceleration is supported. TCP/IP retransmission/arbitrary packet-loss injection and the full UI matrix remain unrun. No long-duration job was initiated.
- [ ] Inspect published ports, app/agent privileges, mounted volumes, registration breadth, cookies, and logs.
- [ ] Verify rate/capacity limits, concurrency, retry-after-lost-response, and documented restart/reset behavior.
- [ ] Save sanitized evidence for every test; mark untested cases as untested rather than inferred passes.
- [ ] Add tested setup, bootstrap, run, inspect, remote login, troubleshooting, and teardown commands to README/runbook.
- [ ] Document how to update binary digests/registrations and how to rotate browser/OIDC secrets.
- [ ] Document demo reset separately from destructive SPIRE CA reset; disable/remove test-only identities and fixtures from the normal profile.
- [x] Reconcile current backend RFC coverage, intentional limits and remaining owner decisions in `docs/backend-final-status.md`; full UI/owner acceptance remains open.
- [ ] Obtain owner acceptance based on evidence, not only working buttons.

**Gate G / done:** Authorized remote operator sees trusted success and unidentified TLS rejection; attestation distinguishes executables with the same UID; wrong identities and unauthorized humans cannot move balances; the documented security matrix passes. This remains a fake-money PoC, not a production payment system.
