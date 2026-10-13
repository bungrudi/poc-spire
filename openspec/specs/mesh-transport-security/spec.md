# mesh-transport-security Specification

## Purpose
Define sidecar-enforced outer mesh transport independently of the demo's inner
workload/broker security, with measured layer provenance and scoped bootstrap
exceptions.

## Requirements

### Requirement: MS-01 Independent outer and inner TLS
The Kubernetes baseline SHALL use Istio sidecars/default Istiod CA for outer mTLS and independent SPIRE SVIDs for inner application mTLS. Inner Money TLS MUST terminate inside Go with existing exact-ID authorization; Kafka SHALL retain its separate TLS/DN ACLs and original-producer JWS. Mesh identity or forwarded headers MUST NOT authorize inner business effects.

#### Scenario: Application transfer crosses mesh
- **WHEN** an eligible trusted caller performs its fixed transfer
- **THEN** sidecars establish outer mesh mTLS while Money independently verifies the inner SVID and exact transfer identity

#### Scenario: Mesh-valid but app-invalid peer
- **WHEN** the unidentified caller has valid mesh identity but no application SVID
- **THEN** mesh admission can succeed but Money rejects inner client authentication before HTTP/history/effect; no mesh identity is substituted

### Requirement: MS-02 Exact mesh flows and shared Pod identity
Mesh traffic SHALL use STRICT receiving policy, ISTIO_MUTUAL origination and default-deny L4 principal/port allowlists for the profile's exact service flows. NetworkPolicy MUST actually enforce complementary role isolation. Adapter/caller processes sharing a Pod SHALL share mesh identity, while distinct inner identities and private mounts remain required.

#### Scenario: Authorized outer flow
- **WHEN** UI, caller, Money or scoped bootstrap SA targets its documented service/port
- **THEN** outer policy permits only that flow subject to the independent inner authorization layer

#### Scenario: Unauthorized mesh or broker reachability
- **WHEN** a wrong mesh SA or the UI targets a forbidden control/broker path
- **THEN** measured outer/network enforcement denies it without fabricated inner HTTP/JWS results

#### Scenario: Adapter shares caller SA
- **WHEN** the control adapter opens a permitted Pod-level route
- **THEN** it gains no application transfer SVID or broker credential merely from that shared mesh principal

### Requirement: MS-03 Opaque inner traffic and explicit exceptions
Encrypted application/control/broker ports MUST remain opaque TCP through the mesh, with no inner TLS termination/origination, plaintext fallback or HTTP identity proxy. The profile SHALL explicitly exclude browser ingress, same-Pod UDS/controller and required system bootstrap from outer mesh, without exempting normal inter-workload application traffic.

#### Scenario: Double encrypted data is forwarded
- **WHEN** an inner TLS connection uses an application/control/broker service
- **THEN** outer TLS wraps its unchanged encrypted bytes; original Go/broker verification and fixed request schemas still apply

#### Scenario: Local browser or system bootstrap
- **WHEN** browser HTTPS, local UDS/controller or an enumerated infrastructure dependency starts
- **THEN** only its explicit exception/native security applies; application issuers cannot require dependent application identity to bootstrap themselves

#### Scenario: Valid app identity without outer mesh
- **WHEN** a narrowly controlled eligible app probe bypasses outer TLS on a normally protected inter-workload port
- **THEN** STRICT outer policy rejects before inner Money HTTP, while separate source evidence proves the probe had a valid app identity

### Requirement: MS-04 Observed layer and failure provenance
Acceptance MUST observe outer peer/protocol/policy and independent inner verification, correlated per flow. Mesh rejection, app TLS rejection/no HTTP, authenticated app 403 and JWS rejection MUST remain distinct. Configuration or encrypted packet shape alone SHALL NOT prove double mTLS. Ambiguous client errors MUST remain infrastructure/unknown without invented stage or identity.

#### Scenario: Both layers succeed
- **WHEN** a positive flow is recorded as double mTLS
- **THEN** correlated sanitized observations establish actual outer authentication and inner current identity/permission, beyond sidecar presence or policy YAML

#### Scenario: One layer denies
- **WHEN** outer authentication fails, a mesh-valid client lacks app credentials, a valid app ID is unauthorized or JWS is invalid
- **THEN** evidence attributes the observed layer and its supported outcome without substituting one failure for another

#### Scenario: Layer cannot be observed
- **WHEN** an opaque transport failure lacks correlated evidence of where denial occurred
- **THEN** it is infrastructure/unknown, never a passing mesh/app identity-denial assertion
