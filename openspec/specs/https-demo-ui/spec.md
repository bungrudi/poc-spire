# HTTPS demo UI

## Purpose

Define the owner-approved public assumed-operator learning demo, its HTTPS/session/CSRF controls, stable caller orchestration and separate optional local owner-login preview.

## Requirements

### Requirement: UI-01 Explicit assumed operator and shared demo scope
The public demo SHALL disclose that its human operator is assumed, not authenticated. Anyone reaching GET / SHALL be able to obtain a session with issuer urn:poc-spire:assumed and subject demo-operator. OIDC/login endpoints MUST NOT be exposed. Principal-bound attempts/rate limits in this profile MUST be described as shared demo state, not per-human privacy or authorization.

#### Scenario: Visitor opens the public page
- **WHEN** any visitor opens the exact configured HTTPS origin
- **THEN** the page discloses the assumption and issues a session permitting the fixed demo attempts

#### Scenario: Direct API request without session or login route
- **WHEN** a visitor calls a protected API without a session or requests a login endpoint with a demo session
- **THEN** the API requires a session with 401 and login routes return 404
- **AND** this does not establish human authentication because any visitor can obtain the session

### Requirement: UI-02 Fail-closed HTTPS and browser certificate replacement
Demo startup MUST require explicit demo mode, an exact HTTPS origin and a provisioned matching current browser certificate/key. Go UI SHALL serve HTTPS; the approved Nginx edge SHALL verify re-encrypted upstream HTTPS separately from workload PKI. Atomic replacement of the combined browser PEM SHALL load on the next handshake without resetting sessions or Money.

#### Scenario: Valid configuration and atomic renewal
- **WHEN** a valid hostname-matching PEM is provisioned or atomically replaced with another valid PEM
- **THEN** browser TLS handshakes use the current certificate while existing sessions and fake ledger continue

#### Scenario: Missing or invalid TLS configuration
- **WHEN** demo configuration has missing/mismatched/expired/not-yet-valid certificate material or a plaintext origin
- **THEN** startup or certificate loading fails without enabling anonymous plaintext fallback

### Requirement: UI-03 Session and browser request defenses
Demo sessions MUST be opaque in-memory sessions with Secure/HttpOnly/SameSite=Lax cookies, 15-minute idle and one-hour absolute expiry. POSTs MUST require a matching CSRF token and exact Origin/Host, rejecting cross-site fetches and forwarded-host overrides. Responses SHALL use escaped templates, restrictive CSP, frame protection and no-store caching.

#### Scenario: Current session posts same-origin request
- **WHEN** a valid demo session supplies its CSRF token from the exact allowed origin/Host
- **THEN** the requested supported operation can proceed and logout invalidates that session

#### Scenario: Forged browser boundary or expired session
- **WHEN** CSRF is missing/wrong, origin/Host is foreign, fetch is cross-site or the session is expired
- **THEN** the UI rejects before dispatching a caller command, with 403 for the boundary or 401 for missing/expired session

### Requirement: UI-04 Fixed asynchronous attempts and stable retry
UI SHALL accept only trusted/unidentified caller choices and a browser_key matching [A-Za-z0-9_-] of length 32–128, derive a deterministic ID from issuer/subject/key and return 202 for accepted attempts. Pending/done repeats SHALL reuse the attempt; retryable failures SHALL redispatch the same ID. Result ownership MUST follow the principal, shared by all public demo sessions.

#### Scenario: Repeat after UI restart and renewed session
- **WHEN** a visitor repeats the same key after a UI-only restart while Money continues
- **THEN** the deterministic Money ID remains unchanged and Money replays without a second debit

#### Scenario: Conflict, invalid payload or unknown outcome
- **WHEN** a retained key selects a different caller, extra fields/invalid keys are supplied or the outcome is unknown
- **THEN** conflict returns 409, malformed input returns 400 and an unknown outcome remains retryable with the original ID

#### Scenario: Another principal requests an attempt
- **WHEN** a distinct local-preview principal requests an attempt it does not own
- **THEN** it receives 404
- **AND** public demo sessions are all the same assumed principal and have no such per-human separation

### Requirement: UI-05 Read-only workload identity and bounded resources
UI MUST read balances using only its own observer SVID and verify Money's exact ID, while triggering callers over private sockets. Defaults SHALL bound attempt history to 64, concurrent dispatch to 2, dispatch rate to 8 per minute per principal and sessions to 128. New work at limits MUST be rejected without unsafe history eviction; browser attempt bodies MUST be at most 1024 bytes.

#### Scenario: Verified balance display and existing attempt lookup
- **WHEN** UI reads Money or an existing pending/done attempt is repeated at history capacity
- **THEN** it returns verified balances or the existing attempt without borrowing caller credentials or creating another dispatch
- **AND** an existing retryable attempt may redispatch with its original ID even at full history capacity, subject to rate/concurrency limits

#### Scenario: Forbidden transfer or resource saturation
- **WHEN** UI's SVID directly posts a Money transfer or new browser work exceeds configured limits
- **THEN** Money returns 403; UI returns 429 for concurrency/rate or 503 for capacity without eviction
- **AND** unavailable Money reads return an infrastructure failure, not fabricated balances

### Requirement: UI-06 Separate local owner-login preview
The optional local profile SHALL use a private UI Unix socket behind a literal loopback HTTP bridge and a separate owner-only admin Unix socket. Kernel UID checking SHALL issue one-use, one-minute login codes, with origin checks and session rotation. This local login MUST NOT be exposed as the public demo or described as OIDC.

#### Scenario: Local owner logs into preview
- **WHEN** the configured OS owner obtains a code through the private admin socket and submits it once from the loopback origin
- **THEN** a local operator session is issued and the previous session is rotated

#### Scenario: Local login misuse or non-loopback configuration
- **WHEN** a code is invalid/expired/reused, the admin peer is unauthorized or local configuration is non-loopback
- **THEN** login/configuration is rejected without caller dispatch
- **AND** demo mode does not enable the local admin listener

## Traceability and limitations

See [UI-01–UI-06 sources, checks and evidence](../../../docs/spec-baseline.md#https-demo-ui). Browser keys are deduplication inputs, not authorization credentials. Full restart resets trust/ledger/history; UI-only restart is different. Actual scheduled ACME renewal is still unobserved: issuance, manual renewal-hook continuity and atomic PEM tests are recorded. Local unit fixtures exercise viewer/owner policies and do not prove public human authentication.

The [UI handler](../../../internal/web/app.go) returns HTTP 200 with `status: alive` for GET `/healthz`. This reports application liveness; it does not check Money/SPIRE dependency readiness.
