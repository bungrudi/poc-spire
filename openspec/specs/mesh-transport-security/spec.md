# mesh-transport-security Specification

## Purpose
Define sidecar-enforced outer mesh transport independently of the demo's inner
workload/broker security, with measured layer provenance and scoped bootstrap
exceptions.

## Requirements

### Requirement: MS-01 Independent outer and inner TLS
The Kubernetes baseline SHALL use Istio sidecars and the default Istiod CA for outer mTLS. Workload HTTP inside that mesh SHALL NOT use application mTLS; its payloads SHALL use the SVID signature in `svid-signed-application-payload`. Kafka SHALL retain its separate TLS and original-producer JWS. Mesh identity or forwarded headers MUST NOT authorize inner business effects.

#### Scenario: Application transfer crosses mesh
- **WHEN** an eligible trusted caller performs its fixed transfer
- **THEN** sidecars establish outer mesh mTLS, the application presents no client certificate, and Money verifies the inner SVID signature and exact transfer identity

#### Scenario: Mesh-valid but app-invalid peer
- **WHEN** the unidentified caller has valid mesh identity but no application SVID
- **THEN** mesh admission can succeed but Money rejects the missing signature before history or effect; no mesh identity is substituted

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
Kafka and other still-encrypted application ports MUST remain opaque TCP through the mesh. Workload HTTP on the signed-payload profile is plaintext inside outer TLS and MUST NOT be wrapped in application TLS. Envoy MUST NOT terminate that HTTP identity, authorize from the JWS, or accept a forwarded identity header. Browser ingress, same-Pod UDS and required system bootstrap remain explicit outer-mesh exceptions.

#### Scenario: Double encrypted data is forwarded
- **WHEN** a still-encrypted broker or other opaque TLS connection uses its mesh service
- **THEN** outer TLS wraps its unchanged encrypted bytes and the original verifier still applies

#### Scenario: Signed HTTP is forwarded inside outer TLS
- **WHEN** a workload HTTP request uses a Money or control service on this profile
- **THEN** outer TLS protects the plaintext HTTP bytes and the receiving application, not Envoy, verifies the SVID signature

#### Scenario: Local browser or system bootstrap
- **WHEN** browser HTTPS, local UDS/controller or an enumerated infrastructure dependency starts
- **THEN** only its explicit exception or native security applies; application issuers cannot require dependent application identity to bootstrap themselves

#### Scenario: Valid app identity without outer mesh
- **WHEN** a narrowly controlled eligible app probe bypasses outer TLS on a normally protected inter-workload port
- **THEN** STRICT outer policy rejects before Money reads the payload, while separate source evidence proves the probe had a valid signing identity

### Requirement: MS-04 Observed layer and failure provenance
Acceptance MUST observe outer peer, protocol and policy together with independent signature verification, correlated per flow. Mesh rejection, signature 401, obsolete 403 and unauthorized 403 MUST remain distinct. Sidecar presence or policy YAML alone SHALL NOT prove outer mTLS plus a signed payload. Ambiguous client errors MUST remain infrastructure or unknown.

#### Scenario: Both layers succeed
- **WHEN** a positive flow is recorded as outer mesh mTLS plus an SVID signature
- **THEN** correlated sanitized observations establish actual outer authentication and inner current identity and permission

#### Scenario: One layer denies
- **WHEN** outer authentication fails, a mesh-valid client lacks a valid signature, the payload is tampered, the signature does not verify, the signature is obsolete, or a valid signer is unauthorized
- **THEN** evidence attributes the observed layer and its supported outcome without substituting one failure for another

#### Scenario: Layer cannot be observed
- **WHEN** an opaque transport failure lacks correlated evidence of where denial occurred
- **THEN** it is infrastructure or unknown, never a passing mesh or application identity-denial assertion
