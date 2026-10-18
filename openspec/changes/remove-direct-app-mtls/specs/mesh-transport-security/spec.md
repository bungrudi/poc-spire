# Spec Delta

## MODIFIED Requirements

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

## ADDED Requirements

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
