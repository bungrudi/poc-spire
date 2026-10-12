# Private caller controls

## Purpose

Define the browser-to-caller trigger boundary, caller-owned workload connections and truthful structured outcomes while preventing an arbitrary network-request deputy.

## Requirements

### Requirement: PC-01 Separate private trigger channels
Each caller SHALL listen only on its own Unix-domain control socket, with no network trigger endpoint. UI SHALL have separate mounts for both channels; the unidentified caller MUST NOT mount or reach the trusted channel. The public profile SHALL combine private mounts with peer-UID checking, not treat shared UID alone as workload identity.

#### Scenario: UI triggers the selected caller
- **WHEN** the UI sends a command over the selected caller's separately mounted socket
- **THEN** only that caller executes the attempt

#### Scenario: Unidentified caller targets trusted control
- **WHEN** the unidentified container attempts to open the trusted control path or reach a network trigger
- **THEN** no reachable trusted control endpoint exists and no trusted attempt is dispatched

### Requirement: PC-02 Fixed bounded peer-checked protocol
Existing POST /attempt SHALL accept exactly one request_id matching [A-Za-z0-9_-]{1,128}, strict JSON with no unknown/trailing fields, at most 1024 body bytes and a kernel peer UID matching configured operator UID. Separate opt-in Kafka routes SHALL use their fixed schema on the same private boundary. Routes MUST NOT accept arbitrary destinations, payloads, amounts, SPIFFE identities or headers.

#### Scenario: Valid command from configured peer
- **WHEN** the configured peer UID sends a valid request ID to its private socket
- **THEN** the fixed executor runs within a bounded deadline and concurrency limit

#### Scenario: Wrong peer or arbitrary-request input
- **WHEN** the peer UID is absent/wrong or the command supplies extra fields, invalid IDs or oversized input
- **THEN** control rejects the peer with 403 or malformed command with 400 before executing

### Requirement: PC-03 Caller-owned authentication and no-SVID probe
The existing private-control caller profile MUST continue obtaining its exact trusted SVID and making fixed direct mTLS transfers. Unidentified Sync attempts SHALL require reachable no-identity denial, then a verified-Money no-certificate probe. Unexpected issuance/HTTP response MUST fail. Opt-in fixed Kafka controls SHALL preserve those Sync commands while publishing trusted/unsigned/tampered scenarios; UI MUST NOT receive producer keys.

#### Scenario: Trusted and unidentified attempts
- **WHEN** each caller executes its own fixed attempt in the measured deployment
- **THEN** trusted succeeds with verified Money evidence and unidentified reports no matching identity followed by TLS rejection without a Money HTTP status or mutation

#### Scenario: Workload API is broken or unexpectedly issues identity
- **WHEN** the API is unreachable, reports another failure or issues an identity to the unidentified caller
- **THEN** the attempt reports infrastructure failure or unexpected_identity_issued rather than claiming no-SVID rejection

#### Scenario: Fixed Kafka publish alongside Sync controls
- **WHEN** the combined profile enables its separate fixed Kafka control route
- **THEN** callers publish only their permitted menu cases, while existing POST /attempt and Sync behavior remain unchanged

### Requirement: PC-04 Stable retry and distinct outcome provenance
Sync results SHALL retain submitted IDs and distinct Money/TLS/business/infrastructure outcomes. Kafka controls SHALL return only bounded descriptor/digest/publication diagnostics, never infer Money success from broker acknowledgement. Lost/malformed responses MUST remain unknown until same-ID recovery. Only authenticated correlated Money receipts SHALL settle Kafka effects; diagnostics MUST NOT invent a verified signer or Money status.

#### Scenario: Stable successful response
- **WHEN** the caller receives a valid Money result for the submitted request ID
- **THEN** the control response includes that ID, outcome, actual Money status and supported verified-server/result evidence

#### Scenario: Response is lost or cannot be correlated
- **WHEN** control loses a response or receives malformed/oversized evidence or a mismatched request ID
- **THEN** it reports unknown_commit, preserves the ID and supplies no invented Money success

### Requirement: PC-05 Fixed private Kafka controls
Opt-in callers SHALL add POST /attempt/kafka with exactly request_id, scenario and a 32-hex authenticated-Money instance nonce, plus GET /attempt/kafka/{request_id} for metadata. Trusted SHALL support only kafka-trusted/kafka-tampered; unidentified SHALL support only kafka-unidentified. Peer checking, 1024-byte input, strict parsing and bounded deadlines MUST apply. No network control listener or signing-key export is allowed.

#### Scenario: Fixed Kafka command is published
- **WHEN** an authorized UI peer submits a valid fixed scenario/ID/instance tuple on the corresponding private socket
- **THEN** the caller reserves exact record metadata before publishing and reports queued or publish_unknown independently of Money effects

#### Scenario: Wrong scenario peer or arbitrary payload
- **WHEN** a peer is unauthorized or sends another caller's scenario, extra fields, oversized data or selectable broker/amount/identity
- **THEN** the command is rejected before publication and the old POST /attempt contract remains unchanged

### Requirement: PC-06 Initialized bounded retry state
The runner SHALL initialize one stream-bound caller manifest/index before startup. Caller-only state SHALL retain at most 64 immutable descriptors and four record variants each, atomically before publication; callers MUST NOT recreate missing/corrupt expected state. Retry/UI or caller restart MUST preserve original deadline, instance nonce and potentially submitted digests. New IDs at capacity SHALL fail without eviction.

#### Scenario: Lost publish response then restart
- **WHEN** publication may have happened but its control response is lost and UI/caller container restarts in the same run
- **THEN** initialized state recovers the exact descriptor/all potentially submitted digests without a new ID/deadline

#### Scenario: State loss or descriptor expiry
- **WHEN** expected state is missing/corrupt, stream/instance differs, or the original deadline expired
- **THEN** Kafka publish fails closed as unknown/reset/expired instead of creating a fresh descriptor; metadata lookup can still recover surviving evidence

## Traceability and limitations

See [PC-01–PC-04 sources, checks and evidence](../../../docs/spec-baseline.md#private-caller-controls). The public UI and callers share UID 1000; private mount separation is essential. The historical M4 harness uses a distinct operator UID to exercise the additional peer check. A compromised UI can exercise its legitimate trigger capability; the protocol does not authenticate the browser human.

The approved opt-in Kafka extension is mapped in [JS coverage](../../../docs/kafka-signing-acceptance.md). Its full wire/receipt/retry rules are in the [stable profile](../../../docs/kafka-signing-profile.md).
