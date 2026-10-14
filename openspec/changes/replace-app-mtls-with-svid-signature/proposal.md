# Proposal

## Why

The current Kubernetes baseline authenticates workloads twice: Istio/Envoy
does outer mTLS, and the Go application does a second, independent mTLS
handshake with SPIRE X.509-SVIDs (MS-01, MT-01). This change specifies the
requested alternative: the application stops doing mTLS and authenticates
the payload by signing it with its SVID. Istio remains the only TLS layer
on that path.

This is a proposed behavior change. It is not implementation, and it does
not replace the current contracts until Rudi approves the delta and short
plan.

## What Changes

- **BREAKING for the Kubernetes workload HTTP path:** UI, callers and Money
  no longer present or require application client or server certificates.
  Each identity-bearing REST request and response body is one compact JWS
  signed by the sender's current X.509-SVID private key.
- Istio sidecars keep outer mTLS (`STRICT`, `ISTIO_MUTUAL`, TLS 1.3). Envoy
  forwards plaintext application HTTP inside that tunnel. It does not
  terminate an inner TLS session, and mesh identity or forwarded headers
  still do not authorize a transfer.
- Money authorizes the exact SPIFFE ID from the verified signing SVID.
  The trusted caller remains the only transfer signer. The UI remains
  read-only. A valid signature from any other ID is 403 with no ledger
  effect.
- An unidentified caller still receives no SVID. Its mesh admission can
  succeed, and Money then rejects the missing or invalid signature before
  ledger access. That rejection is an application result, not a client-TLS
  failure with no HTTP status.
- Clients accept a Money response only when Money's own SVID signature
  names exactly `spiffe://poc.internal/money`, and only after the same
  payload checks.
- REST verification has three separate negative results. A payload whose
  signed bytes were altered is 401. A signature that does not verify the
  intact input is 401. A signature that verifies but is expired, not yet
  valid, or past its 30-second envelope is 403 `credential_not_current`.
  A previous SVID that is still inside its certificate lifetime and
  envelope still verifies: Money has no TLS peer certificate and no
  newest-key registry.
- Compose paths that have no mesh keep today's direct Money mTLS. The
  application-mTLS drop is the Kubernetes workload HTTP layer, where
  Istio remains the TLS layer. Kafka keeps its existing JWS commands and
  conventional broker TLS. Browser HTTPS, CSRF and same-Pod UDS controls
  stay.

## Capabilities

### New Capabilities

- `svid-signed-application-payload`: SVID JWS authentication for Kubernetes
  application HTTP once application mTLS is removed.

### Modified Capabilities

- `mesh-transport-security`: outer Istio/Envoy mTLS stays; inner application
  TLS on workload HTTP becomes a signed payload.
- `money-transport-authorization`: mesh HTTP verifies the signed SVID
  instead of a client certificate; non-mesh mTLS stays.
- `private-caller-controls`: Kubernetes callers and control adapters use
  the signed payload; the no-SVID probe is signature rejection.
- `https-demo-ui`: mesh observer reads trust Money's response signature
  rather than an application mTLS peer.
- `identity-certificate-lifecycle`: mesh HTTP signs each payload with the
  current SVID instead of opening a fresh application TLS handshake.

## Impact

- Affects Kubernetes Money, UI and caller HTTP, Istio opaque-TCP assumptions,
  and the unidentified-caller outcome on that profile.
- Reuses the existing ES256 / `x5c` / `poc.internal` bundle rules from
  `spiffe-jws-message-signing`. It does not add a JWT-SVID or a new trust
  domain.
- No code, deployment or public-demo cutover is in this change. Evidence for
  the signed-payload profile is unrun. Current double-mTLS evidence remains
  the evidence for the unmodified contracts.
