# Spec Delta: private-caller-controls

## MODIFIED Requirements

### Requirement: PC-03 Caller-owned authentication and no-SVID probe
The existing private-control caller profile MUST continue obtaining its exact trusted SVID and making fixed direct mTLS transfers. The unidentified caller SHALL require reachable Workload API no-matching-identity denial, then probe without a client certificate while verifying Money. Unexpected issuance or Money HTTP response MUST be flagged. Explicit Kafka signing mode in the approved trusted executable MUST NOT alter default controls or public triggers.

#### Scenario: Trusted and unidentified attempts
- **WHEN** each caller executes its own fixed attempt in the measured deployment
- **THEN** trusted succeeds with verified Money evidence and unidentified reports no matching identity followed by TLS rejection without a Money HTTP status or mutation

#### Scenario: Workload API is broken or unexpectedly issues identity
- **WHEN** the API is unreachable, reports another failure or issues an identity to the unidentified caller
- **THEN** the attempt reports infrastructure failure or unexpected_identity_issued rather than claiming no-SVID rejection

#### Scenario: Explicit one-shot Kafka signing mode
- **WHEN** the owner-approved fixture selects Kafka mode with an immutable command descriptor
- **THEN** the same narrowly attested trusted executable signs/publishes its fixed command, while default UDS callers and browser triggers retain their synchronous path

## Traceability and proposed coverage

All original scenarios are retained. Existing named checks/evidence are mapped in
[the baseline index](../../../../../docs/spec-baseline.md). New cross-ingress,
Kafka and lifecycle assertions are proposed in [acceptance.md](../../acceptance.md),
not executed passes. The main baseline spec is unchanged by this draft.
