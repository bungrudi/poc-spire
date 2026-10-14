# Design

## Context

See [proposal.md](proposal.md). Kubernetes workload HTTP is double mTLS today. This change removes that application handshake and signs each REST payload with the current X.509-SVID. Compose keeps direct Money mTLS because it has no Envoy tunnel. Kafka already uses compact ES256 JWS.

## Goals / Non-Goals

**Goals:**

- One TLS layer on Kubernetes workload HTTP: Istio/Envoy outer mTLS.
- Application authentication by a current X.509-SVID signature on the REST payload.
- Separate results for a tampered payload, a non-verifying signature, and an obsolete signature.
- Preserve exact SPIFFE IDs and stable retry IDs.

**Non-Goals:**

- Changing Compose direct mTLS, Kafka broker TLS, browser HTTPS, or UDS controls.
- Introducing JWT-SVIDs, dropping Istio, or cutting the public demo over.
- Implementing this before owner approval.

## Decisions

1. **Reuse X.509-SVID JWS, not a JWT-SVID.** Money already trusts `poc.internal` and ES256/`x5c`. The HTTP `typ` is `application/vnd.poc-spire.http-payload.v1+jws`, so a Kafka command is not an HTTP payload.
2. **The HTTP body is the compact JWS.** Protected headers stay `alg`, `typ` and `x5c`. The JSON payload carries method, path, request ID, issued and expiry times, and the route body. POST `/transfer` embeds the transfer object. GET embeds null. Envoy forwards that body and does not verify it. Compact form max 32768 bytes; decoded payload max 4096.
3. **Three negative classes.** Verification uses the exact received bytes. Altered bytes or a non-verifying signature are 401. A verified signature that is expired, not yet valid, or past its 30-second envelope is 403 `credential_not_current`. A verifying unauthorized ID stays 403 without that reason. Five-second future skew and no grace match Kafka. A bad signature stays 401 even when the certificate is also expired.
4. **A previous SVID can still verify.** Money sees `x5c` and the CA bundle, not a TLS peer certificate or a newest-key list. A rotated-away certificate inside `notBefore`/`notAfter` and an unelapsed envelope is accepted. New signatures use the sender's current SVID. Freshness is checked again before commit.
5. **Sign both directions.** Clients accept a response only from `spiffe://poc.internal/money` after the same three checks. A mesh service-account identity does not satisfy that ID.
6. **Leave Compose on direct mTLS.** The drop is the Kubernetes application layer, where Istio supplies the tunnel.

## Risks / Trade-offs

- [Envoy can read the signed HTTP body] → The app verifies the signature; mesh policy must not authorize from the JWS.
- [Unidentified mesh callers get HTTP 401] → Evidence labels that as signature rejection on this profile.
- [A stolen SVID key can sign until the envelope ends] → Same key-compromise limit as an SVID, bounded by 30 seconds. This is not continuous process integrity.

## Migration Plan

Do not deploy or edit runtime code in this change. After approval, implement the mesh profile beside Compose mTLS, prove the three negative cases, and keep double mTLS until that evidence exists. Rollback keeps the existing mTLS profile.

## Open Questions

None. Compose stays on direct mTLS. Obsolete means certificate time or the 30-second envelope.
