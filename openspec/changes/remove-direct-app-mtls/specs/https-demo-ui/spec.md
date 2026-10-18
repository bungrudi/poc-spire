# Spec Delta

## MODIFIED Requirements

### Requirement: UI-04 Fixed asynchronous attempts and stable retry
Existing POST /api/attempts SHALL accept only trusted/unidentified caller and browser_key [A-Za-z0-9_-]{32,128}, derive IDs from issuer/subject/key and return 202. Pending/done repeats SHALL reuse attempts; retryable failures SHALL retain IDs. These scenarios SHALL be labeled Sync (signed HTTP). Separate Kafka creation SHALL retain the ID derivation/ownership boundary and original deadline; public ownership remains shared.

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

### Requirement: UI-07 Fixed Sync and Async scenario groups
The combined UI SHALL keep existing trusted/unidentified scenarios labeled Sync (signed HTTP) and add only kafka-trusted, kafka-unidentified and kafka-tampered labeled Async (Kafka). POST /api/kafka-attempts MUST accept only scenario/browser_key/current stream_id under existing CSRF/origin/session/operator/body limits. Existing browser/private Sync schemas MUST remain unchanged; arbitrary payloads and destinations are forbidden.

#### Scenario: Existing Sync attempts remain intact
- **WHEN** an operator chooses either existing Sync scenario
- **THEN** the signed HTTP caller path and existing background attempt/polling UX execute; trusted succeeds and unidentified reports signature rejection without mutation

#### Scenario: Fixed Kafka scenario is selected
- **WHEN** the configured combined profile accepts one allowed Async scenario and current stream ID
- **THEN** it creates a bounded background attempt using only that caller's fixed private Kafka control

#### Scenario: Unsupported scenario or stale stream
- **WHEN** input supplies another scenario, extra payload/destination fields, stale stream or a rejected browser boundary
- **THEN** UI rejects before dispatch; a new stream cannot silently retry an old attempt with a refreshed deadline

### Requirement: UI-08 Authenticated correlated Money outcomes
Kafka terminal UI evidence MUST come from exact-Money observer reads matched to retained digests. All reads use signed HTTP and verify Money's SVID response signature with method/path/request-ID correlation; no application TLS certificate substitutes for that signature. Broker acknowledgement, logs and claimed IDs MUST NOT imply success. Invalid-JWS rejection stays unattributed. Rejection requires every potentially submitted variant settled with no effect.

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

#### Scenario: Compose observer authenticates its reads
- **WHEN** the Compose UI reads balances or an enabled Kafka outcome
- **THEN** it signs the request with its observer SVID and accepts only Money's current, trusted and correlated response signature; missing, tampered or wrong-ID responses remain unverified

## Traceability

See [UI baseline](../../../../../docs/spec-baseline.md#https-demo-ui), `cmd/ui`, `internal/moneyreader`, `internal/web` and S05/S08/S10 in [tasks](../../tasks.md). Browser HTTPS/session/CSRF behavior remains specified by UI-01–UI-03. New-profile browser acceptance is unrun.
