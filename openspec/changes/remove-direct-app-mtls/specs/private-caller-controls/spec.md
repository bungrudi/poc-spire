# Spec Delta

## MODIFIED Requirements

### Requirement: PC-03 Caller-owned authentication and no-SVID probe
All trusted callers MUST sign the Money payload with their own current SVID and verify Money's correlated signed response. Application mTLS MUST NOT be supported. An unidentified attempt that reaches Money MUST be signature rejection with no ledger effect. Unexpected issuance or HTTP success MUST fail. Kafka controls stay unchanged, and the UI MUST NOT receive producer keys.

#### Scenario: Trusted and unidentified attempts
- **WHEN** each caller executes its fixed attempt on its profile
- **THEN** trusted succeeds by signed payload with verified Money response evidence, and unidentified ends in HTTP 401 signature rejection with no mutation in both Compose and Kubernetes

#### Scenario: Workload API is broken or unexpectedly issues identity
- **WHEN** the API is unreachable, reports another failure or issues an identity to the unidentified caller
- **THEN** the attempt reports infrastructure failure or `unexpected_identity_issued` rather than claiming no-SVID rejection

#### Scenario: Fixed Kafka publish alongside Sync controls
- **WHEN** the combined profile enables its separate fixed Kafka control route
- **THEN** callers publish only their permitted menu cases, while existing POST `/attempt` behavior remains available

### Requirement: PC-04 Stable retry and distinct outcome provenance
Sync results SHALL retain submitted IDs and distinct Money, outer-transport, signature, freshness, business and infrastructure outcomes. Signature rejection MUST NOT be labeled as client-certificate TLS failure in any profile. Kafka controls SHALL return only bounded publication diagnostics. Lost or malformed responses MUST remain unknown until same-ID recovery. Only authenticated correlated Money receipts SHALL settle Kafka effects.

#### Scenario: Stable successful response
- **WHEN** the caller receives a valid Money result for the submitted request ID
- **THEN** the control response includes that ID, outcome, actual Money status and authenticated Money response evidence correlated by exact signer, method, path and request ID

#### Scenario: Response is lost or cannot be correlated
- **WHEN** control loses a response or receives malformed, oversized or mismatched evidence
- **THEN** it reports `unknown_commit`, preserves the ID and supplies no invented Money success

#### Scenario: Mesh signature rejection is distinct
- **WHEN** Money returns 401 for a missing, tampered or non-verifying payload signature
- **THEN** the caller records signature rejection and does not record a TLS handshake failure or a balance change

#### Scenario: Obsolete mesh signature is distinct
- **WHEN** Money returns 403 `credential_not_current` for an obsolete payload signature
- **THEN** the caller records that freshness result and does not record a bad signature or a TLS handshake failure

## Traceability

See [caller baseline](../../../../../docs/spec-baseline.md#private-caller-controls), `internal/caller/caller.go`, `internal/controladapter` and S01/S02/S05/S08/S10 in [tasks](../../tasks.md). PC-01/PC-02/PC-05–PC-07 remain in force; historical TLS outcomes are not evidence for these new outcomes.
