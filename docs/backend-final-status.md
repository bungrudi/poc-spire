# Backend implementation status — 5 October 2026

Existing project: `/home/ubuntu/Workspace/poc-spire` on `personal-vps`.
M1–M4 internal backend behavior is implemented and measured. The remote UI,
operator security, exposure and whole-PoC owner acceptance are not complete.
Implementation stops at the existing M5 decisions listed below.

## Final independent increment

`python3 scripts/backend_proxy_suite.py` adds test-only opaque TCP forwarding
on the existing internal Compose network. It neither terminates TLS nor parses
HTTP and has no credentials, Workload API/admin mounts or host-published ports.
The proxy runs as UID 1000, drops all capabilities, uses a read-only filesystem
and no-new-privileges, and bounds connections/deadlines/record allocation.
The normal demo profile and application code are unchanged.

[Sanitized proxy evidence](evidence/backend-proxy.json) verifies:

- Pass-through preserves client verification of the exact Money identity and
  Money's rejection of the no-SVID client, with no unauthorized ledger change.
- Discarding either direction's handshake records causes infrastructure failure
  with no verified server, Money HTTP result or mutation. Restoring the path and
  retrying the same key commits once; another retry replays the same result.
- The response-loss gate arms on the encrypted client handshake flight and
  discards later server encrypted records. Independently observed server
  verification and the live Money ledger prove the commit occurred. The caller
  reports unknown commit; retrying through the restored path with the same key
  returns the committed result without another debit. Money's special response
  drop fixture is not used for this key.

These are application TLS-stream fault tests. TCP/IP packet retransmission,
arbitrary packet-loss distributions and network reordering are **unrun**. No
host networking, firewall, clock or privilege settings were changed. The first
proxy run incorrectly assumed a single encrypted handshake record; its failed
evidence is retained as `backend-proxy-first-run.json`. The corrected gate and
regression tests pass; the final evidence is passing.

Automatic expired-root sweeping remains **unrun**. The pinned release fixes a
[24-hour post-expiry grace](https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/ca/manager/manager.go)
and a [six-hour sweep timer](https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/ca/rotator/rotator.go).
Neither depends on the fixture CA TTL. There is no supported accelerated
configuration for that branch; no replacement SPIRE binary or clock workaround
was introduced, and no long-duration job was started. Administrative old-root
taint/removal and short-TTL automatic rollover remain separately verified in
[extended coverage](backend-extended-acceptance.md) and
[baseline coverage](backend-acceptance.md).

## Reconciled RFC acceptance coverage

“Backend passed” means the internal scenario below has evidence, not Gate G.
UI-dependent rows remain partial/open even when their backend portion passes.

| RFC case | Current measured coverage |
| --- | --- |
| T01–T02 | Partial: actual trusted/unidentified caller paths pass; remote operator/button behavior unimplemented. |
| T03 | Backend passed: equal caller UID/mapping, actual Workload API and Money reachability. |
| T04–T05 | Backend passed: wrong executable in matching deployment and approved executable in wrong deployment denied. |
| T06–T09 | Backend passed: no cert/forged headers, self-signed allowed URI, valid unauthorized IDs/UI transfer and impostor Money rejected at their proper stages. |
| T10–T11 | Open: remote human roles, CSRF/origin/Host gates are unimplemented. |
| T12 | Backend passed: private per-caller UDS isolation, kernel peer-UID and bounded trigger protocol. |
| T13–T14 | Backend passed: concurrent duplicate keys mutate once; changed payload conflicts without mutation. |
| T15 | Backend integration/unit coverage: input/body bounds, atomicity, conservation, insufficient funds and overflow. Overflow/funds limits include unit evidence. |
| T16 | Backend passed: real post-commit connection close plus opaque proxy response-record loss; same-ID retry prevents a second debit. |
| T17 | Backend passed: real SVID rotation, expiry guards and frozen-expired/reuse attempts; fresh handshakes/no resumption policy. |
| T18 | Backend passed: SPIRE/agent outage, missing socket/bundle and wrong trust classification; live-source recovery tested in own temporary state. UI labels remain open. |
| T19 | Open: OIDC/token/session verification unimplemented. |
| T20 | Partial: internal fixture ports/mounts/capabilities/state permissions and sanitized evidence checked; remote edge/cookies/final deployed layout unrun. |
| T21 | Backend passed: rebuild denial, explicit native digest update preserving exact parent/other selectors/lifetime; broader/duplicate/mismatched-policy guards. |
| T22 | Partial: Money restart resets process-lifetime ledger/history; SPIRE normal restart preserves trust/parent. Explicit node-expiring recovery preserves CA/live ledger in own fixture. UI restart/re-login retry remains open. |
| T23 | Partial: bounded backend bodies/control/history and no unsafe key eviction tested. Full UI rate/concurrency/status-history limits remain open. |

Primary evidence: [attestation](evidence/attestation-latest.json),
[Money security](evidence/money-security.json),
[caller control](evidence/callers-control.json),
[retained restart](evidence/identity-restart.json), and the three backend reports
linked above. Retained keys/private state are not evidence artifacts.

## Validation and stop boundary

[Final validation and cleanup](evidence/backend-final-validation.json) records
check outcomes and report fingerprints.

Formatting, `go test ./...`, `go test -race ./...`, `go vet ./...`, seven Python
unit cases and the real-SPIRE proxy suite pass. Money security, caller/control
and retained restart suites pass on the final checkout. The baseline/extended
lifecycle suites passed the preceding increment; their code did not change in
this increment and they were not rerun. Temporary containers/images/state and
SPIRE child processes are removed; retained CA/agent content/permissions are
unchanged by the proxy suite and remain protected (0700 directories/0600 files).

Pending owner decisions:

1. Keep the recommended different-UID UI and review the agent privilege layout,
   or approve an isolated UID-1000 UI with adapted control peer policy. The
   current UID-1000 agent cannot hash the proposed UID-10002 UI executable.
2. Choose hostname, private/public exposure and browser certificate provisioning;
   supply non-secret OIDC issuer/client/callback and explicit operator/viewer
   mapping. Secret material must use protected runtime provisioning.
3. Real retained-agent recovery after node expiry requires an explicit owner
   operation; temporary recovery tests add no automatic reset/re-enrollment.

No UI bypass, privilege increase, public deployment or merge is authorized
or performed. Rudi separately authorized the initial private GitHub publication
to `bungrudi/poc-spire`; this adds no service exposure or collaborator access. Production-duration CA tests and the full T01–T23/UI acceptance
remain open; this checkpoint makes no whole-PoC completion claim.
