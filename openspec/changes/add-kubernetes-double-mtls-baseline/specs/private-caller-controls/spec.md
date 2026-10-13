# Spec Delta: private-caller-controls

## MODIFIED Requirements

### Requirement: PC-01 Separate private trigger channels
Caller executables SHALL expose only separate local UDS controls. Compose UI SHALL retain separate direct control mounts. Kubernetes UI SHALL instead use the exact-ID adapters in PC-07; only each adapter mounts its own caller UDS. The unidentified caller MUST NOT reach trusted control. Kernel peer checks and private mounts remain; UID/Pod/mesh membership alone MUST NOT grant executable or remote-control authority.

#### Scenario: UI triggers the selected caller
- **WHEN** the UI sends a command over the selected caller's separately mounted socket
- **THEN** only that caller executes the attempt

#### Scenario: Unidentified caller targets trusted control
- **WHEN** the unidentified container attempts to open the trusted control path or reach a network trigger
- **THEN** no reachable trusted control endpoint exists and no trusted attempt is dispatched

#### Scenario: Kubernetes UI uses separate authenticated controls
- **WHEN** the Kubernetes UI selects a caller through its configured adapter
- **THEN** it verifies that exact adapter and borrows no local control mount; each adapter can reach only its own UID-checked caller

### Requirement: PC-05 Fixed private Kafka controls
Opt-in callers SHALL keep POST /attempt/kafka with exactly request_id, scenario and a 32-hex authenticated-Money instance nonce, plus GET /attempt/kafka/{request_id}. Trusted permits only kafka-trusted/kafka-tampered; unidentified only kafka-unidentified. Peer checking, strict 1024-byte input and bounded deadlines remain. Caller executables expose no network listener/key export; Kubernetes adapters in PC-07 only mirror these fixed controls.

#### Scenario: Fixed Kafka command is published
- **WHEN** an authorized UI peer submits a valid fixed scenario/ID/instance tuple on the corresponding private socket
- **THEN** the caller reserves exact record metadata before publishing and reports queued or publish_unknown independently of Money effects

#### Scenario: Wrong scenario peer or arbitrary payload
- **WHEN** a peer is unauthorized or sends another caller's scenario, extra fields, oversized data or selectable broker/amount/identity
- **THEN** the command is rejected before publication and the old POST /attempt contract remains unchanged

### Requirement: PC-06 Initialized bounded retry state
Runner SHALL initialize one stream-bound caller manifest/index before startup, retaining at most 64 immutable descriptors/four variants atomically before publication. Callers MUST NOT repair missing/corrupt state; new IDs at capacity fail without eviction and original ID/deadline/unknown-result semantics remain. Existing Compose intact-restart recovery contracts remain unchanged. Failed/corrupt Kubernetes runs require operator whole-cluster stop/recreate under KR-04; automatic state-loss detection and exhaustive Kubernetes restart/variant recovery validation are DEFERRED. The original recovery scenarios below remain Compose contracts and are not a new Kubernetes recovery guarantee.

#### Scenario: Lost publish response then restart
- **WHEN** publication may have happened but its control response is lost and UI/caller container restarts in the same run
- **THEN** initialized state recovers the exact descriptor/all potentially submitted digests without a new ID/deadline

#### Scenario: State loss or descriptor expiry
- **WHEN** expected state is missing/corrupt, stream/instance differs, or the original deadline expired
- **THEN** Kafka publish fails closed as unknown/reset/expired instead of creating a fresh descriptor; metadata lookup can still recover surviving evidence

#### Scenario: Kubernetes caller Pod is replaced
- **WHEN** a caller Pod UID changes or expected initialized state is lost within its current run
- **THEN** the operator stops and explicitly recreates the whole owned run rather than repairing it or automatically republishing old attempts; no same-run recovery is promised


## ADDED Requirements

### Requirement: PC-07 Exact-identity Kubernetes control adapters
Each Kubernetes caller Pod SHALL have a separately attested control-only adapter. UI MUST verify its exact SVID; adapter MUST authorize only exact UI ID before fixed bounded processing and forward only to its own UID-checked UDS. No generic proxy/signing API, application-key lending, caller-state/broker-key mount or transfer identity is permitted. Remote credential expiry/unknown-response guards MUST preserve original attempt semantics.

#### Scenario: UI triggers a fixed operation
- **WHEN** UI authenticates the selected adapter and submits the unchanged fixed Sync or caller-specific Kafka schema
- **THEN** adapter verifies current UI identity/body/limits, reaches only its own caller as UID 10002 and relays bounded correlated diagnostics without inventing Money effects

#### Scenario: Remote peer or payload is unauthorized
- **WHEN** a no-certificate or valid wrong-ID client connects, or input supplies extra fields/destination/signing parameters
- **THEN** TLS denies no-certificate clients or adapter returns403 for authenticated unauthorized peers; malformed fixed input returns400 before UDS execution

#### Scenario: Adapter seeks app or foreign authority
- **WHEN** an adapter requests a transfer SVID, uses its control SVID at Money or targets another caller/state/broker key
- **THEN** issuance/inner authorization/mount boundaries deny that authority; shared Pod mesh identity does not substitute

#### Scenario: Remote response or credential becomes uncertain
- **WHEN** a control response is lost/uncorrelated or remote credentials cross expiry before forwarding
- **THEN** preserve the original ID/deadline and unknown-result handling without another identity, automatic publication or fabricated verified Money result

## Traceability and proposed coverage

Original modified-block scenarios and existing Compose contracts remain unchanged.
[Review](../../review.md) records the 13 October scope narrowing.
[Profile](../../profile.md) and [acceptance](../../acceptance.md) retain core
security checks and explicitly defer extended Kubernetes recovery. Development
proofs are provisional; final core acceptance is pending. No deferral is a PASS.
