# mesh-transport-security Specification

## Purpose
Define SPIRE-issued sidecar mesh credentials independently of application signing
credentials, with exact traffic policy, safe bootstrap and measured provenance.

## Requirements

### Requirement: MS-01 Independent outer and inner TLS
The Kubernetes baseline SHALL use SPIRE-issued Envoy SVIDs through local SDS for mesh mTLS in `poc.internal`. Proxy identities, keys and grants MUST remain separate from app signing credentials. Workload HTTP MUST use signed payloads; Kafka retains broker TLS and original-producer JWS. Mesh identity or forwarded headers MUST NOT authorize business effects.

#### Scenario: Application transfer crosses mesh
- **WHEN** an eligible trusted caller performs its fixed transfer
- **THEN** sidecars establish SPIRE-backed mesh mTLS and Money independently verifies the caller's payload signature and exact application identity

#### Scenario: Mesh-valid but app-invalid peer
- **WHEN** the unidentified caller has valid mesh identity but no application signing SVID
- **THEN** mesh admission can succeed but Money rejects the missing signature before history or effect

#### Scenario: Proxy requests credentials
- **WHEN** Envoy connects through its CSI-mounted SDS socket
- **THEN** its narrow proxy registration supplies only `spiffe://poc.internal/ns/<namespace>/sa/<service-account>` with a separate private key, never an application transfer SVID

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

### Requirement: MS-05 SPIRE mesh bootstrap and rotation
Owned proxies SHALL consume SPIRE SDS through CSI, using narrow node-parent, namespace/SA, proxy image, UID, Pod UID and run selectors. Registration MUST precede readiness, including the admin Job. Istiod retains discovery/policy; proxies MUST NOT fall back to its workload CA. Policy principals MUST use `poc.internal`. Shared tooling MUST require a compatible mesh without mutating its control plane.

#### Scenario: New proxy is waiting for identity
- **WHEN** a workload Pod exists but its proxy lacks a matching SPIRE registration
- **THEN** mesh readiness fails closed, and bootstrap can observe/register the proxy before waiting for app readiness

#### Scenario: Proxy certificate rotates
- **WHEN** SPIRE renews a proxy SVID or updates its bundle
- **THEN** Envoy consumes the SDS update independently of application signing rotation; runtime acceptance must observe actual issuer and rotation before recording a pass

#### Scenario: Shared mesh is incompatible
- **WHEN** a shared platform has not configured SPIRE SDS and the matching trust domain
- **THEN** the demo refuses to proceed with a mixed issuer profile and does not change the shared mesh control plane
