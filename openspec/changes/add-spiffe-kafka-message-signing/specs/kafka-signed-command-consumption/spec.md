# Spec Delta: Kafka signed command consumption

## Purpose

Add an opt-in Kafka ingestion path inside Money that validates original
workload commands and applies the existing shared in-memory transfer operation.

## ADDED Requirements

### Requirement: KC-01 Embedded opt-in Money ingestion
Kafka ingestion SHALL run inside Money only with complete explicit configuration. Money MUST validate the original producer JWS/SVID itself and use the same ledger instance as POST /transfer and GET /balances. No HTTP forwarding endpoint, separate business ledger or public browser trigger SHALL be introduced. Missing Kafka configuration MUST NOT weaken synchronous mTLS.

#### Scenario: Default synchronous demo
- **WHEN** Money starts without Kafka enabled
- **THEN** existing HTTP routes, callers and assumed-operator browser behavior remain unchanged and no broker connection is started

#### Scenario: Enabled Kafka route
- **WHEN** an authorized signed command is accepted by the configured embedded consumer
- **THEN** the existing fixed transfer changes the same balances/version observed by the authenticated GET /balances route

#### Scenario: Incomplete opt-in configuration
- **WHEN** Kafka is explicitly enabled with missing TLS, topic, group, stream or policy settings
- **THEN** Money startup fails closed rather than silently disabling validation or enabling plaintext broker access

### Requirement: KC-02 Cross-ingress identity-bound idempotency
Synchronous and Kafka transfers MUST share the existing key of authenticated original producer ID plus request_id and canonical business Request fields. Signature, certificate, envelope time, broker identity and offset MUST NOT create a new ledger namespace. Every Kafka replay MUST pass current message validation before history lookup.

#### Scenario: Sync then Kafka or Kafka then sync
- **WHEN** the same trusted identity/request_id/business content is submitted across both ingresses, including concurrently
- **THEN** one debit/version increment occurs during the Money lifetime and the other submission returns the original result as replay

#### Scenario: Signed changed business content
- **WHEN** a current authorized signature reuses an existing key with different transfer fields
- **THEN** the existing conflict result is returned without another mutation

#### Scenario: Expired envelope for a known request
- **WHEN** an invalid/expired signed delivery names an already committed request_id
- **THEN** it is rejected before retrieving history and does not gain access merely because the ID exists

### Requirement: KC-03 Stable Kafka context binding
Within the Money lifetime, the first valid Kafka terminal ledger decision for a producer/request_id MUST bind its immutable context, including original creation/deadline. Subsequent Kafka deliveries MUST match that context while allowing new envelope times/certificates. Context binding and ledger decision MUST be atomic; transient failures MUST NOT consume a binding or evict history.

#### Scenario: Valid re-signing under the original context
- **WHEN** the producer re-signs the same command after rotation with only envelope time/certificate/signature changed
- **THEN** Money replays the existing business result without a second debit

#### Scenario: Retry changes creation or deadline
- **WHEN** a newly valid signature reuses a Kafka-bound key with changed immutable context
- **THEN** Money reports a context conflict without replacing the binding or modifying balances

#### Scenario: First Kafka copy follows a sync result
- **WHEN** a request previously committed synchronously receives its first valid same-business Kafka copy
- **THEN** Money binds that Kafka context and returns the existing ledger result without another debit

### Requirement: KC-04 Acknowledge only known terminal dispositions
One Money instance/member MUST disable auto-commit and process one partition/record at a time. Only known terminal decisions SHALL advance its next offset under current ownership. Pauses MUST preserve heartbeats; revocation MUST fence processing/commits. Structure/crypto/identity/context/freshness failures MUST NOT enter ledger history; authorized deterministic business rejections SHALL retain existing recorded-rejection behavior.

#### Scenario: Mutation and acknowledgement ordering
- **WHEN** a valid command commits its shared ledger result
- **THEN** only afterward is its next Kafka offset acknowledged; a pause before the ledger decision cannot advance the offset

#### Scenario: Malformed or unauthorized poison record
- **WHEN** bounded verification establishes a deterministic invalid command or unauthorized signer
- **THEN** Money records a sanitized terminal rejection and acknowledges that record without balances/history mutation

### Requirement: KC-05 Preserve unknown outcomes and transient records
Capacity, unavailable independent trust material, broker/transport faults and unclassifiable outcomes MUST pause consumption without committing undecided offsets. Rejection against available accepted trust MUST be terminal. Retries MUST preserve the record/ID; uncertain commit acknowledgement MUST block later records and retry the known disposition under current assignment, never inventing success or a new transfer ID.

#### Scenario: Commit succeeded but acknowledgement is uncertain
- **WHEN** the ledger decision is known but Kafka offset acknowledgement is lost
- **THEN** no later record is acknowledged first, and any redelivery during the same Money lifetime is safely deduplicated after current verification

#### Scenario: Bundle outage or capacity saturation
- **WHEN** no accepted independent trust snapshot is available or the shared ledger rejects a new key at capacity
- **THEN** no mutation/offset advancement occurs and processing reports transient infrastructure/capacity failure

### Requirement: KC-06 Honest in-memory restart scope
Money SHALL retain one in-memory ledger, retry history and Kafka context bindings only during its process lifetime. Restart resets them together; broker offsets MUST NOT be described as restoring business state. A still-valid resent/unacknowledged command can be a new operation after restart. Durable replay protection, crash-safe effects and atomic offset-plus-ledger storage SHALL NOT be claimed.

#### Scenario: Crash before or after ledger commit
- **WHEN** Money crashes before commit, or after commit but before offset acknowledgement, and restarts
- **THEN** balances/history reset as today; retained offsets govern delivery only, and a valid redelivery can apply in the new lifetime

#### Scenario: Clean fixture reset
- **WHEN** the isolated fixture is reset with a fresh stream/topic/trust
- **THEN** old-stream records are rejected by configured context, while this does not establish durable protection when an old stream is intentionally reused

### Requirement: KC-07 Separate stock broker admission and protected fixtures
The fixture SHALL use stock Kafka TLS client-certificate admission, hostname checks and default-deny exact DN/topic/group ACLs independent of SPIFFE signer authorization. Broker ports/admin keys SHALL NOT be exposed to applications or the host. Each app SHALL receive only its own temporary broker credential; all fixture credentials/records SHALL be cleaned without touching retained/live state.

#### Scenario: Broker principal cannot replace signer
- **WHEN** a broker-admitted client publishes a forged or wrong-ID signed command
- **THEN** Money still validates and rejects the original JWS identity; no DN-to-SPIFFE inference is made

#### Scenario: Boundaries and cleanup
- **WHEN** the isolated fixture completes or fails
- **THEN** its ports/mounts/ACLs and cleanup are verified, secrets/JWS chains are excluded from evidence and retained/live project state is unchanged

## Traceability and proposed coverage

[Normative profile](../../profile.md) and [JS-05–JS-15 cases](../../acceptance.md)
define outcomes and limits. Baseline FL/T13–T16,T22–T23 supplies existing ledger
and failure evidence only; embedded Kafka, cross-ingress and crash cases are
proposed and unrun. Current application source is unchanged by this draft.
