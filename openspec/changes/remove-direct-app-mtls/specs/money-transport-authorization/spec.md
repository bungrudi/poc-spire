# Spec Delta

## RENAMED Requirements

- FROM: `### Requirement: MT-01 Mandatory direct workload mTLS`
- TO: `### Requirement: MT-01 Mandatory signed workload requests`

## MODIFIED Requirements

### Requirement: MT-01 Mandatory signed workload requests
Every Money HTTP route SHALL authenticate compact SVID-signed payloads in Compose and Kubernetes. Money MUST validate the signature, independent SPIRE trust and current credential before application authorization. It MUST NOT expose direct workload mTLS, optional client-certificate authentication, unsigned fallback or forwarded-identity authentication. Kafka ingestion SHALL continue validating each original producer JWS.

#### Scenario: Trusted caller connects directly
- **WHEN** a Compose caller sends a valid current SVID-backed signed payload
- **THEN** Money authenticates the signer from its independently trusted SVID and payload signature

#### Scenario: No certificate with forged identity headers
- **WHEN** an unidentified peer claims the trusted identity in headers without a valid payload signature
- **THEN** Money returns HTTP 401 before history access or ledger mutation

#### Scenario: Self-signed certificate claims an allowed URI
- **WHEN** a peer signs a payload with an untrusted certificate containing the trusted SPIFFE URI
- **THEN** Money rejects its chain with HTTP 401 and balances remain unchanged

#### Scenario: Mesh caller signs instead of presenting a certificate
- **WHEN** a Kubernetes mesh caller sends the signed transfer payload and no application client certificate
- **THEN** Money authenticates the signer from the payload and does not require a TLS client certificate

### Requirement: MT-02 Exact identity method and route policy
For HTTP, Money MUST authorize the exact verified ID, method and path before history. Only `spiffe://poc.internal/caller/trusted` SHALL POST `/transfer`. Only `spiffe://poc.internal/ui` SHALL GET `/balances` and the enabled Kafka outcome reads. Other authenticated combinations MUST receive 403. Every HTTP identity comes from the verified SVID signature, independent of transport or deployment. Kafka remains separately signed.

#### Scenario: Allowed transfer and observer read
- **WHEN** the trusted ID posts a valid transfer or the UI ID requests balances
- **THEN** Money permits that identity's specified operation

#### Scenario: Valid unauthorized identity
- **WHEN** a valid wrong-ID SVID or the UI SVID is authenticated and posts a transfer
- **THEN** Money returns 403 without reading transfer history or changing balances

#### Scenario: Authorized identity uses another route
- **WHEN** a trusted caller requests balances or any verified ID uses an unsupported method or path
- **THEN** Money denies the operation before business logic

#### Scenario: Observer reads Kafka receipts
- **WHEN** the UI's exact observer SVID requests enabled Kafka outcome metadata or a digest receipt
- **THEN** Money serves bounded read-only evidence; trusted and wrong-ID clients are denied

### Requirement: MT-03 Clients verify Money's exact identity
Every workload client of Money's HTTP API MUST verify signer `spiffe://poc.internal/money` from Money's response signature, plus the matching method, path and request ID. No application TLS certificate SHALL substitute for response verification. Trust-domain membership, DNS and supplied identity strings alone MUST NOT authorize the server. Kafka broker TLS remains separate.

#### Scenario: Expected Money identity
- **WHEN** Money returns a valid signature from its trusted current SVID with the exact expected ID and request correlation
- **THEN** the client accepts the authenticated response body as Money evidence

#### Scenario: Trusted-domain impostor
- **WHEN** an impostor signs a response using a valid SVID with a different SPIFFE ID
- **THEN** the client rejects the response without claiming success or inferring that the transfer did not commit

#### Scenario: Mesh client checks the response signer
- **WHEN** a mesh client receives a Money response
- **THEN** it accepts the response only when the signature signer is exactly `spiffe://poc.internal/money`

### Requirement: MT-04 Bounded input and current credential guards
Money MUST accept one bounded signed transfer object with no unknown fields or trailing values. The decoded JWS payload maximum is 4096 bytes. It MUST check SVID and envelope freshness before authorization, after parsing and under the ledger lock before history or mutation. Requests crossing expiry MUST NOT affect the ledger. All profiles SHALL use the same signature guards; Kafka retains its separate limits.

#### Scenario: Current credential with valid bounded body
- **WHEN** an authorized request supplies the fixed transfer before credential expiry
- **THEN** Money passes the parsed request to the ledger

#### Scenario: Invalid JSON or oversized body
- **WHEN** an authorized request has unknown fields, trailing JSON, malformed content or an oversized body
- **THEN** Money returns 400 without mutation

#### Scenario: Credential expires while reading
- **WHEN** a signing SVID or envelope was valid at receipt but expires before post-parse authorization completes
- **THEN** Money returns 403 `credential_not_current` without ledger access

#### Scenario: Mesh signature is tampered wrong or obsolete
- **WHEN** a transfer payload in either profile is altered, its signature does not verify, or the verified signature is outside certificate or envelope time
- **THEN** altered bytes and a non-verifying signature return 401, and an obsolete verified signature returns 403 `credential_not_current`

#### Scenario: Expiry before atomic ledger decision
- **WHEN** the signing credential or envelope expires while the request waits for the ledger lock
- **THEN** Money returns 403 `credential_not_current` without history access, idempotency insertion or balance changes

## Traceability

See [Money baseline](../../../../../docs/spec-baseline.md#money-transport-and-authorization), `internal/money/server_signed.go`, `internal/httpjws/transport.go` and S01-S08 in [tasks](../../tasks.md). T02/T06/T07 TLS evidence is historical; signature-only acceptance remains unrun.
