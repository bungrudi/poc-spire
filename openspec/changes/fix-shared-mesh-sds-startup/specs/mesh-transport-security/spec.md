## ADDED Requirements

### Requirement: MS-06 SPIRE proxy startup on a shared mesh

SPIRE-owned application Pods SHALL let the kubelet publish the injected proxy's container identity before proxy readiness waits on a SPIRE SDS SVID. They SHALL retain the CSI Workload API socket, exact Pod-bound proxy registration, mesh mTLS, and application readiness gates. The shared mesh configuration for other workloads SHALL remain unchanged.

#### Scenario: Proxy needs its own SVID

- **WHEN** a fresh SPIRE application Pod starts in the shared Istio mesh
- **THEN** its proxy can be attested from the kubelet's observed container identity and receive an exact registered SVID before the application is declared Ready
