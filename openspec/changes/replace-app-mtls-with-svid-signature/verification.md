# Verification

Approved revision: `9f048254a4a89d1be80c6dcf7df88c822ad2cb01`.
Implementation followed that revision. Owner message: “Commit push, then apply new spec” on 14 October 2026.

## Ran

- `gofmt` on the edited Go files
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `npm run spec -- validate --specs --strict --no-interactive` (11 passed)
- `npm run spec -- validate replace-app-mtls-with-svid-signature --strict --no-interactive`
- `python3 -m py_compile scripts/kubernetes_application.py scripts/kubernetes_policy.py`

Unit coverage:

- `internal/httpjws`: tampered payload, wrong signature bytes, `none` algorithm, forged chain, expired certificate, not-yet-valid certificate, elapsed and over-wide envelope, five-second clock boundary, bad signature preferred over an expired certificate, previous SVID still inside its window
- `internal/money`: trusted signed transfer, same-ID retry, UI read, UI transfer 403, missing signature 401, tamper 401, wrong signature 401, forged chain 401, route replay 401, obsolete 403, previous SVID accepted

## Unrun

Live Kubernetes/SPIRE/Istio observation of the signed-HTTP profile is unrun. This environment has no kind cluster for that suite. Envoy is kept off the JWS by the existing `tcp-*` service port names and `ISTIO_MUTUAL` opaque TCP policy; that configuration was not re-captured from a running mesh.

## Scope held

Compose direct Money mTLS, Kafka broker TLS, browser HTTPS and same-Pod UDS controls were not switched to signed HTTP.
