# Design

## Context

See [proposal.md](proposal.md) for why. Today the Kubernetes path is double
mTLS: Envoy uses the Istiod CA, and Go terminates a second SPIRE mTLS session.
Compose has no mesh, so its only workload TLS is that application session.
Signing already exists for Kafka as compact ES256 JWS with an `x5c` SVID.

## Goals / Non-Goals

**Goals:**

- One TLS layer on Kubernetes workload HTTP: Istio/Envoy outer mTLS.
- Application authentication by a current X.509-SVID signature on the payload.
- Preserve exact SPIFFE IDs, distinct denial classes, and stable retry IDs.

**Non-Goals:**

- Changing Compose direct mTLS, Kafka broker TLS, browser HTTPS, or UDS controls.
- Introducing JWT-SVIDs, dropping Istio, or cutting the public demo over.
- Implementing this before owner approval.

## Decisions

1. **Reuse X.509-SVID JWS, not a JWT-SVID.** Money already trusts `poc.internal`
   and ES256/`x5c`. A JWT-SVID would add a second credential and verifier.
2. **Plaintext HTTP inside the mesh tunnel.** Server-only application TLS would
   still be application TLS. Envoy supplies confidentiality and pod identity.
   Envoy does not verify the JWS or accept identity headers.
3. **Sign both directions.** The request signature authenticates the caller.
   Money's response signature replaces the client's TLS check for
   `spiffe://poc.internal/money`. Mesh service-account identity is not that ID.
4. **Keep denial classes apart.** No outer connection means no Money HTTP
   status. A missing or invalid signature is 401. A valid unauthorized signer
   is 403. Expiry before commit stays `credential_not_current`.
5. **Leave Compose on direct mTLS.** Without Envoy, removing application mTLS
   would drop the only transport protection.

## Risks / Trade-offs

- [Envoy can read the signed HTTP body] → The app still verifies the signature;
  mesh policy must not treat the JWS as authorization.
- [Unidentified mesh callers now get HTTP 401 instead of a TLS failure] → UI
  and caller evidence label that as signature rejection, only on this profile.
- [A stolen SVID key can sign without a live handshake] → Same key-compromise
  limit as today's SVID. Expiry is checked again before commit. This is not
  continuous process integrity.

## Migration Plan

Do not deploy or edit runtime code in this change. After approval, implement
the mesh profile beside Compose mTLS, prove the new negatives, and keep the
current double-mTLS path until that evidence exists. Rollback is to keep
serving the existing mTLS profile.

## Open Questions

None. The profile split, signature type, and status classes above are the
proposed contract.
