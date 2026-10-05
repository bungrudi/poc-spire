# Extended backend lifecycle checkpoint — 5 October 2026

Project: `/home/ubuntu/Workspace/poc-spire` on `personal-vps`. This increment
covers additional T18/T21 lifecycle behavior using the existing single native
UID-1000 join-token agent model. UI/exposure/privilege decisions remain pending.
No public listeners, deployment, persistent access, host clock changes or
modified SPIRE binary were introduced.

## Verified cases and changes

[Sanitized evidence](evidence/backend-extended.json) records exact versions,
binary/image/config digests, results and cleanup.

| Case | Measured observation |
| --- | --- |
| T21 rebuild | V2 has a different binary digest inside the same container/image/UID/deployment. Old entry denies fresh issuance. The runtime refuses implicit digest changes. An explicit native update changes only the digest; V2 then gets the trusted identity and transfers, while V1 fresh issuance is denied. |
| Lifetime reuse defect | Previously an otherwise matching entry could be reused with a different requested SVID TTL. The runtime now refuses that mismatch, requiring explicit lifetime review. Native exact-entry reuse and read-only mismatch tests pass. |
| Policy guards | Unit cases reject broader selectors, wrong parent, duplicate grants and mismatched lifetime without create/update/delete. An exact pre-existing entry is reused without becoming owned by test cleanup. No broader real grant was created to test these guards. |
| Active-root guard | Native API rejects taint and revoke of the active CA; the public bundle stays unchanged. |
| Old-root taint | A prepared authority is distributed before explicit activation. Tainting the old authority rotates both caller and Money credentials while the captured old credential remains unexpired. Taint alone leaves that credential usable until root removal. |
| Native old-root removal | Revoke removes the old authority from SDK bundles and the atomically published public file. A captured, still-unexpired old-root peer is rejected at TLS while verifying current Money; it receives no Money HTTP response and makes no ledger change. This is administrative removal, not the automatic expiry sweep. |
| Extended outage | Server is down for 97 seconds, beyond all 90-second workload and 30-second node credentials. Expired requests fail without Money HTTP success. Restoring the server alone does not restore an expired join-token agent. Normal runtime restart fails closed without generating a new join token or resetting CA keys. |
| Explicit recovery | Only the suite's own temporary expired agent state is replaced. A one-use token attests a new parent, applied explicitly to the existing narrow workload entries. CA keys/bundle are preserved; live workload sources reconnect without app restart. Money ledger/history stay unchanged and a prior key replays without another debit. Bootstrap token file is removed. |

An advisory lock serializes this fixture with other substrate commands. The
fixture uses CA TTL 600 seconds, private loopback administration, an internal
Docker network, no published ports and the existing application restrictions.
Private keys/SVIDs are never returned in evidence. Captured TLS credentials stay
only in test-process memory. The re-enrollment branch checks the ownership/path
of its own temporary directory and never targets `.runtime/identity/agent/`.

Retained PoC CA/agent file contents and permissions compare equal before/after.
All ephemeral CA/agent state, tokens, grants, containers, network and image are
removed. Retained directories remain 0700/private files 0600. No automatic
recovery/reset operation was added to the normal PoC runtime.

## Checks and commands

```sh
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go vet ./...
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/backend_extended_suite.py
python3 scripts/backend_suite.py
python3 scripts/test_identity_restart.py
```

Formatting, Go tests/race/vet, seven Python cases, the extended real-SPIRE suite,
baseline lifecycle suite and retained-state restart proof pass. Production Money
and caller code did not change in this increment; their prior integration results
remain available and those two suites were not rerun here.

## Exact remaining work

The automatic expired-root sweep is **unrun**. In the pinned release, pruning
uses a 24-hour post-expiry safety threshold and a six-hour sweep timer. A bounded
minutes-long run cannot observe that timer branch. No clock manipulation or
replacement binary is used to manufacture a pass. Production 24-hour CA rollover
duration and arbitrary packet-loss injection also remain unrun. Administrative
root removal and the actual post-commit connection-drop proof are recorded
separately from those cases.

T15 overflow/funds and T23 capacity include unit coverage; full UI rate/capacity,
OIDC/HTTPS/session/CSRF, deterministic retry across UI restart/re-login and full
T01–T23 owner acceptance remain open. The UID/agent-layout and non-secret
HTTPS/OIDC/operator configuration choices still block UI implementation.
Upstream-authority, federated and multi-agent cases are outside this project's
single-agent baseline; they are not new acceptance requirements.

The retained PoC agent's expired-node recovery still needs an explicit owner
operation/decision. This fixture proves the mechanics; it does not authorize a
new retained node parent or automatically rewrite real registrations.

Pinned behavior was checked against
[local authority API](https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/api/localauthority/v1/service.go),
[agent SVID rotation](https://github.com/spiffe/spire/blob/v1.14.0/pkg/agent/svid/rotator.go),
[CA pruning threshold](https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/ca/manager/manager.go)
and [sweep timer](https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/ca/rotator/rotator.go).

The subsequent [final backend status](backend-final-status.md) reconciles the
matrix and adds passing application-proxy TLS-stream fault cases. This is not
TCP/IP packet retransmission/loss-distribution coverage. Automatic expired-root
sweeping still has no supported bounded acceleration in the pinned binary.
