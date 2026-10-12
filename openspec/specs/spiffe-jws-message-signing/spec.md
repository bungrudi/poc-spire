# spiffe-jws-message-signing

## Purpose

Authenticate the original workload producer of a queued fixed-transfer command
using standard JWS and independently validated X.509-SVIDs at Money acceptance.

## Requirements

### Requirement: SJ-01 Single protected ES256 JWS
Commands MUST use one compact RFC 7515 JWS with ES256, an embedded payload and exactly protected alg, typ and x5c headers as defined in the v1 profile. Verification MUST use the exact received signing input. Unsigned, detached, JSON-serialized, compressed or alternate-algorithm forms MUST be rejected without effects.

#### Scenario: Trusted standard-encoded command
- **WHEN** a valid P-256 signer produces the v1 compact representation
- **THEN** its standard 64-byte R||S signature can be verified over the exact encoded header and payload

#### Scenario: Signature stripped or algorithm confused
- **WHEN** the signature is absent, alg is none/HS256/another curve, signature is DER, or an unsupported critical/key-location header is present
- **THEN** Money rejects the command without unsigned fallback, remote key lookup or ledger access

### Requirement: SJ-02 Bounded unambiguous envelope parsing
Money MUST enforce the v1 value/header/payload/chain/key limits before expensive processing and reject malformed or ambiguous encodings, duplicate decoded JSON names, unknown fields, wrong types, invalid Unicode and trailing values. Kafka headers MUST NOT supply security context or identity.

#### Scenario: Duplicate or oversized input
- **WHEN** a command exceeds a profile bound or contains duplicate/escape-equivalent fields, nested ambiguity or trailing JSON
- **THEN** Money reports malformed input with no business/history access

#### Scenario: Altered protected bytes or body
- **WHEN** signed header/payload bytes are changed without a valid new signature
- **THEN** signature verification fails regardless of equivalent re-serialized JSON

### Requirement: SJ-03 Independently trusted original signer
Money MUST validate the signing X.509-SVID at current time against an independently obtained poc.internal bundle, derive the producer ID from its single URI SAN, verify digitalSignature/non-CA/P-256 constraints and JWS key possession, then authorize exactly spiffe://poc.internal/caller/trusted. Supplied roots, broker principals and claimed IDs MUST NOT grant permission.

#### Scenario: Wrong executable or no eligible SVID
- **WHEN** a same-UID wrong executable/deployment requests a signing SVID
- **THEN** issuance is denied and no valid trusted command is produced; API/network failure is not recorded as attestation rejection

#### Scenario: Valid unauthorized SPIFFE identity
- **WHEN** a command is correctly signed by a trusted-domain UI or test identity
- **THEN** Money authenticates that signer but denies its transfer permission without history access or mutation

#### Scenario: Fake root with the correct claimed ID
- **WHEN** a peer signs with an untrusted chain bearing the permitted URI and includes its own root
- **THEN** Money rejects chain trust and does not treat the supplied root or URI as authority

### Requirement: SJ-04 Signed destination and record binding
Money MUST require the v1 signed environment, stream, topic, audience, version and command type to match its immutable configured policy. Actual Kafka topic and key MUST match the signed topic and request_id; unsupported headers MUST be rejected. Broker-assigned offset/partition/timestamp MUST NOT be treated as signed producer authority.

#### Scenario: Correct target and key
- **WHEN** a valid command arrives on its configured topic with the exact signed request_id key
- **THEN** destination verification permits further validation without trusting broker identity headers

#### Scenario: Altered type destination or record key
- **WHEN** type/environment/stream/audience/topic differs from policy, a valid envelope is copied to another topic, or its Kafka key is changed
- **THEN** Money rejects it without effects even if the remaining signature is valid

### Requirement: SJ-05 Current validity at the commit boundary
Money MUST enforce a maximum 30-second envelope lifetime, original command deadline at most 120 seconds after creation, five-second future-issued clock tolerance and no expiry/certificate grace. Each bounded attempt MUST use one independently obtained local trust/policy snapshot, with time/path-validity guards before Kafka ledger access. Claimed signing time MUST NOT rescue expired trust.

#### Scenario: Delayed delivery or near expiry
- **WHEN** a queued command or signing path expires before the guarded ledger lookup/mutation
- **THEN** Money rejects it without inserting history or changing balances, including when it passed earlier validation

#### Scenario: Clock boundary
- **WHEN** issued_at is at most five seconds in the future with otherwise current trust, or lies beyond that tolerance
- **THEN** the first can proceed and the latter is rejected; exact expiry and not-yet-valid certificate boundaries receive no grace

#### Scenario: Bundle update snapshot boundary
- **WHEN** the updating source changes after a bounded attempt captures its independent local bundle snapshot
- **THEN** that snapshot governs the attempt and the new bundle governs subsequent verification attempts, without a claim of instantaneous revocation

### Requirement: SJ-06 Atomic rotation and stable re-signing
The producer MUST capture the matched SVID chain/key atomically, fail closed on incompatible keys and retain original producer ID, request_id, context/creation/deadline and business fields for retry. Re-signing SHALL change only certificate/signature and envelope issued_at/expires_at within the original deadline. Unknown delivery MUST NOT allocate a new logical command ID.

#### Scenario: Rotation followed by a retry
- **WHEN** a trusted producer rotates and re-signs an unresolved command within its original deadline
- **THEN** the new valid signature carries the same logical content and Money deduplicates it under the same producer/request_id

#### Scenario: Missing trust or expiring source
- **WHEN** a matched current SVID lacks sufficient remaining lifetime or no independent accepted bundle snapshot is available
- **THEN** publication or consumption reports bounded infrastructure/unavailable status without unsigned fallback; available cached trust can validate current credentials during outage

### Requirement: SJ-07 Interoperable bounded evidence
Acceptance MUST include RFC7515 ES256 vectors and bidirectional checks with an independent JOSE implementation. Evidence MUST distinguish signature/trust failure, unauthorized signer, freshness/destination rejection and infrastructure failure, without exporting complete JWS/SVIDs or secrets. Provenance MUST NOT be described as confidentiality, durable replay protection or continuous executable integrity.

#### Scenario: Independent implementation verifies exact bytes
- **WHEN** Go and an independently pinned JOSE implementation exchange v1 envelopes and standard ES256 vectors
- **THEN** both verify compatible signatures, reject negative vectors and report the correct certificate-bound original ID separately from broker admission

## Traceability and limits

See the [normative profile](../../../docs/kafka-signing-profile.md) and
[JS acceptance and evidence](../../../docs/kafka-signing-acceptance.md).
This isolated fake-money profile provides no durable payment replay guarantee
or protection against compromise of the host, SPIRE or an authorized workload.
