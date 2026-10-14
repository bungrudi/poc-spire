# Spec Delta

## Purpose

Authenticate Kubernetes application HTTP by an X.509-SVID signature on the
payload. Istio/Envoy remains the only TLS layer on that path.

## ADDED Requirements

### Requirement: SP-01 SVID-signed application payload
On the Kubernetes mesh profile, an identity-bearing application HTTP payload SHALL be one compact ES256 JWS signed by the sender's current X.509-SVID. Peers MUST NOT authenticate that payload with an application certificate.

#### Scenario: Trusted caller signs a transfer
- **WHEN** the trusted caller submits its fixed transfer on the mesh profile
- **THEN** the payload signature is produced by its current SVID and Money does not require a client certificate

#### Scenario: Unsigned or confused signature
- **WHEN** the payload has no signature, uses `none` or another algorithm, or substitutes an unsigned identity header
- **THEN** Money returns 401 and does not read history or change balances

### Requirement: SP-02 Exact signer authorization
The verifier MUST trust only the independent `poc.internal` bundle, derive the signer from the SVID URI SAN, and authorize that exact ID. Mesh identity and caller-supplied roots MUST NOT grant the effect.

#### Scenario: Permitted trusted signer
- **WHEN** the current trusted-caller SVID signs a transfer and the chain validates
- **THEN** Money authorizes that exact ID for the transfer route

#### Scenario: Valid unauthorized signer
- **WHEN** the UI or another trusted-domain ID produces a valid transfer signature
- **THEN** Money returns 403 and does not change balances

#### Scenario: Forged chain
- **WHEN** a signer presents an untrusted chain that claims the trusted URI and supplies its own root
- **THEN** Money rejects trust and returns 401 without a ledger effect

### Requirement: SP-03 Signed destination binding
The signed payload MUST bind the HTTP method, path and logical request ID. A valid signature on a different method, path or ID MUST be rejected without a ledger effect.

#### Scenario: Bound transfer is accepted
- **WHEN** the signature covers POST `/transfer` and the submitted request ID
- **THEN** destination checks proceed to exact-ID authorization

#### Scenario: Signature is replayed onto another route
- **WHEN** a transfer signature is presented as a balance read or with a different request ID
- **THEN** Money rejects it before history access or mutation

### Requirement: SP-04 Money signs its response
Money's identity-bearing HTTP response MUST be signed by its current SVID. The client MUST accept that response only for signer `spiffe://poc.internal/money`.

#### Scenario: Exact Money response
- **WHEN** Money signs the response for the submitted request ID with its current SVID
- **THEN** the client may treat that response as Money's result

#### Scenario: Impostor response
- **WHEN** another trusted-domain identity signs the response
- **THEN** the client rejects it and does not report a successful transfer

### Requirement: SP-05 Distinct mesh and signature failures
Mesh denial, signature authentication failure, authorized-identity denial and infrastructure failure MUST remain distinct. A mesh-admitted caller with no SVID MUST NOT be reported as a client-certificate TLS failure.

#### Scenario: No SVID after mesh admission
- **WHEN** the unidentified caller reaches Money through outer mesh mTLS and presents no valid signature
- **THEN** Money returns 401 with no HTTP-less TLS classification and no ledger effect

#### Scenario: Outer mesh denies the pod
- **WHEN** Istio/Envoy rejects the connection before the application receives it
- **THEN** Money produces no HTTP status and no ledger effect

### Requirement: SP-06 Current signer and stable retry
Each new payload MUST be signed with the sender's current SVID. Retry of an unknown result MUST keep the original logical request ID and MUST NOT allocate a fresh transfer ID.

#### Scenario: Credential expires before commit
- **WHEN** the signing SVID expires after parsing and before ledger mutation
- **THEN** Money returns 403 `credential_not_current` without a ledger effect

#### Scenario: Unknown result is retried
- **WHEN** the caller retries an unknown transfer result
- **THEN** it re-signs the same logical request ID with its current SVID and Money deduplicates that ID

## Traceability

No signed-payload-profile evidence exists yet. Current proof remains the
double-mTLS rows in
[money transport](../../../../docs/spec-baseline.md#money-transport-and-authorization)
and
[mesh transport](../../../../docs/spec-baseline.md#mesh-transport-security).
Those passes do not satisfy SP-01–SP-06.
