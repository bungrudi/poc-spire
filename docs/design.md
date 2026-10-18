# Design summary

**Current source: SVID-signed workload HTTP, with SPIRE-issued Envoy certificates for Kubernetes mesh mTLS.** This revision has not been rolled out. Detailed decisions and test contracts are in [RFC 0001](rfcs/0001-process-identity.md) and [OpenSpec](../openspec/changes/remove-direct-app-mtls/).

## Recommendation

Use **separate Docker containers with the same numeric UID** for the two Go callers. Each has a different executable. Combine executable-bound attestation with deployment selectors; do not call Docker labels or UID checks process identity.

Add a controlled wrong-executable test inside the trusted deployment, holding UID and deployment selectors constant. This proves the executable restriction without relying on shared-container isolation as a security boundary. Attestor support and exact selectors must first be verified for a pinned SPIRE release.

## Architecture

```text
Remote browser -- HTTPS + assumed operator session + CSRF --> Go UI
                                                                 |
                                        private, separate caller control sockets
                                              /                  \
                                     trusted Go caller     unidentified Go caller
                                     same UID, SVID        same UID, no SVID
                                              \                  /
                                       signed HTTP to Go Money service
                                       independent SVID chain/signature checks
                                          exact-ID authorization
                                           in-memory fake ledger

SPIRE server <--> SPIRE agent -- Workload API socket --> Go workloads
UI -- its own read-only signed payload --> Money GET /balances
SPIRE agent -- SDS/CSI --> Envoy proxy -- mesh mTLS --> peer Envoy
```

Money validates the public SVID chain from the JWS `x5c` against its own updating
SPIRE bundle, verifies the signature with the leaf public key, checks freshness
and request binding, then authorizes the exact caller ID. It never receives a
caller private key. Callers verify Money's response signature and correlation.
The proxy uses a separate namespace/SA SVID and key; it gets no transfer identity.
Compose uses private signed HTTP without encryption; Kubernetes adds mesh mTLS.

## Workload authorization and the human assumption

1. **Human assumption:** the owner explicitly removed OIDC on 11 October 2026. Anyone can trigger fake transfers with an assumed-operator session. The UI discloses this; HTTPS/CSRF/session controls remain.
2. **Workload gate:** Money requires a valid SVID-backed payload signature and allows only the trusted caller's exact SPIFFE ID to transfer. Freshness is rechecked under the ledger lock.

The UI has only read permission on the money API and private access to caller trigger sockets. It never receives the trusted caller's SVID. Callers have no network trigger endpoint; the unidentified caller cannot access the trusted control socket.

## Visible outcomes

- Trusted: eligible executable gets its SVID, signs the payload, and moves a fixed amount once per logical request.
- Unidentified: cannot get a signing SVID; Money returns a signed `401` and balances remain unchanged.
- Valid but unauthorized signer: Money returns signed `403`; balances remain unchanged.
- Lost or unverified response: the outcome stays unknown and a retry keeps the same request ID.

Both callers can reach the money network and Workload API socket. Socket permissions or network isolation alone must not be the reason the negative identity test passes.

## Threat-model boundaries

The demo targets forged workload identities and ineligible same-UID executables. It intentionally does not restrict which humans can trigger transfers. It does not protect against compromise of the host/Docker/SPIRE, the UI's legitimate trigger authority, or the authorized process/private key. An executable digest proves an issuance condition, not continuous runtime integrity.

Only the remote browser HTTPS edge is published. Public access without verified human authentication is the owner-requested simplified scope. The previous localhost-only assumption is superseded by the remote-access requirement.

## Future changes

Use [current OpenSpec contracts](../openspec/specs/) and
[the baseline/evidence index](spec-baseline.md) to establish existing behavior.
Follow [the workflow](spec-workflow.md): a spec delta and short design/plan need
Rudi's explicit approval before any future behavior-changing code/apply.

Implementation is authorized. See [P1 evidence](attestation-feasibility.md); M2 retention is approved and tested. The UI uses an isolated UID-1000 container with the measured native agent. The owner-approved public installation is live and verified; see [the deployment runbook](remote-demo.md). See [TODO.md](../TODO.md) for acceptance gates and [plan.md](plan.md) for sequencing.
