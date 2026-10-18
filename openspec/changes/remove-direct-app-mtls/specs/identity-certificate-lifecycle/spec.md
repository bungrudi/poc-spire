# Spec Delta

## MODIFIED Requirements

### Requirement: IC-01 Rotating sources and fresh workload connections
Eligible Money, trusted caller, UI and control-adapter workloads SHALL obtain SVIDs through updating X.509-SVID sources and close those sources at shutdown. All workload HTTP MUST sign each payload with the current SVID and MUST NOT open application TLS or depend on TLS connection freshness. Kafka keeps broker TLS and per-delivery JWS. Expired credentials MUST NOT transfer.

#### Scenario: Source rotates during operation
- **WHEN** SPIRE renews an eligible workload's SVID
- **THEN** new operations use the current credential and verify Money without resetting the ledger

#### Scenario: Frozen expired credential or reuse attempt
- **WHEN** a sender uses a frozen expired signing SVID or attempts to rely on an already-open connection instead of a current valid signature
- **THEN** the expired credential cannot transfer, and connection reuse never bypasses per-message signature and freshness verification

#### Scenario: Kafka connection survives signer rotation
- **WHEN** a broker connection remains open while the producer SVID rotates
- **THEN** each command still requires its own current-validity SVID and JWS verification; broker admission cannot authorize an expired original signer

#### Scenario: Mesh payload uses the renewed SVID
- **WHEN** a caller in either deployment signs a new or retried payload after rotation
- **THEN** the new signature uses the renewed SVID, a previous signature still inside its certificate lifetime and envelope remains verifiable, and no application TLS session exists

### Requirement: IC-02 Validated atomic public trust bundle
The unidentified caller SHALL obtain no app SVID/key and load authentic public SPIRE trust per probe. Publication MUST validate complete CA PEM, set public-read permissions before atomic replacement and preserve valid data on failed refresh. Kubernetes SHALL use a restricted publisher and read-only whole-directory ConfigMap projection, not subPath. Missing/wrong/unusable trust is infrastructure failure; valid cached trust remains usable within current credential bounds.

#### Scenario: CA rollover and bundle refresh
- **WHEN** a valid changed bundle is fetched from the private SPIRE administration path
- **THEN** complete public trust is refreshed and the unidentified caller verifies Money's signed rejection using the public bundle without obtaining its own SVID or private key

#### Scenario: Malformed refresh or missing trust material
- **WHEN** refresh times out or returns malformed PEM
- **THEN** the last validated published bundle remains intact
- **AND** missing or unrelated client trust causes a failed request classified as infrastructure/trust failure, not identity-denial proof

#### Scenario: Kubernetes public trust refresh without credentials
- **WHEN** the restricted publisher fetches a valid changed public bundle through owned SPIRE admin authority
- **THEN** atomic read-only projection updates the caller-configured bundle path without any private key/SVID/API credential, and the probe independently verifies exact Money

#### Scenario: Issuer outage with valid cached trust
- **WHEN** the issuer is unavailable while cached SVIDs/accepted trust remain valid
- **THEN** their existing operations can continue until actual validity/guard boundaries; availability alone is not identity invalidation or claimed immediate revocation

## Traceability

See [lifecycle baseline](../../../../../docs/spec-baseline.md#identity-and-certificate-lifecycle), `internal/identity`, `internal/httpjws`, the bundle publisher and S04/S06/S07/S09 in [tasks](../../tasks.md). Existing issuance/rotation evidence does not prove the new Compose signature path.
