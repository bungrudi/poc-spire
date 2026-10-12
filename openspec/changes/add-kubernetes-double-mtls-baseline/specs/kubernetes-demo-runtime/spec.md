# Spec Delta: Kubernetes demo runtime

## Purpose

Define an owned disposable Kubernetes deployment and reproducibly validated
baseline for the current fixed SPIRE/Kafka learning demo, without durability or
public cutover.

## ADDED Requirements

### Requirement: KR-01 Complete owned in-cluster topology
The opt-in Kubernetes profile SHALL run SPIRE Server, node Agents/CSI, UI, both callers, Money, Kafka and required bootstrap components inside one owned cluster. It MUST use separate caller Pods with equal app UID/GID 1000, one Money instance and one Kafka partition. Existing Compose/public runtime and retained identity MUST remain unchanged.

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
No durability, HA, backup or PVC feature SHALL be required. Within a run, bounded replay/unknown-result contracts MUST remain. Startup MUST bind initialized state to expected run/Pod/trust identities; destructive replacement/loss SHALL stop both Sync/Kafka UI/adapter dispatch and refuse same-run reinitialization/publication. Explicit fresh start MUST use new run/trust/stream boundaries.

#### Scenario: Container restarts with intact volumes
- **WHEN** UI, caller or adapter containers restart within their existing Pod/run
- **THEN** surviving initialized caller state can recover original IDs/deadlines/variants; UI session loss does not reset a continuing Money ledger

#### Scenario: Money process restarts
- **WHEN** Money restarts even with the same Pod UID
- **THEN** memory resets and instance nonce changes, unknown results remain unknown and a retained valid command can apply again without a durable claim

#### Scenario: Caller broker or issuer state is lost
- **WHEN** expected initialized data or a guarded stateful Pod/trust incarnation is destructively replaced
- **THEN** run readiness/control fails closed, old attempts are not recreated/re-published and recovery requires an explicit run-wide fresh start

#### Scenario: Invalid Kubernetes run stops both transports
- **WHEN** destructive caller Kafka-state or guarded Pod/trust loss invalidates the Kubernetes run
- **THEN** new UI creation/retry and adapter forwarding for Sync and Kafka return 503 run_invalid without new caller invocation until explicit fresh-run reset
- **AND** already submitted work remains unknown/history-backed rather than undone; the existing Compose Sync-survival expectation is unchanged

#### Scenario: Fresh start is repeated
- **WHEN** the operator deliberately tears down and recreates a run
- **THEN** only owned resources are removed, new trust/stream/context is initialized once and old browser context cannot silently resume publication

### Requirement: KR-05 Ownership and cleanup
Every run SHALL identify its cluster, kubeconfig, namespace/Pod UIDs, temporary credentials and processes. Collision, startup failure and shutdown MUST remove only owned resources with verifiable bounded cleanup. Default kubecontext, public endpoint, unrelated workloads and retained identity MUST NOT be changed or reused.

#### Scenario: Name port or context collision
- **WHEN** preflight detects an occupied port/name or foreign context/resource identity
- **THEN** it refuses startup before touching that resource and cleans only its own partial state

#### Scenario: Failure or full shutdown
- **WHEN** startup/acceptance fails or the operator stops the run
- **THEN** cleanup completion is checked, incomplete cleanup remains failed/retryable and actual protected service/state continuity is recorded

### Requirement: KR-06 Reproducible validated baseline freeze
Freeze MUST follow committed final source/configuration, immutable component/app artifacts, complete parity/security acceptance and independent review. Actual topology/resources and runtime manifest hashes MUST match the tested inputs without unexplained drift. Sanitized evidence SHALL state proof modes/limits; later evidence commits MUST NOT alter tested source. No comparison or performance claim is included.

#### Scenario: Known-good baseline is recorded
- **WHEN** all migration checks pass against a committed source and restored normal configuration
- **THEN** freeze records exact image/binary/component/configuration hashes, manifest normalization, topology/resource settings, evidence and independent review

#### Scenario: Drift or incomplete proof
- **WHEN** deployed inputs differ, test overlays are not restored or a required case/layer proof is missing
- **THEN** freeze fails; a successful button or earlier Compose result cannot substitute for the missing Kubernetes proof

#### Scenario: Future comparison is considered
- **WHEN** a later proposal disables only outer TLS while retaining sidecars and application security
- **THEN** it starts from the frozen baseline and separately reviews mesh-principal-dependent policy differences; this change creates no comparison worktree or benchmark mode

### Requirement: KR-07 Complete migration parity
Acceptance SHALL map all five fixed UI scenarios and JS-01–JS-23, declaring Kubernetes transport/layout changes and JS-22 invalid-run availability separately from unchanged Compose behavior. Original HTTP policy, inner credentials, JWS, shared memory ledger, bounded quotas and unknown-result semantics MUST remain. Every new runtime case MUST have evidence or remain explicitly NOT RUN.

#### Scenario: Fixed scenario and security parity
- **WHEN** the Kubernetes baseline is tested
- **THEN** both Sync and all three Kafka cases retain their correct outcomes, with fresh negative/lifecycle/guard/retry proof rather than importing Compose passes

#### Scenario: A proof is only deterministic or unexecuted
- **WHEN** an obligation uses controlled clock/transport/bundle fixtures or has not run
- **THEN** its mode/status is explicit and no infrastructure error or structural validation is recorded as security acceptance

## Traceability and status

All scenarios are proposed and NOT RUN. [KB acceptance](../../acceptance.md),
[profile](../../profile.md) and [references](../../references.md) define the gates,
flow/reset/freeze contract and compatibility assumptions. Current implementation
is [the unchanged baseline](../../../../../docs/kafka-signing-acceptance.md).
