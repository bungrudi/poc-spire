# Spec Delta: identity-certificate-lifecycle

## MODIFIED Requirements

### Requirement: IC-01 Rotating sources and fresh workload connections
Eligible Money, trusted caller and UI workloads SHALL obtain SVIDs and trust through updating X.509-SVID sources and close sources at shutdown. Existing internal HTTP operations MUST use fresh TLS handshakes without reuse, resumption or HTTP/2. Opt-in Kafka uses separate broker TLS connections and validates the original command SVID per delivery. Credential rotation MUST NOT enable unauthenticated fallback; expired credentials MUST NOT transfer.

#### Scenario: Source rotates during operation
- **WHEN** SPIRE renews an eligible workload's SVID
- **THEN** new operations use the current credential and verify Money without resetting the ledger

#### Scenario: Frozen expired credential or reuse attempt
- **WHEN** an existing internal HTTP client attempts an operation with a frozen expired credential or tries to reuse/resume its connection
- **THEN** an expired credential cannot transfer and supported HTTP requests use fresh nonresumed TLS handshakes

#### Scenario: Kafka connection survives signer rotation
- **WHEN** a broker connection remains open while the producer SVID rotates
- **THEN** each command still requires its own current-validity SVID/JWS verification; broker admission cannot authorize an expired original signer

## Traceability and proposed coverage

All original scenarios are retained. Existing named checks/evidence are mapped in
[the baseline index](../../../../../docs/spec-baseline.md). New cross-ingress,
Kafka and lifecycle assertions are proposed in [acceptance.md](../../acceptance.md),
not executed passes. The main baseline spec is unchanged by this draft.
