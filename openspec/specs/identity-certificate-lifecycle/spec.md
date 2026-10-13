# Identity and certificate lifecycle

## Purpose

Define workload credential rotation, trust-bundle refresh and explicit lifecycle boundaries in the measured substrate and isolated demo fixtures.

## Requirements

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

### Requirement: IC-03 Cached lifetime and honest outage outcomes
Removing registration MUST prevent fresh matching issuance without claiming immediate revocation of previously issued SVIDs. During SPIRE outage, already-valid cached credentials SHALL remain usable only within their validity; expiry, missing socket and unavailable agent MUST fail closed and be distinguished from observed policy denial.

#### Scenario: Cached credential still valid
- **WHEN** registration is removed or the server is unavailable while a previously issued credential is valid
- **THEN** that credential may still authenticate until its lifetime boundary

#### Scenario: Cache expires or the agent socket is unavailable
- **WHEN** cached credentials expire or a caller cannot contact its required agent/socket
- **THEN** it cannot transfer and its outcome identifies expiry/infrastructure failure rather than a proven executable-policy rejection

### Requirement: IC-04 Explicit state retention and recovery boundaries
Approved retained/native and remote-demo identity restart contracts SHALL remain unchanged. Kubernetes identity/state SHALL be disposable: on a failed or corrupt issuer/run the operator MUST stop and explicitly recreate the whole owned cluster with new trust/run/stream. No retained trust reset, silent policy broadening, automatic old-attempt republication or durability/recovery claim is allowed.

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
- **AND** Automatic issuer database/key integrity or lineage and exhaustive same-Pod/crash/outage recovery are DEFERRED; current credential/trust validity rules remain unchanged.

#### Scenario: In-cluster agent is unavailable
- **WHEN** the node Agent/CSI cannot supply required fresh material or its current cache expires
- **THEN** the result is infrastructure/expiry rather than attestation rejection; valid issued caches alone are not retroactively revoked

## Traceability and limitations

See [IC-01–IC-04 sources, checks and evidence](../../../docs/spec-baseline.md#identity-and-certificate-lifecycle). Short-TTL rollover and administrative old-root taint/removal have recorded evidence. Automatic expired-root sweeping, production-duration 24-hour rollover and immediate revocation remain unproven; the SPIRE pruning grace/timer cannot be inferred from short-TTL tests. Browser certificate replacement is specified in HTTPS UI.

The approved opt-in Kafka extension is mapped in [JS coverage](../../../docs/kafka-signing-acceptance.md). Its full wire/receipt/retry rules are in the [stable profile](../../../docs/kafka-signing-profile.md).
