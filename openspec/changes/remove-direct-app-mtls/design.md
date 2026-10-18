# Design

## Context

Reuse `internal/httpjws` everywhere. The owner approved replacing Compose mTLS
and integrating SPIRE SDS on 18 October 2026.

## Goals / Non-Goals

One application HTTP authentication path, explicit SVID trust verification and
safe retries. Preserve browser HTTPS, Kafka broker TLS, local
UDS controls, attestation and the memory-only ledger. No new wire format, durable store or deployment.

## Decisions

1. **Use the existing ES256 compact JWS.** The protected header is `alg`, `typ`,
   `x5c`; `x5c` contains the caller's public leaf certificate and intermediates,
   never its private key. The signed JSON contains method, path, request ID,
   issued/expiry times and body. Retain existing bounds and HTTP/Kafka types.
2. **Money owns its trust.** Money opens `identity.Source` against its local
   Workload API. The updating `X509Source` supplies Money's own SVID/key for
   responses and its independent `poc.internal` CA bundle for validation.
   `httpjws.Verify` uses go-jose to verify the exact received bytes with the
   leaf public key and `x509svid.Verify` to validate the X.509-SVID path against
   that bundle and extract the SPIFFE URI SAN. Neither check alone authenticates
   the caller; both must succeed. A root supplied in `x5c` never becomes trusted.
   No per-request issuer lookup is needed.
3. **Authorize after verification.** Validate signing usage/curve, certificate
   and envelope time, method/path and request ID; then require the exact trusted
   ID for transfers or UI ID for reads. Check freshness again under the ledger
   lock before history/effects. Keep 30-second envelopes, five-second future
   timestamp skew, no expiration grace, and identity/request-ID deduplication.
4. **Sign responses too.** Reuse current response verification for exact Money
   or adapter identity and request correlation. Remove the direct workload TLS
   functions, legacy handlers and mTLS/signed-http switches. UI control topology
   still selects local UDS or the single signed remote protocol.

5. **SPIRE also issues mesh SVIDs.** Configure Istio's existing Envoy SDS
   integration and CSI socket mount, trust domain `poc.internal`, and matching
   policy principals. Register only actual proxy containers (UID 1337, pinned
   image, namespace/SA, Pod UID/run labels and attested node parent). Mesh IDs
   use `/ns/<namespace>/sa/<service-account>`; application IDs/keys stay separate.
   Register before waiting for proxy/app readiness, including the native-sidecar
   Kafka admin Job. Keep Istiod for discovery/policy, not workload issuance.
   No broad registration controller or shared-cluster control-plane mutation.

## Risks / Trade-offs

- Compose signatures do not encrypt HTTP; retain private, unpublished service
  ports. Kubernetes retains the Istio tunnel. No new TLS proxy is added.
- Existing HTTP envelopes omit recipient identity and response status. This
  change preserves that wire format; it does not claim those fields are signed.
- Rotation is not instant revocation. A stolen still-valid key can sign; host,
  SPIRE and authorized-process compromise remain outside the PoC guarantee.

## Migration Plan

After approval, update callers/readers, Money and adapters together; remove
workload transport flags and change Compose URLs to HTTP. Migrate legacy TLS
fixtures/assertions to signature tests, retain separate browser/broker TLS
tests, then run focused deterministic checks and isolated SPIRE acceptance if available. Reconcile current contracts/runbooks; preserve historical evidence. A fresh disposable run is
required for acceptance; retained issuer state is not reset. No live rollout
is included; rollback is a source-version rollback, not a retained mode switch.
