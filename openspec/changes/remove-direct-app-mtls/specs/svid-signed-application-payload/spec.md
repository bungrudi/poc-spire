# Spec Delta

## MODIFIED Requirements

### Requirement: SP-01 SVID-signed application payload
In every deployment profile, including Compose and Kubernetes, the body of an identity-bearing REST call SHALL be one compact ES256 JWS signed by the sender's current X.509-SVID. Peers MUST NOT authenticate that call with an application certificate.

#### Scenario: Trusted caller signs a transfer
- **WHEN** the trusted caller submits its fixed transfer in either deployment profile
- **THEN** the payload signature is produced by its current SVID and Money does not require a client certificate

#### Scenario: Unsigned or confused signature
- **WHEN** the payload has no signature, uses `none` or another algorithm, or substitutes an unsigned identity header
- **THEN** Money returns 401 and does not read history or change balances

### Requirement: SP-05 Distinct mesh and signature failures
Mesh denial, signature authentication failure, authorized-identity denial and infrastructure failure MUST remain distinct. A caller with no signing SVID that reaches HTTP MUST NOT be reported as a client-certificate TLS failure, in Compose or Kubernetes.

#### Scenario: No SVID after mesh admission
- **WHEN** the unidentified caller reaches Money through outer mesh mTLS and presents no valid signature
- **THEN** Money returns 401 with no HTTP-less TLS classification and no ledger effect

#### Scenario: Outer mesh denies the pod
- **WHEN** Istio/Envoy rejects the connection before the application receives it
- **THEN** Money produces no HTTP status and no ledger effect

#### Scenario: No SVID on Compose
- **WHEN** the unidentified caller reaches Money over the private Compose network without a valid payload signature
- **THEN** Money returns 401 without history access or a ledger effect, using the same signature authentication rules as Kubernetes

### Requirement: SP-06 Current signer and stable retry
Each new payload MUST be signed with the sender's current SVID. Money MUST recheck certificate and envelope freshness within the atomic ledger decision before history access or mutation. Retry of an unknown result MUST keep the original logical request ID and MUST NOT allocate a fresh transfer ID.

#### Scenario: Credential expires before commit
- **WHEN** the signing SVID expires after parsing and before ledger mutation
- **THEN** Money returns 403 `credential_not_current` without a ledger effect

#### Scenario: Unknown result is retried
- **WHEN** the caller retries an unknown transfer result
- **THEN** it re-signs the same logical request ID with its current SVID and Money deduplicates that ID

#### Scenario: Credential expires while waiting for the ledger lock
- **WHEN** a valid request waits for the ledger lock until its SVID or envelope expires
- **THEN** Money returns 403 `credential_not_current` without reading history, recording idempotency state or mutating balances

### Requirement: SP-07 REST JWS envelope
Every workload HTTP body SHALL be one compact ES256 JWS whose protected header is exactly `alg`, `typ` and `x5c`. `typ` SHALL be `application/vnd.poc-spire.http-payload.v1+jws`. The payload SHALL be one JSON object with exactly `method`, `path`, `request_id`, `issued_at`, `expires_at` and `body`. The compact form MUST be at most 32768 bytes and the decoded payload at most 4096 bytes.

#### Scenario: Transfer object is embedded
- **WHEN** the trusted caller signs POST `/transfer`
- **THEN** `body` is the bounded transfer object, `request_id` equals that object's request ID, and the compact JWS is the HTTP body

#### Scenario: Read has a null body
- **WHEN** the UI signs GET `/balances` or an enabled Kafka outcome read
- **THEN** `body` is null and the compact JWS is still the HTTP body

#### Scenario: Wrong wire form
- **WHEN** the body is detached, uses another `typ`, adds a protected header, or exceeds 32768 compact bytes or 4096 decoded payload bytes
- **THEN** Money returns 400 and does not verify a substituted form or touch the ledger

## ADDED Requirements

### Requirement: SP-11 Independent SVID chain and payload verification
Money MUST authenticate a signed payload only after both its ES256 signature and X.509-SVID certificate path validate. The leaf public key MUST verify the exact JWS signing input. The path MUST anchor in Money's own updating `poc.internal` trust bundle; the signer identity MUST come from the verified leaf URI SAN. A supplied certificate or root MUST NOT become a trust anchor merely by appearing in `x5c`.

#### Scenario: Caller supplies public signing material
- **WHEN** the caller sends compact JWS with its public leaf SVID and any intermediates in protected `x5c`
- **THEN** Money independently verifies the chain, signing usage, signature and identity without receiving the caller's private key or making a per-message SPIRE lookup

#### Scenario: Attacker supplies a certificate with the expected identity
- **WHEN** an attacker signs with its own key and includes its own root and a certificate claiming the trusted SPIFFE ID
- **THEN** Money returns 401 because that chain is not trusted by Money, even if the mathematical signature verifies

#### Scenario: Genuine certificate but another signing key
- **WHEN** a payload includes a genuine caller SVID but was signed by a different private key
- **THEN** Money returns 401 without any ledger effect

#### Scenario: Trust cannot be obtained
- **WHEN** Money has no usable independent bundle
- **THEN** it fails closed with trust unavailable and never trusts caller-supplied roots or substitutes mesh identity

### Requirement: SP-12 Single application protocol
Money, caller, UI observer and remote control HTTP SHALL support only signed payload authentication in all deployment profiles. Application mTLS listeners, certificate-based HTTP authorization and mTLS transport selection MUST be removed. Browser HTTPS, separate Kafka broker TLS and Kubernetes outer mesh TLS remain distinct. Compose workload HTTP SHALL stay on its private unpublished network and MUST NOT claim payload encryption.

#### Scenario: Legacy transport option is supplied
- **WHEN** configuration attempts to select a former application mTLS mode
- **THEN** startup fails as unsupported configuration rather than enabling a second authentication path

#### Scenario: Default startup and unsigned request
- **WHEN** application binaries start without transport selection and an unsigned transfer reaches Money
- **THEN** signed HTTP is the only path and Money rejects the request with 401

#### Scenario: External transport layers remain independent
- **WHEN** the demo serves browser HTTPS, uses Kafka TLS or runs inside Istio
- **THEN** those transport protections do not bypass Money's payload verification or exact signer authorization

## Traceability

See [current payload contract](../../../../specs/svid-signed-application-payload/spec.md), `internal/httpjws`, `internal/money` and acceptance S01-S10 in [tasks](../../tasks.md). This proposal has no new runtime evidence.
