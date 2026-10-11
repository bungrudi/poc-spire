# RFC 0001: Direct process-attested transfers with SPIRE

## Owner supersession — 11 October 2026

The owner requested `spire.adianto.dev` and explicitly removed OIDC, asking to
assume human authentication to focus on SPIRE. The current demo labels its
operator as **assumed, not authenticated**; anyone with access may trigger fake
transfers. This supersedes all earlier mandatory OIDC/verified-human gates,
claims of protection against unauthorized humans, and different-UID UI planning
for this simplified profile. Workload security remains mandatory. The UI runs
in a separate UID-1000 container using the measured unprivileged native agent.
The owner specifically approved the public/persistent installation, which is
now installed and verified at `https://spire.adianto.dev/`. See [current profile and exact plan](../remote-demo.md).
Original human-security tests remain historical requirements, not passing tests
or claims about this public demo.


- **Status:** implementation authorized; identity substrate, money API and internal callers/control verified in the measured UID-1000/native-agent layout; simplified HTTPS UI implemented; isolated integration passes; owner-approved public deployment live and verified
- **Scope:** Go services on one Docker Compose host; fake money only
- **Decision authority:** project owner
- **Related:** [workspace brief](../../WORKSPACE.md), [TODO list](../../TODO.md), [delivery plan](../plan.md)
- **Supersedes:** earlier localhost-only UI assumption

## 1. Summary

Build a remote HTTPS learning demo with an assumed operator with two buttons: **Transfer as trusted process** and **Transfer as unidentified process**. Each button tells the selected Go process to initiate its own direct connection to a money service. The money service requires SPIFFE mTLS and authorizes the exact caller SPIFFE ID in Go. It does not trust browser-supplied identities, network location, or a sidecar's forwarded identity headers.

Recommend separate caller containers with the same numeric UID, plus executable-bound SPIRE workload attestation. A dedicated test runs a different executable under the trusted deployment's other attestation conditions. This separates the executable test from the container test without presenting same-container arbitrary-code execution as a safe security boundary.

The owner intentionally assumes the **human operator** for this learning demo. Workload mTLS proves caller identity, not human authentication. Any visitor can trigger a fake transfer through the UI; this is disclosed on the page.

### Terminology

- **SPIFFE ID:** workload name, represented as a URI; not a secret or a certificate by itself.
- **X.509-SVID:** short-lived X.509 credential binding a SPIFFE ID to a key.
- **SPIRE:** issues SVIDs after node/workload attestation and registration matching.
- **Workload API:** local Unix-socket API through which an eligible process obtains its identity and trust material.
- **Process-attested identity in this RFC:** identity granted to a verified executable within an approved deployment. It is not a persistent PID identifier or a guarantee of uncompromised memory.

## 2. Goals and non-goals

### Goals

1. Authenticate and authorize transfers in the Go money process, end to end from the caller.
2. Demonstrate that same UID, network reachability, knowledge of the expected SPIFFE ID, and Workload API socket access are insufficient to impersonate the trusted executable.
3. Make both the successful transfer and the identity-based rejection visible in the remote UI.
4. Retain browser CSRF/origin/Host controls and duplicate prevention while disclosing the explicit assumed-human scope.
5. Keep startup, teardown, identity inspection, and negative tests reproducible with Compose.
6. Describe limits explicitly; avoid equating container identity, executable attestation, and runtime integrity.

### Non-goals

- Real money, persistent accounting, bank integration, Kubernetes, service mesh, multi-host federation, or high availability.
- A distinct SPIFFE ID per PID or per click.
- Resistance to a compromised Docker host/daemon, SPIRE control plane/agent, UI, or authorized caller.
- Protection after an authorized private key/session is stolen; immediate revocation of already-issued credentials.
- An end-to-end cryptographically signed user transaction protocol, hardware-backed keys, or continuous code-integrity measurement.
- Complete prevention of resource exhaustion by hostile traffic. Basic limits are included, not a DDoS guarantee.

## 3. Design decisions

| ID | Proposed decision |
| --- | --- |
| D1 | Separate caller containers, same numeric UID and UID mapping; different Go executables. |
| D2 | Require both deployment-bound selectors and an agent-derived executable digest. Prefer built-in attestors verified for the selected release. |
| D3 | Strict SPIFFE mTLS on every money API route. Authenticate the trust-domain credential during TLS; apply exact identity/route policy in Go before any operation. |
| D4 | Return HTTP `403` only for an authenticated identity lacking permission. Missing/invalid credentials fail TLS and have no HTTP status from the money API. |
| D5 | Remote Go UI serves browser HTTPS with an explicitly assumed operator, secure in-memory sessions and CSRF protection. No human login. |
| D6 | Per-caller private Unix-domain sockets carry trigger commands. No caller listens for triggers on the transfer network. |
| D7 | UI has a separate, read-only workload identity for balance display; it cannot invoke the transfer API directly. |
| D8 | Fixed fake transfers, integer minor units, atomic ledger mutation, and in-memory idempotency. |

These are design recommendations. Implementation was authorized on 5 October 2026; formal owner acceptance and unresolved deployment choices remain open. See [P1 evidence](../attestation-feasibility.md).

## 4. Architecture and trust boundaries

```text
Remote browser [human identity assumed]
      |
      | HTTPS :443 + demo session + CSRF
      v
Go UI / control service [identity: /ui]
      |                                  |
      | private trusted-control UDS      | private untrusted-control UDS
      v                                  v
Trusted caller [UID N]              Unidentified caller [UID N]
[identity: /caller/trusted]         [no SVID]
      |                                  |
      | direct SPIFFE mTLS               | HTTPS probe, no client SVID
      +------------------+---------------+
                         v
              Go money service [identity: /money]
              TLS verification -> identity policy -> ledger
                         ^
                         |
                UI mTLS GET /balances only

SPIRE server <--- node attestation / issuance ---> SPIRE agent
                                                    |
                                      local Workload API Unix socket
                                      | money | UI | both callers |
```

### Components

| Component | Responsibility | Sensitive material |
| --- | --- | --- |
| SPIRE server | CA/trust domain, registrations, node attestation, issuance | Signing keys, admin authority |
| SPIRE agent | Attest requesting processes and serve Workload API | Agent credentials, cached workload credentials |
| Money service | Terminate workload mTLS, exact-ID authorization, ledger | Its own SVID/private key, balances |
| Trusted caller | Receive a bounded trigger, obtain its SVID, call money directly | Its own SVID/private key |
| Unidentified caller | Try identity acquisition, then demonstrate a no-client-certificate rejection | No authorized SVID; public server trust bundle only |
| UI/control service | HTTPS, OIDC/session/roles/CSRF, trigger dispatch, read-only balances | OIDC client/session secrets, browser TLS key, its own SVID |
| OIDC provider | Authenticate humans; supply validated identity/role claims | Operator accounts, signing keys |

### Network and volume layout

- Publish only the browser HTTPS port on the Compose host. Firewall policy may further restrict it to an operator network/VPN. Remote access must still work; localhost binding is no longer the design.
- Money listens only on the Compose application network. Trusted caller, unidentified caller, and UI all reach it. Identity checks must hold even if an attacker also joins or reaches this network.
- SPIRE server and agent communicate on a separate infrastructure network; no public SPIRE administration or Workload API TCP proxy.
- Callers have **no network trigger port**. Mount `trusted-control` only into UI + trusted caller, and `untrusted-control` only into UI + unidentified caller. Use separate volumes, not two directories under a broadly shared mount.
- Money, UI, and both callers can connect to the agent Workload API socket. Agent owns the socket directory; applications cannot replace the socket. Socket permissions must permit the test UID(s) without using arbitrary host-wide writable directories.
- Applications do not receive the Docker socket, SPIRE server admin socket, server state, or another workload's private keys.
- The SPIRE agent may need host process visibility and Docker metadata access. It is trusted infrastructure, not a transfer-authenticating sidecar. Its exact namespace/capability/mount requirements are a feasibility gate; do not apply application sandbox assumptions blindly to the agent.
- A `:ro` Docker socket mount does **not** make Docker API calls read-only. Prefer a narrowly restricted metadata proxy if supported; otherwise document that agent Docker socket access carries host-level trust.

## 5. Identity and attestation design

### 5.1 Proposed SPIFFE identities

Use a clearly non-production trust domain, provisionally `poc.internal`:

| Workload | SPIFFE ID | Money API permissions |
| --- | --- | --- |
| Money | `spiffe://poc.internal/money` | Server identity only; no implicit client privilege |
| Trusted caller | `spiffe://poc.internal/caller/trusted` | `POST /transfer` |
| UI | `spiffe://poc.internal/ui` | `GET /balances` |
| Unidentified caller | None | None |
| Test-only wrong-ID client | `spiffe://poc.internal/test/wrong-id` | None |

Use exact parsed ID comparisons, not suffix/prefix matching or 'any identity in this trust domain'. The UI identity is a separate observer/control workload, not a third transfer caller in the main demo. A wrong-ID test registration exists only in the security-test profile and is removed afterward.

### 5.2 Same user, different executable: recommendation

Run both caller containers as UID `10001`, with the same host mapping where user namespaces are involved. Each has its own PID/mount namespaces, no capabilities, no privilege escalation, no writable application binary, and no access to the other's control volume or memory.

Running both processes in one container is a weaker practical boundary: depending on kernel configuration, same-user code can inspect processes, signal them, exploit shared writable files, or steal credentials. A hash selector does not solve those problems. Use co-location only as a controlled **attestation test**, not the default deployment or a claim of isolation after compromise.

### 5.3 Logical registration policy

For the trusted identity, require a conjunction resembling:

```text
expected attested agent parent
AND approved deployment role/metadata controlled by the operator
AND approved immutable image identity where supported
AND digest of the requesting process's executable
AND optional expected UID as defense in depth
```

This is policy pseudocode, **not SPIRE configuration or verified selector syntax**. Avoid a second, broader registration entry that accidentally grants the same SPIFFE ID without all restrictions. Verify how selectors from multiple attestors combine in the pinned SPIRE release.

Evaluate the built-in Unix workload attestor's executable hashing capabilities together with the Docker workload attestor. Verify exact plugin options, selectors, platform support, caching behavior, and PID namespace handling from release-specific documentation/source. Previous planning could not retrieve web documentation; no plugin configuration has been validated here.

A digest must be derived by the agent from the authenticated Unix peer's kernel-observed process, not from request parameters, environment variables, a claimed PID, or a file path supplied by the workload. Verify the agent measures the actual running executable through a trusted process/filesystem view and handles process exit/PID reuse and binary replacement safely. Use static Go binaries without interpreters or dynamically loaded application plugins for the initial proof. The digest does not cover mutable configuration, all memory, or arbitrary runtime inputs.

Image digest and deployment metadata serve different purposes. A copied approved executable in an unauthorized container must not qualify solely because its bytes match. Conversely, matching a container label or image must not let a different executable inherit the authorized identity. Labels are useful only because the Docker administrator is trusted and applications cannot relabel/create authorized deployments.

If built-in support is insufficient, stop for review. A custom attestor would expand the trusted computing base and testing burden; it requires a separate design/approval, not an automatic workaround. UID/path/label-only matching is not an acceptable silent downgrade.

### 5.4 Required evidence

1. Record runtime, SPIRE release, effective UID mapping, executable digests, selected attestors, actual observed selectors, and registration entries.
2. Show both callers have Workload API socket access and money network connectivity.
3. Show the trusted executable gets the intended SVID; the unidentified executable gets no matching identity.
4. In a test-only image, include both the approved binary and a distinct probe binary. Register that fixture's deployment/image plus the approved binary digest. Run the probe with the same UID in the same fixture container: other selectors remain satisfied, only the executable differs. It must not get the trusted SVID. The privileged test harness may use Docker exec; runtime apps may not.
5. Run the approved binary in a deployment lacking the approved deployment metadata. It must not get the trusted identity either.

The fixture test proves issuance filtering, not safety against an attacker already able to read the trusted process's key or control its execution.

### 5.5 Node bootstrap and lifecycle

- One SPIRE server and one attested agent on the Compose host are enough. A controlled one-time join-token bootstrap is acceptable for this PoC; node identity and parent binding must be explicit.
- Bootstrap/admin operations run through a trusted setup path, never through the public UI. Do not put join tokens in committed files, logs, images, or long-lived app environment variables.
- Persist SPIRE server state separately from application volumes. Resetting the CA/trust domain is an explicit destructive action, not an accidental consequence of a normal app restart.
- Build first, record final binary/image digests, then generate the narrow registrations. Rebuilds require explicit digest/registration updates; never respond to a mismatch by broadening selectors.
- Use Workload API-backed Go SPIFFE sources for certificate and bundle updates. Keep private keys in memory; close sources during shutdown.
- A short configurable SVID lifetime is desirable; determine a supported value during validation. Removing a registration does not necessarily invalidate already-issued certificates or existing connections immediately.

## 6. TLS and authorization in the money process

The money service terminates TLS itself using a Workload API-sourced server identity. Require TLS 1.3 for this controlled Go-only internal PoC, with no early-data transfer path, plaintext listener, or optional-client-certificate mode.

At connection establishment:

1. Require a client certificate.
2. Verify the chain, validity, SPIFFE X.509-SVID profile, and intended trust domain using Go SPIFFE library support.
3. Make the verified peer identity available to request authorization. The handler must not parse an unverified raw certificate and call that authentication.

At each HTTP request:

1. Extract the authenticated peer ID from verified TLS state.
2. Match exact identity + method + route against the permission table.
3. Deny by default, before business logic or access to an existing idempotency result.
4. Validate bounded input and perform the operation.

TLS-level trust-domain acceptance is **authentication, not transfer authorization**. This placement permits a valid but unauthorized workload to receive HTTP `403`. An unidentified caller never reaches this handler. Do not weaken TLS to produce a friendlier status code.

Every legitimate client verifies the money service's exact SPIFFE ID `spiffe://poc.internal/money`, not just that it has some certificate from the trust domain. Never use `InsecureSkipVerify` as an application shortcut; any library-managed custom verification must perform complete SPIFFE verification.

No caller-supplied `X-SPIFFE-ID`, `X-Forwarded-Client-Cert`, user ID, source IP, DNS name, or JSON field can grant a permission. There is no proxy between caller and money service that supplies trusted identity headers.

### Credential expiry and long-lived connections

Rotation affects new TLS handshakes; it does not reauthenticate an already-established connection automatically. For this low-volume PoC, prefer a fresh internal TLS handshake per operation: disable internal connection reuse and session resumption, and bound request lifetimes. The money handler also checks credential expiry immediately before authorization. If pooling/HTTP2 is introduced later, it needs explicit expiry/re-authentication tests rather than relying on idle timeouts alone. Test rotation and expiry behavior; do not claim instant credential revocation. If SPIRE is unavailable, already-valid cached credentials may work until expiry, but no new unauthenticated mode is allowed.

## 7. Remote browser access and safe triggering

### 7.1 Workload identity and the assumed human

The human principal is a declared constant (`urn:poc-spire:assumed`,
`demo-operator`), not a verified login. Anyone can obtain a demo session by
opening `/`. Money still authenticates real SPIFFE workload IDs during TLS and
authorizes the exact ID by route. The browser never presents caller credentials.

### 7.2 Browser HTTPS and sessions

Go UI serves HTTPS with a DNS-valid certificate. The proposed existing VPS Nginx
edge terminates browser TLS and re-encrypts to Go UI, verifying its exact DNS
name and certificate using ordinary web PKI. Neither proxy terminates workload
mTLS. This specific edge installation was approved and is live.

Opaque in-memory sessions use Secure/HttpOnly/SameSite=Lax cookies, 15-minute
idle and one-hour absolute expiry. They bind CSRF controls, not a human identity.
POST `/auth/logout` clears a session with CSRF protection. OIDC is removed.
Missing or invalid browser TLS configuration fails startup.

### 7.3 Browser defenses

- Both buttons are available to the assumed demo operator. No human-access
  restriction or anonymous-user-denial claim is made.
- CSRF tokens and exact Origin/Host checks protect state-changing routes.
  Forwarded identity or Host headers never substitute for policy.
- Escaped templates, a restrictive CSP, frame protection, no third-party
  scripts and no-store responses remain.
- Fixed transfer input, request deadlines, bounded concurrency/history and a
  rate limit shared by the assumed principal remain. These are demo limits,
  not production abuse or denial-of-service protection.

### 7.4 Private control channel

Each caller exposes a small HTTP-over-UDS control endpoint to the UI. The UI is the only other container with that volume. Have the caller own the socket directory and allow the UI group to connect but not replace the socket. The measured demo uses a separate UID-1000 UI container; kernel peer checks enforce that UID, while separate mounts provide cross-caller isolation. Test permissions at startup. No world-writable socket directory.

The trigger message contains a server-created attempt ID and bounded correlation metadata. It does **not** accept an arbitrary upstream URL, arbitrary headers, SPIFFE ID, command line, shell command, or account/amount selection. Target money address and transfer parameters are fixed deployment configuration. This prevents the control API becoming a general HTTP proxy or command runner.

The UI can reach both control sockets. The unidentified caller cannot reach the trusted caller's control socket even though the callers share a UID. Socket permissions alone are not the distinguishing mechanism here; separate mount visibility is essential. The private control boundary is distinct from the deliberately shared Workload API socket used for attestation tests.

## 8. Request flows and failure semantics

### 8.1 Trusted button

1. Browser submits an assumed-operator, CSRF-protected action with a stable idempotency key for this logical click.
2. UI validates session/role/CSRF, reserves an attempt ID, and sends a fixed command over the trusted caller's UDS.
3. Trusted caller obtains/uses its live SVID and establishes direct mTLS to the expected money identity.
4. Money authenticates, checks exact transfer permission, validates the request, atomically applies or retrieves the idempotent result, and records an audit event.
5. Caller returns structured evidence. UI displays the attempt outcome and reads balances using its separate read-only SVID.

### 8.2 Unidentified button

1. Same demo-session and CSRF checks; UI triggers the unidentified caller over its separate UDS.
2. The caller tries the Workload API with a bounded deadline and records that no matching SVID is available. Distinguish attestation denial from an unavailable socket/agent.
3. As an explicit **negative test mode**, it then attempts HTTPS to the money service with **no client certificate**. Use a public trust bundle supplied through trusted bootstrap to verify the exact server SPIFFE identity. Do not assume an unregistered process can fetch trust material from the Workload API; verify support or use the read-only bootstrap bundle.
4. Money rejects TLS. No HTTP transfer is processed and balances do not change.
5. UI shows both facts: no SVID and server-side TLS rejection. If the probe instead fails DNS, connection, or server verification, label it an inconclusive infrastructure failure, not proof of client-identity enforcement.

The no-certificate probe is intentional test behavior, not an operational fallback to insecure HTTP. It never disables server verification, sends plaintext, or borrows another identity. If the supposedly unidentified executable unexpectedly receives an SVID, flag a security-test failure; do not silently ignore that fact.

### 8.3 Valid but unauthorized identity

The test-only wrong-ID client (or the UI attempting a forbidden transfer) completes valid SPIFFE TLS, then receives HTTP `403`. This demonstrates the difference between trust-domain membership and application permission. Its request must leave balances and idempotency records unchanged.

### 8.4 Outcome taxonomy

| Outcome | HTTP from money? | Evidence |
| --- | --- | --- |
| No eligible SVID | No request yet | Caller Workload API diagnostic |
| TLS client authentication rejected | No | TLS failure and server-side connection evidence where available |
| Server identity mismatch | No | Client-side verification failure |
| Authenticated identity denied | `403` | Money authorization event with verified peer ID |
| Transfer applied or idempotent result returned | `200` | Money response/audit event with verified caller ID |
| Validation/conflict/insufficient funds | `400` / `409` | Authorized request rejected without mutation |
| Operator denied | No money request | UI `401` / `403` |
| Infrastructure timeout/error | Possibly unknown | Separate error/unknown state; never claim an attestation success |

If the response is lost after commit, report the result as unknown until retrying the **same** request ID. Never create a new transfer ID automatically after a timeout.

## 9. Proposed API and data contracts

Money and private caller contracts are implemented and tested; simplified assumed-operator browser contracts are implemented. See [implementation checkpoint](../implementation-status.md) for measured coverage.

### 9.1 Browser-facing routes

| Method/path | Access | Behavior |
| --- | --- | --- |
| `GET /` | Public assumed operator | Issue/renew demo session; buttons, balances, evidence explanation |
| `POST /auth/logout` | Session + CSRF | Invalidate local session |
| `POST /api/attempts` | Operator + CSRF + idempotency key | Accept only `trusted` or `unidentified`; create bounded attempt |
| `GET /api/attempts/{id}` | Authorized session | Read sanitized result; restrict to owner unless explicitly granted broader access |
| `GET /api/balances` | Viewer/operator | UI reads money with its own read-only SVID |
| `GET /healthz` | Public | Minimal status only |

Return `202` + attempt ID for a newly accepted action; poll status rather than keeping a browser connection open across Workload API delays. Bind browser idempotency keys to `(issuer, subject, key)` and record the selected caller. Reusing a key with a different caller returns `409` while that UI record exists. This UI deduplication complements, rather than replaces, the money service's idempotency.

Require a high-entropy browser key for each logical click, retained for retries. Derive the money `request_id` deterministically from a domain-separated, unambiguously encoded `(issuer, subject, browser key)` using SHA-256; do not make it a fresh random value on every dispatch. Thus, after UI restart and a renewed demo session, retrying the same logical transfer still reaches the money service with the same request ID even if UI attempt history was lost. UI attempt display IDs may be separate. This derivation is a deduplication mechanism, not an authorization credential.

### 9.2 Money-facing routes

- `POST /transfer`: trusted caller ID only. Required fields: `request_id`, `from`, `to`, `amount_minor`, `currency`. Proposed fixed demo values: `demo-a`, `demo-b`, `100`, `USD`.
- `GET /balances`: UI ID only. Return integer balances and a monotonically increasing in-memory ledger version.
- All routes require mTLS. Any internal readiness route should be a local check or separately authenticated, not an unauthenticated shortcut to the ledger.

Successful transfer result: request ID, transfer ID, verified caller SPIFFE ID, amount/currency, resulting balances/version, and whether this was an idempotent replay. No private credentials or unverified identity claims in responses.

### 9.3 Attempt evidence

An attempt record contains: attempt ID, selected caller, timestamps, stage, outcome, money HTTP status **when one exists**, sanitized error category, transfer ID, and verified identity **only when supported by verified TLS/money evidence**. Keep configured expected IDs separate from observed IDs. The UI response's own HTTP status is not the money service's status.

Caller diagnostics and money audit events have separate provenance. A caller's self-reported identity is not authoritative. TLS failure may occur before an HTTP correlation ID exists; show timestamp/connection correlation as such rather than inventing a server-observed request ID.

### 9.4 Ledger semantics

- Start `demo-a` and `demo-b` at `100000` minor units each; a successful logical transfer moves `100` from A to B.
- Reject nonpositive amounts, overflow, unknown accounts, identical source/destination, other currency, or a request that differs from the fixed demo transfer.
- Authorize before parsing large input or consulting idempotency history; keep bodies small and bounded.
- A mutex or equivalent atomic critical section covers validation of funds, both balance updates, version increment, and idempotency-record insertion. Never debit without crediting.
- Key idempotency by `(verified caller SPIFFE ID, request_id)` and bind it to canonical transfer fields. After bounded syntactic parsing, check an existing key before new-operation business validation: same key/same payload returns the original result; same key/different payload returns `409`. Store accepted operations and deterministic business rejections consistently; unauthorized requests cannot populate or inspect this store.
- Bound the store; reject new operations on capacity exhaustion instead of evicting keys and silently allowing a forgotten duplicate. A restart/reset clears ledger and history together.
- Guarantees are process-lifetime only. This is not durable exactly-once payment processing. Concurrent independent clicks with different IDs are independent transfers; retrying one logical click must reuse its ID.

## 10. Threat model

### Trusted computing base

Linux kernel/host administrator, Docker daemon and deployment authority, SPIRE server/agent/attestors and registration authority, approved images/configuration, OIDC provider, UI/control service, trusted caller, and money service. Their secrets and runtime integrity are assumed intact for the primary claim.

### Adversaries and controls

| Capability or attack | Required control/outcome |
| --- | --- |
| Send arbitrary traffic to the remote UI | HTTPS, validated session, default-deny role, CSRF, limits; no trigger for unauthenticated/unauthorized user |
| Send arbitrary traffic to money from a reachable network | Mandatory client SVID verification, then exact-ID authorization |
| Spoof SPIFFE/forwarded headers, source IP, DNS, or JSON identity | Ignore as authentication evidence |
| Self-sign a certificate with the desired SPIFFE URI | Reject untrusted issuer/invalid SVID during TLS |
| Use a legitimate but wrong SPIFFE identity | `403`; no mutation |
| Run the unidentified binary under the same UID with socket access | No matching attestation registration; no authorized SVID |
| Execute a different binary under the approved deployment selectors in a controlled test | Executable selector fails; no trusted issuance |
| Copy the approved executable into an unauthorized deployment | Deployment selectors fail |
| Forge or reach a caller trigger over the transfer network | No network listener; separate control volumes; UI authenticates real operator actions |
| CSRF a logged-in operator or replay a captured request | CSRF/origin checks, session protections, TLS; same logical request ID cannot duplicate a transfer |
| Substitute a different server or intercept traffic | Exact server SPIFFE ID verification and TLS encryption/integrity |
| Duplicate/reorder requests or lose responses | Atomic ledger + identity-bound idempotency; unknown result resolved with same ID |

### Explicit limitations

- A host administrator or Docker API client can create eligible containers, read secrets, or change the policy. Container labels and the Workload API are not protection from that administrator.
- A compromised authorized process can legitimately make authorized transfers. Executable attestation is not continuous runtime integrity, code signing of every instruction, or transaction approval.
- A same-UID attacker already co-resident with the authorized process may attack its memory or shared state. The controlled substitution test does not negate that risk.
- Someone holding a valid private key/SVID can act as that workload until the credential/connection policy stops them. Registration deletion alone is not immediate revocation.
- A compromised UI can use its intended trigger capability. Stronger separation would require independently verified, per-transaction human approval at the money service, outside this RFC.
- TLS stops off-path spoofing and on-path modification, not authorized misuse, every denial-of-service attack, or replay initiated with a stolen active session. Idempotency prevents duplicate logical operations, not deliberate new requests by an authorized operator.

## 11. Runtime hardening and operations

- Native Linux Docker Engine is the baseline. Explicitly validate host PID/procfs access for attestation; do not assume Docker Desktop, user namespaces, or rootless mode behaves identically.
- Non-root application UIDs; same UID for callers, different UID for UI. Drop application capabilities, use `no-new-privileges`, default restrictive seccomp, separate PID namespaces, and no application host PID mode.
- Prefer minimal, digest-pinned images with static Go binaries, read-only root filesystems, and only required writable tmpfs/socket directories. No Docker CLI/daemon socket in application images/containers.
- Give the agent only measured permissions required by supported attestors; record any elevated permissions and why. Keep server/admin paths inaccessible to workloads.
- Secrets supplied through protected runtime files/secret mounts; Compose secret mounts are not automatically an encrypted secret manager. Protect host files and backups. No secrets in build arguments, URLs, committed `.env`, or screenshots.
- Readiness requires correct dependencies/configuration. `depends_on` alone is not proof of SPIRE readiness or eligible identity. Apply deadlines and distinguish not-ready from policy denial.
- On errors, fail closed. The assumed-human remote mode must be explicit and disclosed; never enable it as an error fallback. No identity-header mode, blanket trust-domain transfer permission, or certificate-verification bypass.
- Restarting UI invalidates sessions; restarting money resets balances and idempotency; SPIRE state persists unless explicitly reset. Document these independently.
- Provide structured audit events for verified identities, human action subjects, results, and correlation IDs. Do not log SVID PEM, private keys, OIDC tokens, cookie values, join tokens, or CSRF tokens.
- Keep result history bounded and redact UI-facing internal details. An attacker-controlled error string must never become executable HTML.

## 12. Verification matrix

The full application matrix remains **planned**. Measured M1–M4 and backend lifecycle subsets now pass; see [implementation checkpoint](../implementation-status.md) and [backend coverage](../backend-acceptance.md) for exact limits. The [final backend coverage map](../backend-final-status.md) reconciles T01–T23, including measured lifecycle/proxy cases and exact remaining limits. UI authentication/session/CSRF and full owner acceptance remain open. Integration tests must use real SPIRE issuance and direct Go TLS connections, not only mocked identity headers.

| ID | Scenario | Required observation |
| --- | --- | --- |
| T01 | Authorized operator clicks trusted | Caller obtains expected SVID; money records verified trusted ID; balances move once |
| T02 | Operator clicks unidentified | Workload API reachable, no eligible SVID; no-cert TLS probe rejected; no ledger change |
| T03 | Compare runtime users | Same numeric caller UID and intended mapping; both reach socket and money |
| T04 | Different executable in trusted fixture | Deployment selectors still match; executable selector prevents trusted issuance |
| T05 | Approved binary in unapproved deployment | Deployment restriction prevents trusted issuance |
| T06 | Direct no-cert request with forged SPIFFE headers | TLS rejection; no HTTP handler mutation |
| T07 | Self-signed cert containing allowed URI | TLS rejection; no mutation |
| T08 | Valid wrong-ID SVID, including UI identity on transfer | TLS may succeed; HTTP `403`; no idempotency/ledger mutation |
| T09 | Money impostor with wrong SPIFFE ID | Client rejects server before sending application data |
| T10 | Original verified-human authorization | Retired by owner for this simplified public demo; no human-denial claim |
| T11 | Missing/wrong CSRF token or foreign origin | Rejected by UI; no control command |
| T12 | Unidentified caller targets trusted trigger | No exposed network endpoint; trusted control volume inaccessible |
| T13 | Same transfer ID/payload retried concurrently | Exactly one ledger mutation within process lifetime; stable result |
| T14 | Same ID with different payload | `409`; no additional mutation |
| T15 | Invalid amount, overflow, insufficient funds, body over limit | Rejected atomically; conserved total balance |
| T16 | Response lost after money commit | Retry same ID resolves result without second debit |
| T17 | SVID rotation/expiry and attempted connection reuse | Fresh handshakes use current credentials; expired peer cannot transfer; baseline disables reuse/resumption |
| T18 | SPIRE down, socket missing, wrong server bundle | Fail closed; UI labels infrastructure failure, not proven identity rejection |
| T19 | Original OIDC claims / session expiry | OIDC retired; demo session expiry and renewed-session retry tested |
| T20 | Host ports, volumes, image configs, logs inspected | Only intended remote edge exposed; no credential leaks or cross-caller control mount |
| T21 | Rebuild changes binary digest | Old registration fails closed; explicit reviewed registration update needed |
| T22 | Restart and reset | UI-only restart preserves deterministic transfer IDs on retry; money restart resets ledger/history; SPIRE state persists unless explicitly reset |
| T23 | Request limits/history capacity reached | Bounded rejection; no unauthorized operation or duplicate due to key eviction |

For each run, save versions/config digests, command/scenario, expected outcome, actual outcome, sanitized logs, and ledger snapshots. Do not attach credentials. A TLS failure caused only by bad routing or server trust is not a passing no-client-identity test.

## 13. Implementation sequencing and gates

1. **Documentation approval and explicit coding authorization.** No code changes before this separate instruction.
2. **Attestation feasibility.** Pin release/runtime; prove same-UID, wrong-executable, and wrong-deployment cases before building the UI.
3. **Identity substrate.** Compose bootstrap, registrations, Workload API consumers, rotation, direct mutual verification.
4. **Money API and ledger.** Exact route authorization, atomicity, idempotency, tests.
5. **Caller/control path.** Private sockets, trusted call, intentional no-SVID probe, structured outcomes.
6. **Remote security and UI.** HTTPS/OIDC/roles/CSRF before any remote exposure; read-only balance identity and result display.
7. **Security verification and runbook.** Entire matrix, evidence, reset/diagnostics, limitations.

See [TODO.md](../../TODO.md) for checkable tasks. If executable-bound attestation cannot be demonstrated, stop. A container-only demo may be useful but is a different deliverable requiring an explicit scope change.

## 14. Alternatives considered

| Alternative | Reason not selected |
| --- | --- |
| UID-only attestation | Both callers share a UID; cannot meet the requirement |
| Docker labels/image only | Describes deployment/container, not the specific requesting executable |
| Binary hash only | Same approved binary in an unauthorized deployment could qualify |
| Same-container production topology | Weakens isolation and invites credential theft not prevented by hash attestation |
| Sidecar/mesh authenticates then forwards identity | Violates direct in-process validation requirement; adds header trust boundary |
| JWT-SVID bearer tokens for transfers | Adds token replay/audience handling; direct X.509 mTLS better fits this connection-authentication PoC |
| Network allowlist as primary control | Reachable/compromised network peer can spoof requests; not workload identity |
| Public unauthenticated demo buttons | Lets outsiders use the trusted caller as a deputy; defeats the stated security goal |
| TLS optional client certificates to produce `403` | Weakens anonymous-client boundary unnecessarily; show TLS failure in the UI instead |
| UI calls money with the trusted caller identity | Fails the requirement that the selected caller process originates the transfer |

## 15. Unresolved configuration and validation

Before implementation, select the pinned SPIRE/Go/SDK versions and verify the built-in attestors against their release-specific documentation and source. P1 verifies built-in selector behavior in the measured UID-1000/native-agent layout. Other agent namespaces/UIDs, automatic expired-root sweeping and production-duration rollover remain unverified. Real SVID/short-TTL CA rotation, administrative old-root taint/removal and temporary node-expiring recovery now pass. The final backend coverage map records pinned source and actual observations; short fixture CA TTLs cannot accelerate the fixed pruning grace/timer. Relevant sources to consult are the selected SPIRE release's Unix and Docker workload-attestor docs, SPIFFE Workload API/X.509-SVID specifications, and the selected Go SPIFFE SDK's TLS/source documentation.

Before remote deployment, choose the DNS name, certificate provisioning, host/runtime, network exposure, OIDC issuer/client, and operator mapping. Exact endpoints and configuration defaults must be reviewed with those choices. No unanswered configuration item permits a security bypass.
