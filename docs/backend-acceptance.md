# Backend acceptance checkpoint — 5 October 2026

Project: `/home/ubuntu/Workspace/poc-spire` on `personal-vps`. This bounded
increment follows the existing RFC's backend matrix. UI, exposure and agent
privilege decisions are unchanged. Nothing was deployed or publicly exposed.

## Changes and evidence

The public Workload API trust bundle previously stayed at its startup snapshot.
`BundlePublisher` now polls the private SPIRE server API every two seconds,
validates nonempty CA PEM, and replaces the public file atomically with mode
0444 set before rename. The private bootstrap copy stays 0600. Failed or
malformed refresh preserves the last authenticated bundle; there is no fabricated
trust or certificate-verification bypass. The publisher has a three-second fetch
deadline and stops with its owning runtime.

The unidentified caller now labels a missing trust file as `server_trust_bundle`
infrastructure failure, preserving its separately measured no-matching-identity
eligibility. It does not label missing sockets/bundles as successful TLS rejection.

[Sanitized lifecycle evidence](evidence/backend-lifecycle.json) records versions,
binary/image/config digests, narrow selector policy, outcomes and cleanup:

| Case | Verified result |
| --- | --- |
| T18 unrelated valid CA bundle | SDK rejects Money's chain before application data; no money HTTP status or mutation. The unrelated private key is generated only in test-process memory. |
| T18 missing bundle/socket | Actual unidentified executor reports the correct infrastructure stage; no mutation or claimed identity rejection. |
| T16 response lost after commit | Test-only Money wrapper invokes the real authenticated handler, then closes its TLS connection before responding. Actual trusted executor reports unknown commit; same-ID retry returns the original committed result without another debit. |
| Registration removal | Fresh Workload API issuance becomes no-matching-identity. The live client's cached SVID cannot transfer after expiry. Restoring the same exact narrow entry permits issuance again. No instant certificate revocation is claimed. |
| Real CA rollover | Both client and Money certificate authority IDs change. SDK bundle adds overlap roots, atomic public bundle matches the authenticated server bundle, and fresh verified requests continue. Public-bundle no-cert probe verifies rotated Money and is rejected for client authentication. |
| T22 Money restart | Balances/version/history reset. A prior request key becomes a new operation, documenting process-lifetime-only idempotency. SPIRE trust is not reset. |
| T18 server outage | Existing valid cached caller SVID succeeds; after its expiry the peer is rejected at TLS with no Money HTTP status or mutation. Last validated public bundle is preserved. |
| T18 agent outage | Actual unidentified caller reports Workload API infrastructure failure rather than identity rejection; no mutation. |
| T23 HTTP capacity boundary (unit) | Unauthorized identity cannot consume history slots; full capacity returns 503; an earlier accepted key remains replayable without mutation or eviction. Synthetic peer data isolates handler behavior; real-SPIRE suites separately prove TLS/route identity verification. |

The lifecycle fixture uses an isolated temporary SPIRE CA with TTL 120 seconds
and shorter caller credentials. Its host server binds only loopback; its Docker
network is internal with no published ports. Applications are UID/GID 1000,
read-only, no-new-privileges, drop all capabilities, and mount only the read-only
public Workload API directory. Administration, private state and Docker access
remain outside application mounts.

All ephemeral CA/agent keys, tokens, databases, fixture containers/network/image
and registrations are removed. The suite compares retained private-state file
content and permissions before/after, emitting only an equality boolean. Approved
retained PoC CA/agent state remains under `.runtime/identity/`; no reset, new
persistent access or host security change was introduced.

## Verification and commands

Run commands serially in the existing Linux checkout:

```sh
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go vet ./...
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/backend_suite.py
python3 scripts/test_identity_restart.py
python3 scripts/security_suite.py
python3 scripts/caller_suite.py
```

Final formatting, Go tests/race/vet, five Python regression cases, and all four
real-SPIRE suites pass. Earlier M3/M4 evidence is refreshed by regression runs.
The backend fixture is test-only stdin/stdout tooling, not a remotely accessible
trigger or an OIDC substitute. Its controlled response-loss wrapper is separate
from the production Money binary.

## Remaining limits

Short-TTL CA rollover and overlap pass. A subsequent
[extended lifecycle checkpoint](backend-extended-acceptance.md) also passes
native old-root taint/revocation propagation, reviewed executable updates and
recovery after a node/workload-expiring outage. Automatic expired-root sweeping,
production 24-hour rollover duration and arbitrary packet-loss injection remain
unrun. T15 overflow/funds and T23 capacity include unit coverage; browser security,
UI restart/re-login retry and full T01–T23 owner acceptance remain open.

Pinned-release behavior was checked against SPIRE v1.14.0
[server configuration](https://github.com/spiffe/spire/blob/v1.14.0/doc/spire_server.md),
[CA rotator](https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/ca/rotator/rotator.go)
and [CA slots](https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/ca/manager/slot.go).
Measured results, rather than configured TTLs alone, establish rollover and expiry.

The subsequent [final backend status](backend-final-status.md) reconciles the
matrix and adds passing application-proxy TLS-stream fault cases. This is not
TCP/IP packet retransmission/loss-distribution coverage. Automatic expired-root
sweeping still has no supported bounded acceleration in the pinned binary.
