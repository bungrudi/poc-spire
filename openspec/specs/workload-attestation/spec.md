# Workload attestation

## Purpose

Define which executable in which deployment can obtain a workload identity in the measured UID-1000/native-agent demo. This is an issuance contract, not continuous runtime integrity.

## Requirements

### Requirement: WA-01 Executable and deployment conjunction
Identity issuance MUST require the exact attested agent parent and the approved UID, executable SHA-256, deployment role and image config digest together. The measured profile SHALL use SPIRE 1.14.0 with built-in Unix and Docker attestors.

#### Scenario: Approved executable in approved deployment
- **WHEN** the requesting executable satisfies every registered condition
- **THEN** it receives its exact registered SPIFFE ID

#### Scenario: Wrong executable under matching deployment conditions
- **WHEN** a distinct executable runs with the same UID, image and deployment role in the controlled trusted fixture
- **THEN** it receives no matching identity despite access to the Workload API

#### Scenario: Approved executable in wrong deployment
- **WHEN** the approved executable runs with the same UID and image but the wrong deployment role
- **THEN** it receives no matching identity

### Requirement: WA-02 Narrow registration and explicit binary updates
Registration setup MUST refuse duplicate, broader, wrong-parent or wrong-lifetime grants for an existing SPIFFE ID. An exact existing entry SHALL be reused without taking ownership of it for cleanup. A rebuilt executable MUST require an explicitly reviewed policy update.

#### Scenario: Exact policy already exists
- **WHEN** setup finds one entry with the intended parent, complete selectors and lifetime
- **THEN** it reuses that entry without broadening it or scheduling its deletion

#### Scenario: Existing policy differs
- **WHEN** an existing grant is broader, duplicated or has a different parent or lifetime
- **THEN** setup fails without silently creating, modifying or deleting policy

#### Scenario: Executable changes
- **WHEN** a rebuilt executable requests identity under the previous digest registration
- **THEN** issuance is denied until a reviewed update changes the digest while preserving the other required boundaries

### Requirement: WA-03 Same-UID reachability and workload isolation
The two demo callers SHALL run in separate containers with equal UID/GID 1000 and access to both the Workload API and Money network. Applications MUST receive no Docker/admin sockets, private SPIRE state or other workloads' keys. UID or container membership alone MUST NOT grant executable identity.

#### Scenario: Both callers can reach the identity and money services
- **WHEN** the measured demo starts with its intended mounts and network
- **THEN** both callers have socket/network reachability and only the eligible caller obtains its authorized identity

#### Scenario: A mount or network prevents the negative probe
- **WHEN** a purported issuance-denial test cannot reach the Workload API or its required network target
- **THEN** it fails as an infrastructure problem rather than passing as executable-attestation proof

### Requirement: WA-04 Evidence scope and provenance
Attestation evidence MUST correlate observed selectors to the requesting executable and deployment, record actual versions/digests, redact bootstrap secrets and treat timeout, incomplete selectors or unexpected issuance as test failures. Successful attestation MUST NOT be described as protection against host/SPIRE/authorized-process compromise.

#### Scenario: Complete measured proof
- **WHEN** all expected selector observations and issuance/denial outcomes are present
- **THEN** evidence records the measured layout and bounded executable/deployment claim

#### Scenario: Missing observations or infrastructure failure
- **WHEN** observation is missing, a digest differs, bootstrap times out or an ineligible executable receives identity
- **THEN** the suite fails without exposing join-token arguments or claiming security rejection

## Traceability and limitations

See [WA-01–WA-04 sources, checks and evidence](../../../docs/spec-baseline.md#workload-attestation). Native Linux UID 1000 was measured; UID 10001, rootless Docker, Docker Desktop and a containerized agent are unverified. Test-only co-location demonstrates issuance filtering, not safe isolation from same-user memory/key theft.
