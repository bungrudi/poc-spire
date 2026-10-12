# Money transport and authorization

## Purpose

Define direct workload authentication and exact route permissions at the Go Money service, including the distinction between TLS rejection and an authenticated HTTP denial.

## Requirements

### Requirement: MT-01 Mandatory direct workload mTLS
For every existing HTTP route, Money SHALL terminate caller TLS inside the Go service and require a trusted X.509-SVID. It MUST use SPIFFE verification with TLS 1.3 or later and MUST NOT accept plaintext, optional client certificates, forwarded identity or application verification bypasses. Opt-in Kafka ingestion separately validates signed original-producer identity inside Money.

#### Scenario: Trusted caller connects directly
- **WHEN** a caller presents a valid current SVID from the accepted trust domain
- **THEN** Money authenticates its workload identity during the direct TLS handshake

#### Scenario: No certificate with forged identity headers
- **WHEN** an unidentified peer sends a request claiming the trusted identity in headers without a client SVID
- **THEN** TLS fails before the HTTP handler and no Money HTTP status or ledger mutation exists

#### Scenario: Self-signed certificate claims an allowed URI
- **WHEN** a peer presents an untrusted certificate containing the trusted SPIFFE URI
- **THEN** TLS rejects it and balances remain unchanged

### Requirement: MT-02 Exact identity method and route policy
For HTTP, Money MUST authorize exact verified ID/method/path before history. Only spiffe://poc.internal/caller/trusted SHALL POST /transfer. Only spiffe://poc.internal/ui SHALL GET /balances and enabled GET /kafka/outcomes or GET /kafka/outcomes/{64-hex-digest}. Other authenticated HTTP combinations MUST receive 403. Kafka separately verifies its signed producer; observer reads MUST NOT trigger transfer.

#### Scenario: Allowed transfer and observer read
- **WHEN** the trusted ID posts a valid transfer or the UI ID requests balances
- **THEN** Money permits that identity's specified operation

#### Scenario: Valid unauthorized identity
- **WHEN** a valid wrong-ID SVID or the UI SVID posts a transfer
- **THEN** TLS can succeed but Money returns 403 without reading or populating transfer history or changing balances

#### Scenario: Authorized identity uses another route
- **WHEN** a trusted caller requests balances or any verified ID uses an unsupported method/path
- **THEN** Money denies the operation before business logic

#### Scenario: Observer reads Kafka receipts
- **WHEN** the UI's exact observer SVID requests enabled Kafka outcome metadata or a digest receipt
- **THEN** Money serves bounded read-only evidence; trusted/wrong-ID clients are denied and transfer/balance route permissions remain unchanged

### Requirement: MT-03 Clients verify Money's exact identity
Every workload client of Money's HTTP API, including the UI and unidentified no-certificate probe, MUST verify the exact server SPIFFE ID spiffe://poc.internal/money using trusted SPIFFE verification. Trust-domain membership, DNS or a supplied identity string alone MUST NOT authorize the server. Kafka broker TLS admission is separate from Money's original-signer validation.

#### Scenario: Expected Money identity
- **WHEN** the Money server presents a trusted valid SVID with the exact expected ID
- **THEN** the client may send its application request over the verified connection

#### Scenario: Trusted-domain impostor
- **WHEN** an impostor server presents a valid SVID with a different SPIFFE ID
- **THEN** the client rejects server verification without a successful transfer

### Requirement: MT-04 Bounded input and current credential guards
For POST /transfer, after authorization, Money MUST accept one bounded JSON transfer object with no unknown fields or trailing values and a maximum 4096-byte body. It MUST check the peer credential's validity before authorization and again after body parsing. Requests crossing expiry MUST NOT mutate the ledger. Kafka uses its separate signed-message limits and guarded acceptance; these HTTP checks remain unchanged.

#### Scenario: Current credential with valid bounded body
- **WHEN** an authorized request supplies the fixed transfer before credential expiry
- **THEN** Money passes the parsed request to the ledger

#### Scenario: Invalid JSON or oversized body
- **WHEN** an authorized request has unknown fields, trailing JSON, malformed content or an oversized body
- **THEN** Money returns 400 without mutation

#### Scenario: Credential expires while reading
- **WHEN** a certificate was valid at handshake but expires before post-parse authorization completes
- **THEN** Money returns 403 credential_not_current without ledger access

## Traceability and limitations

See [MT-01–MT-04 sources, checks and evidence](../../../docs/spec-baseline.md#money-transport-and-authorization). Handler unit tests use constructed TLS state and do not prove a handshake; the recorded real-SPIRE suite supplies that proof. A post-handshake expiry 403 differs from the no-certificate TLS failure. Browser/Nginx web PKI is separate from workload mTLS.

The approved opt-in Kafka extension is mapped in [JS coverage](../../../docs/kafka-signing-acceptance.md). Its full wire/receipt/retry rules are in the [stable profile](../../../docs/kafka-signing-profile.md).
