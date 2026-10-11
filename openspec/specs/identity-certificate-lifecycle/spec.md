# Identity and certificate lifecycle

## Purpose

Define workload credential rotation, trust-bundle refresh and explicit lifecycle boundaries in the measured substrate and isolated demo fixtures.

## Requirements

### Requirement: IC-01 Rotating sources and fresh workload connections
Eligible Money, trusted caller and UI workloads SHALL obtain SVIDs and trust through updating X.509-SVID sources and close sources at shutdown. Internal operations MUST use fresh TLS handshakes without connection reuse, session resumption or HTTP/2. Current credentials SHALL rotate without an unauthenticated fallback; stale credentials MUST NOT authorize a transfer.

#### Scenario: Source rotates during operation
- **WHEN** SPIRE renews an eligible workload's SVID
- **THEN** new operations use the current credential and verify Money without resetting the ledger

#### Scenario: Frozen expired credential or reuse attempt
- **WHEN** a client attempts an operation with a frozen expired credential or tries to reuse/resume an internal connection
- **THEN** an expired credential cannot transfer and supported requests use fresh nonresumed TLS handshakes

### Requirement: IC-02 Validated atomic public trust bundle
The unidentified caller SHALL obtain no SVID and load the published public trust bundle separately per probe to verify Money without a client certificate. Bundle publication MUST validate fetched PEM and replace complete files atomically with public-read permissions set before replacement. Failed refresh MUST preserve the last validated bundle.

#### Scenario: CA rollover and bundle refresh
- **WHEN** a valid changed bundle is fetched from the private SPIRE administration path
- **THEN** complete public trust is refreshed and the no-certificate caller can verify Money before its client-authentication rejection

#### Scenario: Malformed refresh or missing trust material
- **WHEN** refresh times out or returns malformed PEM
- **THEN** the last validated published bundle remains intact
- **AND** missing or unrelated client trust causes a failed request classified as infrastructure/trust failure, not identity-denial proof

### Requirement: IC-03 Cached lifetime and honest outage outcomes
Removing registration MUST prevent fresh matching issuance without claiming immediate revocation of previously issued SVIDs. During SPIRE outage, already-valid cached credentials SHALL remain usable only within their validity; expiry, missing socket and unavailable agent MUST fail closed and be distinguished from observed policy denial.

#### Scenario: Cached credential still valid
- **WHEN** registration is removed or the server is unavailable while a previously issued credential is valid
- **THEN** that credential may still authenticate until its lifetime boundary

#### Scenario: Cache expires or the agent socket is unavailable
- **WHEN** cached credentials expire or a caller cannot contact its required agent/socket
- **THEN** it cannot transfer and its outcome identifies expiry/infrastructure failure rather than a proven executable-policy rejection

### Requirement: IC-04 Explicit state retention and recovery boundaries
Normal restart of the approved retained identity substrate SHALL preserve its CA and canonical agent identity without retaining a join token. A full remote-demo restart SHALL instead create fresh temporary trust and reset fake state. Expired-node recovery MUST NOT silently reset retained trust or automatically re-enroll the retained agent.

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

## Traceability and limitations

See [IC-01–IC-04 sources, checks and evidence](../../../docs/spec-baseline.md#identity-and-certificate-lifecycle). Short-TTL rollover and administrative old-root taint/removal have recorded evidence. Automatic expired-root sweeping, production-duration 24-hour rollover and immediate revocation remain unproven; the SPIRE pruning grace/timer cannot be inferred from short-TTL tests. Browser certificate replacement is specified in HTTPS UI.
