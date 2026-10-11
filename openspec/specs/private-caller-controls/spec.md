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
Control SHALL accept only POST /attempt with one JSON request_id matching [A-Za-z0-9_-] of length 1–128, no unknown fields/trailing values and at most 1024 body bytes. The kernel-observed peer UID MUST match the configured operator UID. Control MUST NOT accept caller-selected destinations, amounts, identities or headers.

#### Scenario: Valid command from configured peer
- **WHEN** the configured peer UID sends a valid request ID to its private socket
- **THEN** the fixed executor runs within a bounded deadline and concurrency limit

#### Scenario: Wrong peer or arbitrary-request input
- **WHEN** the peer UID is absent/wrong or the command supplies extra fields, invalid IDs or oversized input
- **THEN** control rejects the peer with 403 or malformed command with 400 before executing

### Requirement: PC-03 Caller-owned authentication and no-SVID probe
The trusted caller MUST obtain its own exact trusted identity and make the fixed direct mTLS transfer. The unidentified caller SHALL require an observed reachable Workload API no-matching-identity denial, then make a no-client-certificate probe verifying Money. Unexpected issuance or any Money HTTP response on that unidentified path MUST be flagged as failure.

#### Scenario: Trusted and unidentified attempts
- **WHEN** each caller executes its own fixed attempt in the measured deployment
- **THEN** trusted succeeds with verified Money evidence and unidentified reports no matching identity followed by TLS rejection without a Money HTTP status or mutation

#### Scenario: Workload API is broken or unexpectedly issues identity
- **WHEN** the API is unreachable, reports another failure or issues an identity to the unidentified caller
- **THEN** the attempt reports infrastructure failure or unexpected_identity_issued rather than claiming no-SVID rejection

### Requirement: PC-04 Stable retry and distinct outcome provenance
Caller/control results SHALL retain the submitted request ID and distinguish transfer success, authenticated denial, business rejection, server-ID mismatch, TLS rejection, infrastructure failure and unknown commit. A lost or malformed post-dispatch response MUST remain unknown until retry with the same ID; diagnostics MUST NOT invent verified Money identity/status.

#### Scenario: Stable successful response
- **WHEN** the caller receives a valid Money result for the submitted request ID
- **THEN** the control response includes that ID, outcome, actual Money status and supported verified-server/result evidence

#### Scenario: Response is lost or cannot be correlated
- **WHEN** control loses a response or receives malformed/oversized evidence or a mismatched request ID
- **THEN** it reports unknown_commit, preserves the ID and supplies no invented Money success

## Traceability and limitations

See [PC-01–PC-04 sources, checks and evidence](../../../docs/spec-baseline.md#private-caller-controls). The public UI and callers share UID 1000; private mount separation is essential. The historical M4 harness uses a distinct operator UID to exercise the additional peer check. A compromised UI can exercise its legitimate trigger capability; the protocol does not authenticate the browser human.
