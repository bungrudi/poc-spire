# Spec Delta: workload-attestation

## MODIFIED Requirements

### Requirement: WA-01 Executable and deployment conjunction
Identity issuance MUST bind exact attested parent, UID, executable SHA-256, deployment and image conditions together. Native profiles SHALL retain SPIRE 1.14.0 Unix/Docker config-digest attestation. Kubernetes SHALL require verified built-in Unix/K8s selectors, secure kubelet and actual process visibility; its immutable requesting-container image selector MUST NOT be substituted by a Docker config digest or Pod identity alone.

#### Scenario: Approved executable in approved deployment
- **WHEN** the requesting executable satisfies every registered condition
- **THEN** it receives its exact registered SPIFFE ID

#### Scenario: Wrong executable under matching deployment conditions
- **WHEN** a distinct executable runs with the same UID, image and deployment role in the controlled trusted fixture
- **THEN** it receives no matching identity despite access to the Workload API

#### Scenario: Approved executable in wrong deployment
- **WHEN** the approved executable runs with the same UID and image but the wrong deployment role
- **THEN** it receives no matching identity

#### Scenario: Kubernetes executable and deployment binding
- **WHEN** the in-cluster feasibility gate observes actual parent/PID/UID/hash and immutable requesting-container namespace/SA/image/role/owner selectors
- **THEN** only the exact intended app/control IDs issue under their complete conjunction; wrong binary/deployment and adapter app-ID requests fail with explicit denial

#### Scenario: Kubernetes feasibility is not established
- **WHEN** PID/proc, verified kubelet, immutable selector or negative issuance proof is missing
- **THEN** dependent implementation stops; timeout, container-only identity, custom plugins or TLS-verification bypass cannot substitute

### Requirement: WA-03 Same-UID reachability and workload isolation
Both caller app processes SHALL retain UID/GID 1000 in separate containers and, for Kubernetes, separate Pods, with Workload API/Money reachability for controlled denial probes. Apps MUST receive no Docker/admin/private-SPIRE state or foreign keys. Kubernetes control adapters SHALL be separate UID 10002 processes with control-only identities/mounts; shared Pod mesh identity MUST NOT confer app executable identity.

#### Scenario: Both callers can reach the identity and money services
- **WHEN** the measured demo starts with its intended mounts and network
- **THEN** both callers have socket/network reachability and only the eligible caller obtains its authorized identity

#### Scenario: A mount or network prevents the negative probe
- **WHEN** a purported issuance-denial test cannot reach the Workload API or its required network target
- **THEN** it fails as an infrastructure problem rather than passing as executable-attestation proof

#### Scenario: Normal mesh proxy has no application socket or key
- **WHEN** the normal mesh proxy without a CSI/Workload API mount is inspected
- **THEN** its Workload API/application-key absence is recorded structurally, while its own mesh credentials are allowed; missing reachability is not claimed as PermissionDenied issuance proof

#### Scenario: Separate caller Pods with adapter identities
- **WHEN** both caller Pods run app UID 1000 and independently attested UID 10002 adapters
- **THEN** both app probes can reach their required targets, only trusted app gets transfer identity, and adapters/proxies receive no app identity or foreign state/key mount


## Traceability and proposed coverage

All original modified-block scenarios are retained. New Kubernetes scenarios are
NOT RUN. [Profile](../../profile.md), [KB acceptance](../../acceptance.md) and
[references](../../references.md) define the proposed gates. Current source and
main contracts remain unchanged at `98f0fa3`; existing evidence is background,
not Kubernetes proof.
