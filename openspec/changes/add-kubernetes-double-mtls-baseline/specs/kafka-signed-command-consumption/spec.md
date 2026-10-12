# Spec Delta: Kafka signed command consumption

## ADDED Requirements

### Requirement: KC-11 Kubernetes broker and ingestion parity
The Kubernetes profile SHALL place its stock broker, scoped bootstrap and existing embedded Money ingestion in-cluster with one broker/partition/member. Broker TLS/hostname/DN ACLs and original-producer JWS MUST remain independent of mesh admission. UI/adapters MUST receive no broker/admin credentials. Shared ledger, context, receipts, ordered acknowledgement and memory-only reset behavior MUST preserve the existing profile.

#### Scenario: Kubernetes fixed Kafka cases
- **WHEN** trusted, unidentified/unsigned and tampered records traverse the double-TLS broker path
- **THEN** Money independently validates original bytes/SVID/context and reports authenticated exact-record receipts with the same success/rejection/unknown semantics

#### Scenario: Broker or consumer resets
- **WHEN** a process restarts or destructive broker/Pod state loss occurs
- **THEN** offsets never restore Money state, valid pending records can apply in a new Money lifetime and destructive loss follows the explicit owned run-reset contract without automatic republication

#### Scenario: Broker admission or isolation fails
- **WHEN** a wrong DN/topic/group, hostname or forbidden UI path targets the broker
- **THEN** admission/mesh/network denial is measured separately from JWS rejection and no inaccessible poison record counts as original-signer proof

## Traceability and proposed coverage

All new scenarios are NOT RUN. Existing KC-01–KC-10/JS-01–JS-23 and
[Compose proof](../../../../../docs/kafka-signing-acceptance.md) remain intact
compatibility references. New [KB acceptance](../../acceptance.md) and
[profile](../../profile.md) require real Kubernetes broker/application/mesh proof.
