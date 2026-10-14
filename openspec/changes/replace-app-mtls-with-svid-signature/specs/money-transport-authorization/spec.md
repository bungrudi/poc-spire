# Spec Delta: money-transport-authorization

## MODIFIED Requirements

### Requirement: MT-01 Mandatory direct workload mTLS
Non-mesh HTTP routes SHALL keep direct Money mTLS. Money terminates caller TLS inside Go, requires a trusted X.509-SVID, uses SPIFFE verification at TLS 1.3 or later, and MUST NOT accept plaintext, optional client certificates, forwarded identity or an application verification bypass. On the Kubernetes mesh profile, those routes MUST NOT use application mTLS and MUST authenticate the SVID-signed payload instead. Kafka ingestion remains separate.

#### Scenario: Trusted caller connects directly
- **WHEN** a non-mesh caller presents a valid current SVID from the accepted trust domain
- **THEN** Money authenticates its workload identity during the direct TLS handshake

#### Scenario: No certificate with forged identity headers
- **WHEN** a non-mesh unidentified peer claims the trusted identity in headers without a client SVID
- **THEN** TLS fails before the HTTP handler and no Money HTTP status or ledger mutation exists

#### Scenario: Self-signed certificate claims an allowed URI
- **WHEN** a non-mesh peer presents an untrusted certificate containing the trusted SPIFFE URI
- **THEN** TLS rejects it and balances remain unchanged

#### Scenario: Mesh caller signs instead of presenting a certificate
- **WHEN** a Kubernetes mesh caller sends the signed transfer payload and no application client certificate
- **THEN** Money authenticates the signer from the payload and does not require a TLS client certificate

### Requirement: MT-02 Exact identity method and route policy
For HTTP, Money MUST authorize the exact verified ID, method and path before history. Only `spiffe://poc.internal/caller/trusted` SHALL POST `/transfer`. Only `spiffe://poc.internal/ui` SHALL GET `/balances` and the enabled Kafka outcome reads. Other authenticated combinations MUST receive 403. Non-mesh identity comes from the TLS peer. Mesh identity comes from the verified SVID signature. Kafka remains separately signed.

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
Every non-mesh workload client of Money's HTTP API MUST verify server ID `spiffe://poc.internal/money` during TLS. On the Kubernetes mesh profile, the client MUST instead verify that ID from Money's response signature and MUST NOT require an application server certificate. Trust-domain membership, DNS or a supplied identity string alone MUST NOT authorize the server. Kafka broker TLS stays separate.

#### Scenario: Expected Money identity
- **WHEN** non-mesh Money presents a trusted valid SVID with the exact expected ID
- **THEN** the client may send its application request over the verified connection

#### Scenario: Trusted-domain impostor
- **WHEN** an impostor server presents a valid SVID with a different SPIFFE ID
- **THEN** the non-mesh client rejects server verification without a successful transfer

#### Scenario: Mesh client checks the response signer
- **WHEN** a mesh client receives a Money response
- **THEN** it accepts the response only when the signature signer is exactly `spiffe://poc.internal/money`

### Requirement: MT-04 Bounded input and current credential guards
For POST `/transfer`, after authorization, Money MUST accept one bounded transfer object with no unknown fields or trailing values and a maximum 4096-byte body. It MUST check credential validity before authorization and again after body parsing. Requests crossing expiry MUST NOT mutate the ledger. Non-mesh validity is the TLS peer certificate. Mesh validity is the signing SVID and its 30-second envelope. On mesh, 4096 applies to the decoded JWS payload. Kafka keeps its separate limits.

#### Scenario: Current credential with valid bounded body
- **WHEN** an authorized request supplies the fixed transfer before credential expiry
- **THEN** Money passes the parsed request to the ledger

#### Scenario: Invalid JSON or oversized body
- **WHEN** an authorized request has unknown fields, trailing JSON, malformed content or an oversized body
- **THEN** Money returns 400 without mutation

#### Scenario: Credential expires while reading
- **WHEN** a non-mesh certificate or mesh signing SVID was valid at receipt but expires before post-parse authorization completes
- **THEN** Money returns 403 `credential_not_current` without ledger access

#### Scenario: Mesh signature is tampered wrong or obsolete
- **WHEN** a mesh transfer payload is altered, its signature does not verify, or the verified signature is outside certificate or envelope time
- **THEN** altered bytes and a non-verifying signature return 401, and an obsolete verified signature returns 403 `credential_not_current`

## Traceability

Non-mesh scenarios keep the recorded
[MT-01–MT-04](../../../../docs/spec-baseline.md#money-transport-and-authorization)
proof. Mesh signature scenarios are unrun.
