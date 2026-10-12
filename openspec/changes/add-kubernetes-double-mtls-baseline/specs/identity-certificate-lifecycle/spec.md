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
Approved retained/native and remote-demo identity restart contracts SHALL remain unchanged. Kubernetes identity/state SHALL be disposable within its owned run, without durability/PVC/backup/HA requirements. Container restart may reuse intact run state; destructive issuer/Pod/trust loss MUST invalidate the run and require explicit fresh start. No retained trust reset, silent policy broadening or automatic old-attempt republication is allowed.

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
- **THEN** same-run bootstrap refuses to invent continuity; the operator performs explicit owned fresh start with new trust/run/stream and old logical results remain unknown

#### Scenario: In-cluster agent is unavailable
- **WHEN** the node Agent/CSI cannot supply required fresh material or its current cache expires
- **THEN** the result is infrastructure/expiry rather than attestation rejection; valid issued caches alone are not retroactively revoked


## Traceability and proposed coverage

All original modified-block scenarios are retained. New Kubernetes scenarios are
NOT RUN. [Profile](../../profile.md), [KB acceptance](../../acceptance.md) and
[references](../../references.md) define the proposed gates. Current source and
main contracts remain unchanged at `98f0fa3`; existing evidence is background,
not Kubernetes proof.
