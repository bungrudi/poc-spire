# Spec Delta: fake-ledger-idempotency

## MODIFIED Requirements

### Requirement: FL-02 Identity-bound stable replay
After ingress authentication/validity and Kafka immutable-context checks, idempotency MUST use the shared original producer ID and request_id ([A-Za-z0-9_-], length 1–128). Matching canonical business fields SHALL replay the original status/result; differing fields MUST return 409. Kafka-only context metadata MUST NOT change a valid synchronous replay. Signature/certificate/envelope time/offset MUST NOT create another key; unknown outcomes MUST retain the ID.

#### Scenario: Concurrent duplicates or lost response
- **WHEN** the same identity and request ID repeat the same transfer, including concurrently or after response loss
- **THEN** exactly one ledger mutation occurs within the Money process lifetime and retries recover the stable result

#### Scenario: Conflicting payload or invalid request ID
- **WHEN** an existing key is reused for different fields or a request ID is malformed
- **THEN** Money returns 409 for conflict or 400 for an invalid ID without another mutation

#### Scenario: Distinct verified identities use one request ID
- **WHEN** ledger-level requests use the same request ID under different caller IDs
- **THEN** their records remain separate; Money's route policy still decides which identities can access the ledger

#### Scenario: Same operation across sync and Kafka
- **WHEN** current valid submissions use the same authenticated producer ID, request_id and business fields across either ingress, including concurrently
- **THEN** one mutation occurs in the Money lifetime and the other retrieves its original result as replay

### Requirement: FL-04 Process-lifetime accounting and reset
Money SHALL keep one shared sync/Kafka ledger, retry history and Kafka context bindings in memory only. A Money restart MUST reset them to initial state together. A UI-only restart MUST NOT reset Money. Kafka offsets are delivery progress, not ledger recovery; the demo MUST NOT claim durable exactly-once processing across Money/full-demo restart.

#### Scenario: UI restarts while Money continues
- **WHEN** the browser renews its session and repeats a logical request after a UI-only restart
- **THEN** the same Money request ID replays without a second debit

#### Scenario: Money process restarts
- **WHEN** Money restarts and a previous request ID is submitted again
- **THEN** balances/version start fresh and that ID is a new operation in the new process lifetime

#### Scenario: Kafka redelivery after Money restart
- **WHEN** a still-valid unacknowledged or resent signed operation arrives after Money restarted with the same stream configured
- **THEN** its previous ledger/history no longer exists and it can be a new operation in the new Money lifetime, regardless of retained broker offsets

## Traceability and proposed coverage

All original scenarios are retained. Existing named checks/evidence are mapped in
[the baseline index](../../../../../docs/spec-baseline.md). New cross-ingress,
Kafka and lifecycle assertions are proposed in [acceptance.md](../../acceptance.md),
not executed passes. The main baseline spec is unchanged by this draft.
