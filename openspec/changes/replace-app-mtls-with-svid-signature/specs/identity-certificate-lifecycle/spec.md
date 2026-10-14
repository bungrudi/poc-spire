# Spec Delta: identity-certificate-lifecycle

## MODIFIED Requirements

### Requirement: IC-01 Rotating sources and fresh workload connections
Eligible Money, trusted caller and UI workloads SHALL obtain SVIDs through updating X.509-SVID sources and close those sources at shutdown. Non-mesh HTTP MUST use fresh TLS handshakes without reuse, resumption or HTTP/2. Mesh HTTP MUST sign each payload with the current SVID and MUST NOT open application TLS. Kafka keeps broker TLS and per-delivery JWS. Expired credentials MUST NOT transfer.

#### Scenario: Source rotates during operation
- **WHEN** SPIRE renews an eligible workload's SVID
- **THEN** new operations use the current credential and verify Money without resetting the ledger

#### Scenario: Frozen expired credential or reuse attempt
- **WHEN** a non-mesh client uses a frozen expired credential or tries to reuse its TLS connection, or a mesh sender signs with an expired SVID
- **THEN** the expired credential cannot transfer; non-mesh HTTP still uses a fresh nonresumed TLS handshake

#### Scenario: Kafka connection survives signer rotation
- **WHEN** a broker connection remains open while the producer SVID rotates
- **THEN** each command still requires its own current-validity SVID and JWS verification; broker admission cannot authorize an expired original signer

#### Scenario: Mesh payload uses the renewed SVID
- **WHEN** a mesh caller signs a new or retried payload after rotation
- **THEN** the new signature uses the renewed SVID, a previous signature still inside its certificate lifetime and envelope remains verifiable, and the caller does not resume an application TLS session

## Traceability

Recorded rotation proof is the current TLS profile in
[identity lifecycle](../../../../docs/spec-baseline.md#identity-certificate-lifecycle).
The mesh signing scenario is unrun.
