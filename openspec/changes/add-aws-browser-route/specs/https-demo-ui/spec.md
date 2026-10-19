## ADDED Requirements

### Requirement: UI-09 AWS-assigned public browser origin

The shared-cluster profile MAY expose the assumed-operator UI at an explicitly pinned AWS CloudFront HTTPS origin. Its dedicated browser HTTPS listener SHALL accept TLS 1.2 or newer for the ALB backend connection. Its browser certificate, Host/Origin checks, UI Service and ingress policy SHALL agree on the exact route. SPIRE-issued mesh mTLS SHALL retain TLS 1.3. Money and callers SHALL remain private. The existing loopback profile SHALL remain available when no public origin is configured.

#### Scenario: CloudFront browser request

- **WHEN** a user opens the pinned AWS HTTPS URL through the ALB route
- **THEN** only the Ready UI receives it, its certificate matches the configured origin, and its existing CSRF and signed Money checks run

#### Scenario: ALB backend handshake

- **WHEN** the ALB health-checks a Ready UI using TLS 1.2
- **THEN** the UI completes browser TLS and applies its Host check without changing SPIRE workload mTLS

#### Scenario: Mismatched route input

- **WHEN** an origin, NodePort or ALB subnet input is missing or inconsistent
- **THEN** shared deployment refuses the route before changing Kubernetes resources
