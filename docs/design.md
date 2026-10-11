# Design summary

**Status: measured UID-1000/native-agent backend verified; simplified HTTPS UI implemented with isolated browser integration passes. The owner-approved deployment is live at https://spire.adianto.dev/. See [current demo](remote-demo.md).** Detailed decisions and test contracts are in [RFC 0001](rfcs/0001-process-identity.md). The current workspace brief is [WORKSPACE.md](../WORKSPACE.md).

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
                                       direct HTTPS to Go money service
                                           strict SPIFFE mTLS
                                          exact-ID authorization
                                           in-memory fake ledger

SPIRE server <--> SPIRE agent -- Workload API socket --> Go workloads
UI -- its own read-only SPIFFE mTLS identity --> money GET /balances
```

The money service validates the caller in Go. No sidecar, proxy, identity header, source IP, or browser credential substitutes for workload authentication. The caller verifies the money service's exact SPIFFE ID too.

## Workload authorization and the human assumption

1. **Human assumption:** the owner explicitly removed OIDC on 11 October 2026. Anyone can trigger fake transfers with an assumed-operator session. The UI discloses this; HTTPS/CSRF/session controls remain.
2. **Workload gate:** money requires a valid X.509-SVID and allows only the trusted caller's exact SPIFFE ID to transfer.

The UI has only read permission on the money API and private access to caller trigger sockets. It never receives the trusted caller's SVID. Callers have no network trigger endpoint; the unidentified caller cannot access the trusted control socket.

## Visible outcomes

- Trusted: eligible executable gets its SVID, establishes direct mTLS, and moves a fixed fake amount once per logical request.
- Unidentified: cannot get a matching SVID; an intentional no-client-certificate HTTPS probe demonstrates TLS rejection. The UI shows a TLS failure, not a fabricated money-service HTTP `403`.
- Valid but unauthorized identity: TLS authenticates it, then the money handler returns `403`; balances do not change. This is a security test, not a third main button.

Both callers can reach the money network and Workload API socket. Socket permissions or network isolation alone must not be the reason the negative identity test passes.

## Threat-model boundaries

The demo targets forged workload identities and ineligible same-UID executables. It intentionally does not restrict which humans can trigger transfers. It does not protect against compromise of the host/Docker/SPIRE, the UI's legitimate trigger authority, or the authorized process/private key. An executable digest proves an issuance condition, not continuous runtime integrity.

Only the remote browser HTTPS edge is published. Public access without verified human authentication is the owner-requested simplified scope. The previous localhost-only assumption is superseded by the remote-access requirement.

## Next step

Implementation is authorized. See [P1 evidence](attestation-feasibility.md); M2 retention is approved and tested. The UI uses an isolated UID-1000 container with the measured native agent. The owner-approved public installation is live and verified; see [the deployment runbook](remote-demo.md). See [TODO.md](../TODO.md) for acceptance gates and [plan.md](plan.md) for sequencing.
