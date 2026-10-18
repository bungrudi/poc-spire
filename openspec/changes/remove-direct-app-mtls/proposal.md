# Proposal

## Why

The application currently has two authentication paths: Compose uses direct
SPIFFE mTLS, while Kubernetes uses SVID-signed HTTP. Rudi requested one simpler
application path on 18 October 2026: remove direct application mTLS and make
Money always verify the payload signature, including an explicit explanation
of how a caller's SVID establishes trust.

## What Changes

- **BREAKING:** all workload HTTP uses the existing compact ES256 JWS profile
  in Compose and Kubernetes. Remove application mTLS listeners, clients,
  certificate-based HTTP authorization and transport-mode switches.
- Money validates the supplied public X.509-SVID chain against its own updating
  SPIRE trust bundle, verifies the signature with the leaf public key, checks
  freshness and route binding, then authorizes the exact SPIFFE URI SAN.
  Caller-supplied roots, identity headers and mesh membership confer no permission.
- Money signs responses; callers and UI verify the exact Money identity and
  matching method/path/request ID. Network control adapters use the same
  signature-only protocol; local UDS controls retain their peer checks.
- An unidentified caller reaches HTTP but is rejected for lacking a valid
  signature. There is no no-client-certificate TLS failure in the application.
- Preserve attestation, rotation, Kafka original-producer JWS, exact permissions,
  the fixed ledger and stable retries. Recheck HTTP freshness under the ledger
  lock before accessing history or mutating balances.
- Kubernetes uses SPIRE-issued proxy SVIDs for Istio outer mTLS through Envoy SDS.
  Proxy identities/keys remain separate from app identities/keys; use mesh trust
  domain `poc.internal` and standard namespace/service-account SPIFFE IDs. Compose uses private-network plaintext
  HTTP carrying signed payloads: authenticity/integrity are provided, confidentiality
  is not. Browser HTTPS and separate Kafka broker TLS/DN admission remain.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `svid-signed-application-payload`: one protocol for every workload HTTP path;
  explicit SVID verification and no application-mTLS fallback.
- `money-transport-authorization`: replace MT-01 direct mTLS with mandatory
  signature verification; make MT-02–MT-04 independent of deployment profile.
- `identity-certificate-lifecycle`: use updating sources for signatures and
  verify signed responses with authentic public trust on unidentified probes.
- `private-caller-controls`: use signed requests/responses and uniform failure
  semantics while preserving private UDS and caller-owned keys.
- `mesh-transport-security`: SPIRE SDS issuance for narrowly registered proxies,
  matching trust domain/policies and registration before proxy readiness.
- `https-demo-ui`: label Sync as signed HTTP and verify all Money outcomes by
  signature, including Compose observer reads.

## Impact

Affected: `internal/{identity,httpjws,money,caller,moneyreader,controladapter}`,
Money/caller/UI/adapter entrypoints, Compose/Kubernetes runners and fixtures,
negative tests, UI wording, RFC/runbooks and current OpenSpec contracts.
Reuse go-spiffe and go-jose; no new issuer, registration controller, custom attestor, JWT-SVID, signing
service, TLS proxy or crypto dependency. Existing Kafka wire format is retained.

This supersedes the Compose-mTLS exception in the prior
`replace-app-mtls-with-svid-signature` change. Historical T02/T06/T07 TLS proofs
remain historical; their replacements must establish signature rejection.
See `docs/spec-baseline.md` and the acceptance matrix in `tasks.md`.
This proposal does not change running services or authorize deployment.
