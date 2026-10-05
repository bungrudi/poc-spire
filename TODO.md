# TODO — SPIRE process-identity PoC

**Current phase: measured identity substrate, money API and internal caller/control path verified (M2–M4). M5 awaits UI UID/agent-layout and HTTPS/OIDC configuration decisions. PoC state retention is explicitly approved.**

Checked boxes mean work actually completed. Unchecked boxes are planned work, not tests that passed. Technical decisions live in [RFC 0001](docs/rfcs/0001-process-identity.md); workspace requirements live in [WORKSPACE.md](WORKSPACE.md).

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

## P5 — Remote UI and operator security

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

**Gate F:** Not passed: T10–T11 and T19 are unimplemented/unrun. Only after they pass may the chosen remote exposure be enabled. A reachable trusted trigger without human authorization is a release blocker.

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
