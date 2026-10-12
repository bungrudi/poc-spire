# Spec Delta: HTTPS demo UI

## MODIFIED Requirements

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
UI MUST read balances and enabled Kafka metadata/receipts only with its own observer SVID, verifying exact Money identity, and trigger callers only over private sockets. All scenarios SHALL share 64 attempts, two active workers, eight dispatches/minute per principal and 128 sessions; bounded bodies and no unsafe history eviction remain. UI MUST receive no Kafka/admin credentials or producer private keys.

#### Scenario: Verified balance display and existing attempt lookup
- **WHEN** UI reads Money or an existing pending/done attempt is repeated at history capacity
- **THEN** it returns verified balances or the existing attempt without borrowing caller credentials or creating another dispatch
- **AND** an existing retryable attempt may redispatch with its original ID even at full history capacity, subject to rate/concurrency limits

#### Scenario: Forbidden transfer or resource saturation
- **WHEN** UI's SVID directly posts a Money transfer or new browser work exceeds configured limits
- **THEN** Money returns 403; UI returns 429 for concurrency/rate or 503 for capacity without eviction
- **AND** unavailable Money reads return an infrastructure failure, not fabricated balances

## ADDED Requirements

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
Kafka terminal UI evidence MUST come from exact-Money mTLS observer reads matched to retained complete-record digests. Broker ack, balances, logs and claimed producer/request IDs MUST NOT imply success. Invalid-JWS rejection SHALL remain unattributed to a producer. Any applied/replayed authenticated expected result can settle success; rejection requires every potentially submitted variant settled with no effect.

#### Scenario: Authenticated successful receipt
- **WHEN** Money returns a matching-instance/digest terminal applied/replayed result for the expected trusted producer, request ID and fixed transfer
- **THEN** the UI shows verified success separately from queued broker/control diagnostics and the outcome-query HTTP status

#### Scenario: Invalid JWS rejection is correlated by bytes
- **WHEN** Money returns a terminal signature/missing-JWS rejection matching the exact submitted record digest
- **THEN** UI shows Money-verified rejection without trusting its claimed producer or request ID; caller diagnostics remain separately labeled

#### Scenario: Earlier variant is unresolved
- **WHEN** a newer variant is rejected but an earlier potentially published variant has missing/expired evidence
- **THEN** the logical attempt stays unknown rather than declaring rejection; every retained digest is queried

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

## Traceability and proposed coverage

[Profile](../../profile.md) defines fixed routes, receipt digest, retention and
Compose lifecycle. [Acceptance](../../acceptance.md) contains proposed unrun
UI/control/outcome cases. Existing UI-01–UI-03/UI-06 security contracts and all
original UI-04/UI-05 scenarios remain, with source/tests mapped in
[the baseline index](../../../../../docs/spec-baseline.md).
