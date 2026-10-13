# Spec Delta: identity-certificate-lifecycle

## MODIFIED Requirements

### Requirement: IC-02 Validated atomic public trust bundle
The unidentified caller SHALL obtain no app SVID/key and load authentic public SPIRE trust per probe. Publication MUST validate complete CA PEM, set public-read permissions before atomic replacement and preserve valid data on failed refresh. Kubernetes SHALL use a restricted publisher and read-only whole-directory ConfigMap projection, not subPath. Missing/wrong/unusable trust is infrastructure failure; valid cached trust remains usable within current credential bounds.

#### Scenario: CA rollover and bundle refresh
- **WHEN** a valid changed bundle is fetched from the private SPIRE administration path
- **THEN** complete public trust is refreshed and the no-certificate caller can verify Money before its client-authentication rejection

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

### Requirement: IC-04 Explicit state retention and recovery boundaries
Approved retained/native and remote-demo identity restart contracts SHALL remain unchanged. Kubernetes identity/state SHALL be disposable: on a failed or corrupt issuer/run the operator MUST stop and explicitly recreate the whole owned cluster with new trust/run/stream. No retained trust reset, silent policy broadening, automatic old-attempt republication or durability/recovery claim is allowed. Automatic issuer database/key integrity or lineage and exhaustive same-Pod/crash/outage recovery are DEFERRED; current credential/trust validity rules remain unchanged.

#### Scenario: Retained substrate restarts
- **WHEN** its server/agent restart from approved protected state
- **THEN** CA and parent identity remain unchanged and issuance resumes without a new persisted join token

#### Scenario: Node credentials expire during prolonged fixture outage
- **WHEN** both cached node/workload credentials expire in an isolated lifecycle fixture
- **THEN** recovery requires explicit fixture-only re-enrollment and reviewed parent-bound registrations
- **AND** the approved retained state remains unchanged

#### Scenario: Full demo restarts
- **WHEN** the supervised temporary demo stack is fully restarted
- **THEN** its temporary trust, fake balances and retry history are recreated independently of retained substrate state

#### Scenario: Disposable Kubernetes issuer is lost
- **WHEN** a guarded SPIRE Server Pod/trust incarnation or expected identity state is destructively replaced
- **THEN** the operator stops and explicitly recreates the whole owned cluster with new trust/run/stream; continuity is not invented and old logical results may remain unknown

#### Scenario: In-cluster agent is unavailable
- **WHEN** the node Agent/CSI cannot supply required fresh material or its current cache expires
- **THEN** the result is infrastructure/expiry rather than attestation rejection; valid issued caches alone are not retroactively revoked


## Traceability and proposed coverage

Original modified-block scenarios and existing Compose contracts remain unchanged.
[Review](../../review.md) records the 13 October scope narrowing.
[Profile](../../profile.md) and [acceptance](../../acceptance.md) retain core
security checks and explicitly defer extended Kubernetes recovery. Development
proofs are provisional; final core acceptance is pending. No deferral is a PASS.
