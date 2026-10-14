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
Demo startup MUST require explicit mode, exact HTTPS origin and matching current browser certificate/key. Go UI SHALL serve HTTPS. The named public profile SHALL retain existing Nginx with verified HTTPS upstream and exact Host/Origin; private Kubernetes loopback access remains direct HTTPS. Browser TLS MUST remain separate from workload PKI and atomic PEM renewal SHALL preserve sessions/Money.

#### Scenario: Valid configuration and atomic renewal
- **WHEN** a valid hostname-matching PEM is provisioned or atomically replaced with another valid PEM
- **THEN** browser TLS handshakes use the current certificate while existing sessions and fake ledger continue

#### Scenario: Missing or invalid TLS configuration
- **WHEN** demo configuration has missing/mismatched/expired/not-yet-valid certificate material or a plaintext origin
- **THEN** startup or certificate loading fails without enabling anonymous plaintext fallback

#### Scenario: Kubernetes loopback browser access
- **WHEN** the isolated Kubernetes profile forwards its browser port
- **THEN** literal loopback exposes the matching Go UI HTTPS certificate and current session/CSRF/Host/origin controls, without changing the existing Nginx/public endpoint


#### Scenario: Named Kubernetes public cutover
- **WHEN** the owner-authorized Kubernetes profile serves spire.adianto.dev
- **THEN** existing Nginx validates the exact upstream hostname/chain over a supervised literal-loopback route and UI enforces the exact public Origin/Host
- **AND** browser renewal updates only the owned UI Secret without exporting workload credentials or restarting Money

### Requirement: UI-03 Session and browser request defenses
Demo sessions MUST be opaque in-memory sessions with Secure/HttpOnly/SameSite=Lax cookies, 15-minute idle and one-hour absolute expiry. POSTs MUST require a matching CSRF token and exact Origin/Host, rejecting cross-site fetches and forwarded-host overrides. Responses SHALL use escaped templates, restrictive CSP, frame protection and no-store caching.

#### Scenario: Current session posts same-origin request
- **WHEN** a valid demo session supplies its CSRF token from the exact allowed origin/Host
- **THEN** the requested supported operation can proceed and logout invalidates that session

#### Scenario: Forged browser boundary or expired session
- **WHEN** CSRF is missing/wrong, origin/Host is foreign, fetch is cross-site or the session is expired
- **THEN** the UI rejects before dispatching a caller command, with 403 for the boundary or 401 for missing/expired session

### Requirement: UI-04 Fixed asynchronous attempts and stable retry
Existing POST /api/attempts SHALL accept only trusted/unidentified caller and browser_key [A-Za-z0-9_-]{32,128}, derive IDs from issuer/subject/key and return 202. Pending/done repeats SHALL reuse attempts; retryable failures SHALL retain IDs. These scenarios SHALL be labeled Sync (HTTP/mTLS). Separate Kafka creation SHALL retain the ID derivation/ownership boundary and original deadline; public ownership remains shared.

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
While its run is valid, UI MUST use only its observer SVID for balances/enabled Kafka metadata/receipts, verifying exact Money. Compose triggers remain private UDS; Kubernetes triggers MUST use exact-ID adapters in PC-07 without caller mounts/keys. All scenarios SHALL share 64 attempts, two workers, eight dispatches/minute per principal and 128 sessions; bounded bodies/no unsafe eviction remain. UI MUST receive no Kafka/admin credentials or producer keys.

#### Scenario: Verified balance display and existing attempt lookup
- **WHEN** UI reads Money or an existing pending/done attempt is repeated at history capacity
- **THEN** it returns verified balances or the existing attempt without borrowing caller credentials or creating another dispatch
- **AND** an existing retryable attempt may redispatch with its original ID even at full history capacity, subject to rate/concurrency limits

#### Scenario: Forbidden transfer or resource saturation
- **WHEN** UI's SVID directly posts a Money transfer or new browser work exceeds configured limits
- **THEN** Money returns 403; UI returns 429 for concurrency/rate or 503 for capacity without eviction
- **AND** unavailable Money reads return an infrastructure failure, not fabricated balances

#### Scenario: Kubernetes UI uses its own identity at adapters
- **WHEN** the UI dispatches one fixed scenario through a configured private network adapter
- **THEN** its own observer SVID verifies that exact control adapter, with unchanged fixed API/ownership/quota/retry behavior and no borrowed caller state/key

### Requirement: UI-06 Separate local owner-login preview
The optional local profile SHALL use a private UI Unix socket behind a literal loopback HTTP bridge and a separate owner-only admin Unix socket. Kernel UID checking SHALL issue one-use, one-minute login codes, with origin checks and session rotation. This local login MUST NOT be exposed as the public demo or described as OIDC.

#### Scenario: Local owner logs into preview
- **WHEN** the configured OS owner obtains a code through the private admin socket and submits it once from the loopback origin
- **THEN** a local operator session is issued and the previous session is rotated

#### Scenario: Local login misuse or non-loopback configuration
- **WHEN** a code is invalid/expired/reused, the admin peer is unauthorized or local configuration is non-loopback
- **THEN** login/configuration is rejected without caller dispatch
- **AND** demo mode does not enable the local admin listener

### Requirement: UI-07 Fixed Sync and Async scenario groups
The combined UI SHALL keep existing trusted/unidentified scenarios labeled Sync (HTTP/mTLS) and add only kafka-trusted, kafka-unidentified and kafka-tampered labeled Async (Kafka). POST /api/kafka-attempts MUST accept only scenario/browser_key/current stream_id under existing CSRF/origin/session/operator/body limits. Existing browser/private Sync schemas MUST remain unchanged; arbitrary payloads and destinations are forbidden.

#### Scenario: Existing Sync attempts remain intact
- **WHEN** an operator chooses either existing Sync scenario
- **THEN** the same HTTP/mTLS caller path and background attempt/polling UX execute with their original outcomes

#### Scenario: Fixed Kafka scenario is selected
- **WHEN** the configured combined profile accepts one allowed Async scenario and current stream ID
- **THEN** it creates a bounded background attempt using only that caller's fixed private Kafka control

#### Scenario: Unsupported scenario or stale stream
- **WHEN** input supplies another scenario, extra payload/destination fields, stale stream or a rejected browser boundary
- **THEN** UI rejects before dispatch; a new stream cannot silently retry an old attempt with a refreshed deadline

### Requirement: UI-08 Authenticated correlated Money outcomes
Kafka terminal UI evidence MUST come from exact-Money observer reads matched to retained digests. Non-mesh reads use Money mTLS. Mesh reads use Money's SVID response signature and no application certificate. Broker acknowledgement, logs and claimed IDs MUST NOT imply success. Invalid-JWS rejection stays unattributed. Rejection requires every potentially submitted variant settled with no effect.

#### Scenario: Authenticated successful receipt
- **WHEN** Money returns a matching-instance and digest terminal applied or replayed result for the expected trusted producer, request ID and fixed transfer
- **THEN** the UI shows verified success separately from queued broker or control diagnostics and the outcome-query HTTP status

#### Scenario: Invalid JWS rejection is correlated by bytes
- **WHEN** Money returns a terminal signature or missing-JWS rejection matching the exact submitted record digest
- **THEN** UI shows Money-verified rejection without trusting its claimed producer or request ID; caller diagnostics remain separately labeled

#### Scenario: Earlier variant is unresolved
- **WHEN** a newer variant is rejected but an earlier potentially published variant has missing or expired evidence
- **THEN** the logical attempt stays unknown rather than declaring rejection; every retained digest is queried

#### Scenario: Mesh observer read has no application certificate
- **WHEN** the UI reads a Kafka outcome on the Kubernetes mesh profile
- **THEN** it accepts the result only from Money's SVID signature and does not present an application client certificate

### Requirement: UI-09 Bounded recovery without deadline refresh
Kafka polling SHALL use a 30-second worker budget, bounded one-second/variant polling and existing quotas. UI restart MUST recover initialized caller state and retain browser key/stream, original deadline and all potentially submitted digests. Timeout/404/evidence expiry/reset MUST remain unknown. After deadline or state loss, UI MUST NOT recreate/re-sign a retry or auto-republish under a changed Money instance.

#### Scenario: UI restart after ambiguous submission
- **WHEN** UI restarts or a control response is lost while callers/Money continue
- **THEN** same-key lookup recovers the original descriptor/digests and authenticates Money receipts without a new command deadline

#### Scenario: Evidence timeout or receipt expiry
- **WHEN** the poll budget expires or no retained Money evidence is available
- **THEN** UI preserves the unresolved ID as queued/unknown and permits only safe bounded same-ID recovery, never inferring non-commit

#### Scenario: State loss or Money reset
- **WHEN** initialized sender state is absent/corrupt or the authenticated process-instance nonce changes
- **THEN** UI reports unknown/reset and stops automatic republishing; an explicit new logical attempt is not presented as retry

## Traceability and limitations

See [UI-01–UI-06 sources, checks and evidence](../../../docs/spec-baseline.md#https-demo-ui). Browser keys are deduplication inputs, not authorization credentials. Full restart resets trust/ledger/history; UI-only restart is different. Actual scheduled ACME renewal is still unobserved: issuance, manual renewal-hook continuity and atomic PEM tests are recorded. Local unit fixtures exercise viewer/owner policies and do not prove public human authentication.

The [UI handler](../../../internal/web/app.go) returns HTTP 200 with `status: alive` for GET `/healthz`. This reports application liveness; it does not check Money/SPIRE dependency readiness.

The approved opt-in Kafka extension is mapped in [JS coverage](../../../docs/kafka-signing-acceptance.md). Its full wire/receipt/retry rules are in the [stable profile](../../../docs/kafka-signing-profile.md).
