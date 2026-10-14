# Spec Delta: https-demo-ui

## MODIFIED Requirements

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

## Traceability

Recorded UI outcome proof is the current mTLS observer path in
[the HTTPS demo](../../../../docs/spec-baseline.md#https-demo-ui).
The mesh signature scenario is unrun.
