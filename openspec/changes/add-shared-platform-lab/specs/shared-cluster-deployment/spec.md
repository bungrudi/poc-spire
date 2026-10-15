## ADDED Requirements

### Requirement: Explicit platform target
The system SHALL require an explicit kubeconfig/context, expected API endpoint, CA SHA-256, kube-system UID and matching platform identity before deployment, verification or removal.

#### Scenario: Target mismatch
- **WHEN** any expected target attribute differs
- **THEN** the command fails before a Kubernetes mutation

### Requirement: Single resource ownership
The platform SHALL own AWS and shared Istio. Each PoC SHALL own only declared namespaces and explicitly inventoried cluster resources.

#### Scenario: Independent removal
- **WHEN** one PoC is removed
- **THEN** other PoC namespaces, shared Istio and the cluster remain intact

### Requirement: Reproducible configuration
The system SHALL use immutable source revisions and image references and accept runtime environment configuration without copying component definitions or committing secrets.

#### Scenario: Unresolved composition
- **WHEN** source or image pins are unresolved
- **THEN** deployment is refused with an actionable error

### Requirement: Honest lifecycle evidence
The system SHALL export sanitized evidence before teardown and distinguish static validation from runtime acceptance.

#### Scenario: Teardown evidence failure
- **WHEN** evidence export fails
- **THEN** normal teardown stops rather than reporting success or discarding the evidence

### Requirement: Approved shared lab account and region
The shared lab SHALL run only in AWS account `581695527661` and Sydney `ap-southeast-2`. Infrastructure, state bootstrap, backend configuration and PoC target guards SHALL agree on this location. Commands SHALL reject any other account or region before mutating AWS or Kubernetes resources. This lab location does not change a separate production target.

#### Scenario: Approved Sydney target
- **WHEN** the configured account is `581695527661`, the region is `ap-southeast-2` and all other explicit target checks pass
- **THEN** location validation permits the operation

#### Scenario: Former Jakarta target
- **WHEN** a shared-lab operation is configured with `ap-southeast-3`
- **THEN** it is rejected before mutation rather than silently redirected

#### Scenario: Unexpected account
- **WHEN** the configured or authenticated account differs from `581695527661`
- **THEN** the operation is rejected before mutation

### Requirement: Free Plan compatible workload identity
The platform SHALL use EKS Pod Identity for EBS, Loki and Tempo with role trust scoped to the expected cluster ARN, namespace and service account. PoCs SHALL NOT configure static AWS credentials or IRSA annotations for this profile. The platform SHALL preserve the existing account setup.

#### Scenario: Scoped credential agent access
- **WHEN** an owned workload requires AWS credentials
- **THEN** its policy permits the Pod Identity agent at 169.254.170.23/32 TCP 80 and its assigned service account uses only the associated role

### Requirement: Single selected SPIRE worker
SPIRE SHALL require exactly one Ready amd64 node labelled with the expected lab and poc-platform/workload=spire. It SHALL preserve its observed-node identity binding and refuse ambiguous or missing selections.

#### Scenario: Second non-SPIRE worker
- **WHEN** an observability worker exists alongside the single selected SPIRE worker
- **THEN** SPIRE deploys only to the selected SPIRE worker

#### Scenario: Missing or ambiguous SPIRE worker
- **WHEN** zero or multiple SPIRE-labelled nodes exist, or the selected node is not Ready
- **THEN** SPIRE refuses deployment before creating its namespaces

### Requirement: Owned SPIRE infrastructure connectivity
The shared adapter SHALL install its infrastructure connectivity policies in its owned SPIRE namespace before issuer or agent startup. Agent kubelet egress SHALL be restricted to the selected node InternalIP on TCP 10250. Application ingress rules and kubelet TLS verification SHALL remain unchanged.

#### Scenario: Strict networking startup
- **WHEN** SPIRE is deployed to the shared EKS cluster
- **THEN** only its server and agent selectors receive DNS/API connectivity, the agent can reach the server on TCP 8081, and kubelet access is scoped to the selected node

#### Scenario: Unverified node address
- **WHEN** the selected node lacks a single valid private IPv4 InternalIP
- **THEN** deployment refuses to broaden kubelet access
