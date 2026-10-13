# Spec Delta: Named public Kubernetes runtime

## MODIFIED Requirements

### Requirement: KR-01 Complete owned in-cluster topology
The opt-in Kubernetes profile SHALL run SPIRE Server, Agents/CSI, UI, both callers, Money, Kafka and bootstrap inside one owned cluster, with separate UID/GID 1000 caller Pods, one Money and one Kafka partition. Retained identity and unrelated services MUST remain unchanged. Private baseline preserves existing Compose/public runtime; explicit named public cutover SHALL retire only the identified superseded PoC runtimes.

#### Scenario: All runtime components are placed
- **WHEN** the approved Kubernetes baseline starts
- **THEN** every required component is deployed in its owned cluster with recorded Pod/container placement and no native host SPIRE dependency

#### Scenario: Missing component or unsupported replica
- **WHEN** a required component is absent or Money/Kafka topology exceeds the supported single instance/partition
- **THEN** readiness and baseline acceptance fail without silently using host/Compose resources or claiming replicated ledger safety

### Requirement: KR-05 Ownership and cleanup
Every run SHALL identify its cluster, kubeconfig, namespaces/Pods, temporary credentials and processes. Cleanup MUST remove only owned resources. Default context, retained identity and unrelated workloads MUST remain unchanged. The explicitly authorized public profile SHALL own only spire.adianto.dev and allow retirement of precisely recorded old PoC IDs; its own endpoint downtime during whole-run stop/reset is not protected foreign-service continuity.

#### Scenario: Name port or context collision
- **WHEN** preflight detects an occupied port/name or foreign context/resource identity
- **THEN** it refuses startup before touching that resource and cleans only its own partial state

#### Scenario: Failure or full shutdown
- **WHEN** startup/acceptance fails or the operator stops the run
- **THEN** cleanup completion is checked, incomplete cleanup remains failed/retryable and actual protected service/state continuity is recorded

#### Scenario: Authorized public retirement
- **WHEN** the named public Kubernetes route is verified and superseded public/preview PoC runtimes are stopped
- **THEN** only captured owned stack/container/process IDs are retired and excluded from public-run continuity, while original foreign-container/default-context/retained-state fingerprints remain enforced


## ADDED Requirements

### Requirement: KR-08 Supervised named public Kubernetes route
The public profile SHALL keep scripts/state in main and use the existing HTTPS edge/certificate for only spire.adianto.dev. Its UI forward MUST bind literal loopback and use the exact owned kubeconfig/context. Route supervision MUST fail closed and preserve the current run on a route-only restart. No controller, HA, automatic state repair, extra public application port or resource expansion is permitted.

#### Scenario: Public route starts or restarts
- **WHEN** the owned public run is ready and its route service starts or restarts
- **THEN** the named HTTPS domain reaches that run's UI with validated upstream TLS and unchanged Pod/config guards, five scenarios, ledger and browser defenses

#### Scenario: Ownership or upstream TLS is invalid
- **WHEN** the descriptor/profile/context differs or upstream certificate verification fails
- **THEN** no wrong workload, plaintext fallback or broad context adoption is used; the public route fails closed and operator whole-cluster reset remains explicit
