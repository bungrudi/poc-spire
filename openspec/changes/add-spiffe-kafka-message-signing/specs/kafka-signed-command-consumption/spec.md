# Spec Delta: Kafka signed command consumption

## Purpose

Add an opt-in Kafka ingestion path inside Money that validates original
workload commands and applies the existing shared in-memory transfer operation.

## ADDED Requirements

### Requirement: KC-01 Embedded opt-in Money ingestion
Kafka ingestion SHALL run inside Money only with complete explicit configuration. Money MUST validate the original producer JWS/SVID itself and use the same ledger as POST /transfer and GET /balances. The opt-in combined UI SHALL add only its fixed Kafka scenarios and observer outcome reads. There SHALL be no Kafka-to-HTTP transfer forwarding or separate ledger; missing Kafka settings MUST NOT weaken synchronous mTLS.

#### Scenario: Default synchronous demo
- **WHEN** Money starts without Kafka enabled
- **THEN** existing transfer/balance routes, Sync callers and assumed-operator browser behavior remain unchanged; no broker connection or Kafka menu is started

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
Capacity, unavailable independent trust, snapshot-budget timeout, assignment loss and unclassifiable outcomes MUST pause without committing undecided offsets or creating terminal rejection receipts. Expired credentials/messages and rejection against available accepted trust MUST be terminal. Retries MUST preserve record/ID; uncertain commit acknowledgement MUST block later records and retry the known disposition under current assignment.

#### Scenario: Commit succeeded but acknowledgement is uncertain
- **WHEN** the ledger decision is known but Kafka offset acknowledgement is lost
- **THEN** no later record is acknowledged first, and any redelivery during the same Money lifetime is safely deduplicated after current verification

#### Scenario: Bundle outage or capacity saturation
- **WHEN** no accepted independent trust snapshot is available or the shared ledger rejects a new key at capacity
- **THEN** no mutation/offset advancement occurs and processing reports transient infrastructure/capacity failure

### Requirement: KC-06 Honest in-memory restart scope
Money SHALL retain one in-memory ledger, retry history, context bindings and outcome receipts only during its lifetime. Restart resets them; broker offsets MUST NOT be described as restoring business state. Caller descriptor files surviving a container/UI restart MUST NOT be described as durable Money recovery. Still-valid unacknowledged commands can apply after Money restart; durable replay protection or atomic offset-plus-ledger storage SHALL NOT be claimed.

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

### Requirement: KC-08 Observer-authorized record evidence
With Kafka enabled, Money SHALL serve bounded GET /kafka/outcomes metadata and GET /kafka/outcomes/{digest} receipts only to its exact observer SVID over mandatory direct mTLS. Digest MUST cover complete record bytes/fields, including invalid JWS, not claimed producer/request IDs. Invalid signatures SHALL have no authenticated producer assertion. Query HTTP status MUST remain distinct from Kafka/business disposition.

#### Scenario: Observer queries a known invalid record
- **WHEN** the observer requests a digest whose exact record Money rejected without valid JWS
- **THEN** the authenticated receipt proves that record's terminal rejection without trusting claimed identity or request ID

#### Scenario: Unavailable or expired evidence
- **WHEN** no retained receipt exists or Money has reset
- **THEN** bounded metadata/404 includes the current process instance but does not prove non-commit; wrong valid readers receive 403 and no-certificate readers fail TLS

### Requirement: KC-09 Atomic bounded immutable receipts
Money MUST reserve receipt capacity before Kafka effects and atomically expose each ledger/context/terminal business receipt decision. Receipt capacity SHALL be 4096 with 180-second retention and no fresh-entry eviction. Retained rejected digests MUST NOT later cause effects; cached receipts MUST NOT authorize signature/history access. Receipt expiry does not evict ledger deduplication. Transient failures MUST NOT create terminal receipts.

#### Scenario: Receipt capacity is full
- **WHEN** no receipt slot can be reserved without evicting a retained entry
- **THEN** Kafka pauses before business effects/offset acknowledgement; Sync remains independent of receipt capacity

#### Scenario: Duplicate rejected bytes later validate
- **WHEN** a previously rejected exact digest is redelivered while its rejection receipt is retained, after time or trust changed
- **THEN** it is terminally discarded without business effects and the original receipt stays coherent; no receipt becomes signer authorization

#### Scenario: Budget or assignment guard fails
- **WHEN** the bounded validation snapshot times out or assignment is lost before effects
- **THEN** no history/effect/terminal rejection receipt is created; certificate/message expiry is separately terminal

### Requirement: KC-10 Isolated combined Compose organization
The iteration SHALL define infra/kafka-signing/ for stock single-node KRaft Compose/config/helpers/README, reusing hardened Go app images. The opt-in UI/Money/callers/broker stack MUST use separate project/network/runtime/lock/descriptor paths and native UID-1000 SPIRE. Only its loopback browser port SHALL publish; UI MUST receive no broker/admin credentials or network. Cleanup MUST leave deployed and retained state untouched.

#### Scenario: Combined stack starts beside existing resources
- **WHEN** the isolated combined profile initializes its own validated ephemeral runtime and broker PKI/topic/ACL readiness
- **THEN** existing demo processes, ports, native agent state and retained lock are not stopped, reused or bypassed

#### Scenario: Startup failure or complete shutdown
- **WHEN** readiness fails or the combined run is stopped
- **THEN** only owned temporary containers/volumes/state/PKI/child processes are removed; the current public deployment is unchanged

## Traceability and proposed coverage

[Normative profile](../../profile.md) and [JS-05–JS-15 cases](../../acceptance.md)
define outcomes and limits. Baseline FL/T13–T16,T22–T23 supplies existing ledger
and failure evidence only; embedded Kafka, cross-ingress and crash cases are
proposed and unrun. Current application source is unchanged by this draft.
