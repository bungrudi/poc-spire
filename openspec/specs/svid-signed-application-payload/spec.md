# svid-signed-application-payload Specification

## Purpose

Authenticate every workload HTTP request with an X.509-SVID payload signature.
Kubernetes adds SPIRE-backed Envoy TLS; private Compose uses signed HTTP.

## Requirements

### Requirement: SP-01 SVID-signed application payload
In every deployment profile, including Compose and Kubernetes, the body of an identity-bearing REST call SHALL be one compact ES256 JWS signed by the sender's current X.509-SVID. Peers MUST NOT authenticate that call with an application certificate.

#### Scenario: Trusted caller signs a transfer
- **WHEN** the trusted caller submits its fixed transfer in either deployment profile
- **THEN** the payload signature is produced by its current SVID and Money does not require a client certificate

#### Scenario: Unsigned or confused signature
- **WHEN** the payload has no signature, uses `none` or another algorithm, or substitutes an unsigned identity header
- **THEN** Money returns 401 and does not read history or change balances

### Requirement: SP-02 Exact signer authorization
The verifier MUST trust only the independent `poc.internal` bundle, derive the signer from the SVID URI SAN, and authorize that exact ID. Mesh identity and caller-supplied roots MUST NOT grant the effect.

#### Scenario: Permitted trusted signer
- **WHEN** the current trusted-caller SVID signs a transfer and the chain validates
- **THEN** Money authorizes that exact ID for the transfer route

#### Scenario: Valid unauthorized signer
- **WHEN** the UI or another trusted-domain ID produces a valid transfer signature
- **THEN** Money returns 403 and does not change balances

#### Scenario: Forged chain
- **WHEN** a signer presents an untrusted chain that claims the trusted URI and supplies its own root
- **THEN** Money rejects trust and returns 401 without a ledger effect

### Requirement: SP-03 Signed destination binding
The signed payload MUST bind the HTTP method, path and logical request ID. A valid signature on a different method, path or ID MUST be rejected without a ledger effect.

#### Scenario: Bound transfer is accepted
- **WHEN** the signature covers POST `/transfer` and the submitted request ID
- **THEN** destination checks proceed to exact-ID authorization

#### Scenario: Signature is replayed onto another route
- **WHEN** a transfer signature is presented as a balance read or with a different request ID
- **THEN** Money rejects it before history access or mutation

### Requirement: SP-04 Money signs its response
Money's identity-bearing HTTP response MUST be signed by its current SVID. The client MUST accept that response only for signer `spiffe://poc.internal/money`.

#### Scenario: Exact Money response
- **WHEN** Money signs the response for the submitted request ID with its current SVID
- **THEN** the client may treat that response as Money's result

#### Scenario: Impostor response
- **WHEN** another trusted-domain identity signs the response
- **THEN** the client rejects it and does not report a successful transfer

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

### Requirement: SP-08 Tampered REST payload
The verifier MUST check the signature over the exact received protected-header and payload bytes. Changed bytes without a new valid signature MUST fail. Re-serialized JSON that is not byte-identical MUST NOT verify.

#### Scenario: Payload bytes are altered
- **WHEN** any embedded payload byte, including the transfer body, changes and the original signature remains
- **THEN** Money returns 401 and does not read history or change balances

#### Scenario: Protected header bytes are altered
- **WHEN** a protected-header byte changes without a new valid signature
- **THEN** verification fails even if the decoded JSON would look equivalent

#### Scenario: Money response bytes are altered
- **WHEN** a client receives a Money response whose signed bytes differ from the verified input
- **THEN** the client rejects it and does not report a successful transfer

### Requirement: SP-09 Wrong REST signature
A signature segment that does not verify the intact signing input under the presented SVID public key MUST be authentication failure. That result MUST stay distinct from a signature that does verify for an unauthorized SPIFFE ID.

#### Scenario: Signature bytes do not verify
- **WHEN** the protected header and payload are unchanged and the signature segment does not verify
- **THEN** Money returns 401 with no ledger effect

#### Scenario: Verifying signature from another ID
- **WHEN** another trusted-domain ID produces a transfer signature that does verify
- **THEN** Money returns 403 and does not classify that result as a bad signature

### Requirement: SP-10 Obsolete REST signature
A signature that has already verified against a trusted chain MUST be rejected when the certificate is expired or not yet valid, or when the signed envelope has elapsed. Future skew is five seconds and there is no grace. A previous SVID still inside its certificate lifetime and unelapsed envelope MUST verify.

#### Scenario: Certificate is expired or not yet valid
- **WHEN** the signing certificate is outside its validity, beyond five seconds of future skew
- **THEN** Money returns 403 `credential_not_current` and does not treat it as a bad signature

#### Scenario: Envelope has elapsed
- **WHEN** the signature verifies and the certificate is unexpired, but `expires_at` is not after now or is more than 30 seconds after `issued_at`
- **THEN** Money returns 403 `credential_not_current` with no ledger effect

#### Scenario: Clock boundary
- **WHEN** `issued_at` is five seconds ahead with an otherwise current certificate, or six seconds ahead
- **THEN** the first proceeds to exact-ID authorization and the second returns 403 `credential_not_current`

#### Scenario: Previous SVID is still inside the window
- **WHEN** a rotated-away SVID is still inside `notBefore` and `notAfter`, and its envelope has not elapsed
- **THEN** Money verifies that signature and continues to exact-ID authorization

#### Scenario: Bad signature wins over an old certificate
- **WHEN** the signature segment does not verify and the certificate is also expired
- **THEN** Money returns 401 rather than `credential_not_current`

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


## Traceability and limitations

Unit tests in `internal/httpjws` and `internal/money` cover tampered payloads, non-verifying signatures, obsolete envelopes, forged chains, route replay, UI denial and a still-valid previous SVID. Live mesh observation is unrun. Historical double-mTLS evidence does not prove this profile.
