# kubernetes-demo-runtime Specification

## Purpose
Define an owned disposable Kubernetes deployment and reproducibly validated
baseline for the current fixed SPIRE/Kafka learning demo, without durability or
public cutover.

## Requirements

### Requirement: KR-01 Complete owned in-cluster topology
The opt-in Kubernetes profile SHALL run SPIRE Server, Agents/CSI, UI, both callers, Money, Kafka and bootstrap inside one owned cluster, with separate UID/GID 1000 caller Pods, one Money and one Kafka partition. Retained identity and unrelated services MUST remain unchanged. Private baseline preserves existing Compose/public runtime; explicit named public cutover SHALL retire only the identified superseded PoC runtimes.

#### Scenario: All runtime components are placed
- **WHEN** the approved Kubernetes baseline starts
- **THEN** every required component is deployed in its owned cluster with recorded Pod/container placement and no native host SPIRE dependency

#### Scenario: Missing component or unsupported replica
- **WHEN** a required component is absent or Money/Kafka topology exceeds the supported single instance/partition
- **THEN** readiness and baseline acceptance fail without silently using host/Compose resources or claiming replicated ledger safety

### Requirement: KR-02 Ordered bootstrap and complete readiness
Startup SHALL follow the profile's dependency edges without requiring app mesh/SVID credentials to bootstrap their issuing infrastructure. Complete readiness MUST prove enforcing networking, mesh, SPIRE/CSI/attestation, public trust, private controls, broker TLS/topic/ACLs, Money assignment and authenticated UI reads. Liveness alone MUST NOT authorize dispatch.

#### Scenario: Cold start with empty disposable state
- **WHEN** a new run bootstraps its cluster, issuers, policies and workloads
- **THEN** each stage has bounded positive/negative readiness and metadata needed for registration is obtainable before dependent identity/readiness waits

#### Scenario: Live process with unavailable dependency
- **WHEN** an application answers liveness but a required identity, control, trust, broker or consumer dependency is unavailable
- **THEN** unavailable required operations fail readiness and report infrastructure/unknown rather than identity rejection, plaintext fallback or success
- **AND** issuer outage alone does not invalidate usable cached SVIDs/trust; current cached operations remain allowed until their existing validity boundary

#### Scenario: Bootstrap job or mount stalls
- **WHEN** CSI registration, source startup or an injected Job cannot complete within its bound
- **THEN** startup fails with owned cleanup; a sidecar cannot leave a falsely successful or indefinitely running bootstrap Job

### Requirement: KR-03 Enforced isolation and resource boundaries
The cluster SHALL enforce role-scoped network policy and explicit mesh flows, not merely install policy YAML. Apps MUST retain hardened roots/capabilities and no admin/host credentials; substrate privileges MUST be declared and bounded to owned nodes. Capacity checks and limits MUST preserve shared-host services and fail before exceeding the approved budget.

#### Scenario: UI attempts broker or foreign control access
- **WHEN** a UI/test workload targets a forbidden broker/control flow
- **THEN** real negative reachability demonstrates enforcement, with no credential or namespace bypass

#### Scenario: Same-Pod processes share networking
- **WHEN** caller and adapter share a Pod and mesh SA
- **THEN** evidence states that mesh/network identity is shared and proves their separate inner identities/mounts instead of claiming process isolation from Pod networking

#### Scenario: Budget or privilege is unsuitable
- **WHEN** required resource limits, reserve or approved node visibility cannot be met
- **THEN** implementation stops at the named gate without evicting other workloads, widening app privilege or changing physical-host security settings

### Requirement: KR-04 Disposable state and explicit reset
Kubernetes run state SHALL be disposable. On a failed or corrupt run, the operator MUST stop the whole owned cluster and explicitly recreate a fresh run/trust/stream. Existing useful guards SHALL remain; no silent state repair, registration broadening, automatic old-attempt republication or durability claim is permitted. Automatic integrity/loss monitoring and exhaustive same-Pod/crash recovery are not required by this bounded PoC. Existing Compose contracts SHALL remain unchanged.

#### Scenario: Kubernetes run fails or is corrupt
- **WHEN** a Kubernetes run fails or its disposable state is corrupt or lost
- **THEN** the operator stops the whole owned cluster and explicitly recreates it; previous logical outcomes may remain unknown and no same-run recovery is promised

#### Scenario: Fresh start is repeated
- **WHEN** the operator deliberately tears down and recreates a run
- **THEN** only owned resources are removed, new trust/run/stream/context is initialized once and old browser context cannot silently resume publication

#### Scenario: Memory-only limitation is described
- **WHEN** Money or the whole run starts a new lifetime
- **THEN** balances/history/receipts may reset and manually reused IDs may act anew, without a durable or cross-run exactly-once guarantee

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

### Requirement: KR-06 Reproducible validated baseline freeze
Freeze MUST use committed source/config, pinned normal and distinct test artifacts, retained core proofs, relevant tests, basic cleanup/coexistence and independent review. Deployed topology/resources and restored hashes MUST match tested inputs. Evidence SHALL state modes and deferred coverage; DEFERRED cases are neither PASS nor prerequisites. Evidence-only follow-up SHALL NOT change tested source. No comparison or performance claim is included.

#### Scenario: Known-good core baseline is recorded
- **WHEN** retained core acceptance passes against committed source and restored normal configuration
- **THEN** freeze records exact image/binary/component/configuration hashes, normalization, topology/resources, evidence, deferred coverage and independent review

#### Scenario: Drift or missing core proof
- **WHEN** deployed inputs differ, an overlay is not restored or a required core security/layer proof is missing
- **THEN** freeze fails; a successful button, deferred case or earlier Compose result cannot replace that proof

#### Scenario: Future comparison is considered
- **WHEN** a later proposal disables only outer TLS while retaining sidecars and application security
- **THEN** it separately reviews mesh-principal-dependent policies; this change creates no comparison worktree or benchmark mode

### Requirement: KR-07 Core migration acceptance
Kubernetes acceptance SHALL prove five scenarios, exact attestation/authorization/JWS, correlated inner/outer TLS, private role boundaries and basic reproducibility/cleanup/coexistence. Existing HTTP/JWS/ledger/quota/unknown semantics and Compose contracts SHALL remain. Full JS reliability re-proving, issuer integrity/lineage, exhaustive recovery and failure injection are DEFERRED; mixed cases MUST retain core security checks.

#### Scenario: Fixed scenarios and focused security
- **WHEN** the Kubernetes core PoC is accepted
- **THEN** all five outcomes and focused attestation, authorization, JWS and layer positives/negatives have current-source evidence

#### Scenario: Coverage is partial or deferred
- **WHEN** an obligation is deterministic, partial, unexecuted or outside the narrowed scope
- **THEN** its mode/status is explicit; infrastructure errors and DEFERRED cases are never recorded as security PASS

### Requirement: KR-08 Supervised named public Kubernetes route
The public profile SHALL keep scripts/state in main and use the existing HTTPS edge/certificate for only spire.adianto.dev. Its UI forward MUST bind literal loopback and use the exact owned kubeconfig/context. Route supervision MUST fail closed and preserve the current run on a route-only restart. No controller, HA, automatic state repair, extra public application port or resource expansion is permitted.

#### Scenario: Public route starts or restarts
- **WHEN** the owned public run is ready and its route service starts or restarts
- **THEN** the named HTTPS domain reaches that run's UI with validated upstream TLS and unchanged Pod/config guards, five scenarios, ledger and browser defenses

#### Scenario: Ownership or upstream TLS is invalid
- **WHEN** the descriptor/profile/context differs or upstream certificate verification fails
- **THEN** no wrong workload, plaintext fallback or broad context adoption is used; the public route fails closed and operator whole-cluster reset remains explicit
