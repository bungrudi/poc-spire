## ADDED Requirements

### Requirement: UI-09 AWS-assigned public browser origin

The shared-cluster profile MAY expose the assumed-operator UI at an explicitly pinned AWS CloudFront HTTPS origin. Its backend browser certificate, Host/Origin checks, UI Service and ingress policy SHALL agree on the exact route. Money and callers SHALL remain private. The existing loopback profile SHALL remain available when no public origin is configured.

#### Scenario: CloudFront browser request

- **WHEN** a user opens the pinned AWS HTTPS URL through the ALB route
- **THEN** only the Ready UI receives it, its certificate matches the configured origin, and its existing CSRF and signed Money checks run

#### Scenario: Mismatched route input

- **WHEN** an origin, NodePort or ALB subnet input is missing or inconsistent
- **THEN** shared deployment refuses the route before changing Kubernetes resources
