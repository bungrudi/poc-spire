# Spec Delta: private-caller-controls

## MODIFIED Requirements

### Requirement: PC-03 Caller-owned authentication and no-SVID probe
Non-mesh callers MUST keep direct mTLS transfers with their trusted SVID. Mesh callers MUST sign the Money payload with that SVID and MUST NOT use application mTLS. An unidentified mesh attempt that reaches Money MUST be signature rejection with no ledger effect. Unexpected issuance or HTTP success MUST fail. Kafka controls stay unchanged, and the UI MUST NOT receive producer keys.

#### Scenario: Trusted and unidentified attempts
- **WHEN** each caller executes its fixed attempt on its profile
- **THEN** non-mesh trusted succeeds with verified Money TLS evidence and unidentified ends in TLS rejection with no Money HTTP status, while mesh trusted succeeds by signed payload and mesh unidentified ends in signature rejection with no mutation

#### Scenario: Workload API is broken or unexpectedly issues identity
- **WHEN** the API is unreachable, reports another failure or issues an identity to the unidentified caller
- **THEN** the attempt reports infrastructure failure or `unexpected_identity_issued` rather than claiming no-SVID rejection

#### Scenario: Fixed Kafka publish alongside Sync controls
- **WHEN** the combined profile enables its separate fixed Kafka control route
- **THEN** callers publish only their permitted menu cases, while existing POST `/attempt` behavior remains available

### Requirement: PC-04 Stable retry and distinct outcome provenance
Sync results SHALL retain submitted IDs and distinct Money, TLS, signature, freshness, business and infrastructure outcomes. Mesh signature rejection MUST NOT be labeled as client-certificate TLS failure. Kafka controls SHALL return only bounded publication diagnostics. Lost or malformed responses MUST remain unknown until same-ID recovery. Only authenticated correlated Money receipts SHALL settle Kafka effects.

#### Scenario: Stable successful response
- **WHEN** the caller receives a valid Money result for the submitted request ID
- **THEN** the control response includes that ID, outcome, actual Money status and supported verified-server or result evidence

#### Scenario: Response is lost or cannot be correlated
- **WHEN** control loses a response or receives malformed, oversized or mismatched evidence
- **THEN** it reports `unknown_commit`, preserves the ID and supplies no invented Money success

#### Scenario: Mesh signature rejection is distinct
- **WHEN** Money returns 401 for a missing, tampered or non-verifying payload signature
- **THEN** the caller records signature rejection and does not record a TLS handshake failure or a balance change

#### Scenario: Obsolete mesh signature is distinct
- **WHEN** Money returns 403 `credential_not_current` for an obsolete payload signature
- **THEN** the caller records that freshness result and does not record a bad signature or a TLS handshake failure

### Requirement: PC-07 Exact-identity Kubernetes control adapters
Each Kubernetes caller Pod SHALL have a separately attested control-only adapter. UI MUST verify the adapter by its SVID-signed response, and the adapter MUST authorize only an exact UI signature before forwarding to its own UID-checked UDS. No application mTLS, generic signing API, application-key lending or transfer identity is permitted. Remote credential expiry and unknown-response guards MUST preserve original attempt semantics.

#### Scenario: UI triggers a fixed operation
- **WHEN** UI verifies the selected adapter's signature and submits the unchanged fixed Sync or caller-specific Kafka schema
- **THEN** the adapter verifies the current UI signature, reaches only its own caller as UID 10002 and relays bounded diagnostics without inventing Money effects

#### Scenario: Remote peer or payload is unauthorized
- **WHEN** a request has no valid UI signature or a valid wrong-ID signature, or input adds fields, a destination or signing parameters
- **THEN** a missing or invalid signature is rejected as authentication failure and a valid unauthorized signer receives 403; malformed input returns 400 before UDS execution

#### Scenario: Adapter seeks app or foreign authority
- **WHEN** an adapter requests a transfer SVID, uses its control SVID at Money or targets another caller, state or broker key
- **THEN** issuance, signature authorization and mount boundaries deny that authority; shared Pod mesh identity does not substitute

#### Scenario: Remote response or credential becomes uncertain
- **WHEN** a control response is lost or uncorrelated, or remote credentials cross expiry before forwarding
- **THEN** preserve the original ID and deadline and unknown-result handling without another identity or a fabricated verified Money result

## Traceability

Non-mesh caller proof remains
[PC-01–PC-04](../../../../docs/spec-baseline.md#private-caller-controls).
Mesh signature and adapter scenarios are unrun.
