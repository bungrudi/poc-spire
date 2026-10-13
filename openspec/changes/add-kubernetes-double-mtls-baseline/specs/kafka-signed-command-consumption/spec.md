# Spec Delta: Kafka signed command consumption

## ADDED Requirements

### Requirement: KC-11 Kubernetes broker and ingestion parity
Kubernetes SHALL run its stock broker/bootstrap and embedded Money ingestion in-cluster with one broker/partition/member. Broker TLS/hostname/DN ACLs and original-producer JWS MUST remain independent of mesh admission; UI/adapters MUST receive no broker/admin credentials. Shared ledger/context/receipts/ordered acknowledgement semantics remain. Exhaustive crash/ack/rebalance re-proving is DEFERRED; failed/corrupt runs use operator whole-cluster stop/recreate.

#### Scenario: Kubernetes fixed Kafka cases
- **WHEN** trusted, unidentified/unsigned and tampered records traverse the double-TLS broker path
- **THEN** Money independently validates original bytes/SVID/context and reports authenticated exact-record receipts with the same success/rejection/unknown semantics

#### Scenario: Broker or consumer resets
- **WHEN** a process restarts or destructive broker/Pod state loss occurs
- **THEN** no durable Money state or recovery is claimed; the operator stops and recreates a failed/corrupt whole run without automatic republication, and existing Compose reset contracts remain unchanged

#### Scenario: Broker admission or isolation fails
- **WHEN** a wrong DN/topic/group, hostname or forbidden UI path targets the broker
- **THEN** admission/mesh/network denial is measured separately from JWS rejection and no inaccessible poison record counts as original-signer proof

## Traceability and proposed coverage

Existing KC-01–KC-10/JS-01–JS-23 and [Compose proof](../../../../../docs/kafka-signing-acceptance.md)
remain unchanged. [Review](../../review.md), [acceptance](../../acceptance.md) and
[profile](../../profile.md) define narrowed core Kubernetes proof; extended
reliability re-proving is DEFERRED, not PASS. Final core acceptance is pending.
