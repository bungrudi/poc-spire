# SPIRE / SPIFFE process-identity PoC

**Status: identity substrate, money service and internal caller/control path verified. Remote UI is blocked on configuration/layout decisions.**

The proposed Go + Docker Compose demo has a remotely accessible web UI with two buttons. Each button triggers a different caller process to attempt the same fake money transfer. The money service authenticates workloads directly using SPIFFE mTLS: the authorized caller succeeds; the unidentified caller fails. No sidecar terminates workload authentication.

## Workspace documents

| Document | Purpose |
| --- | --- |
| [WORKSPACE.md](WORKSPACE.md) | Requirements, recommended decisions, scope, document map, and future repository layout |
| [Technical RFC](docs/rfcs/0001-process-identity.md) | Detailed architecture, attestation, remote access, API contracts, threat model, and test matrix |
| [TODO.md](TODO.md) | Ordered, checkable backlog with acceptance gates |
| [Design summary](docs/design.md) | Short architecture overview |
| [Delivery plan](docs/plan.md) | Milestones and sequencing |
| [AGENTS.md](AGENTS.md) | Rules for future coding agents |

## Recommendations

- Run callers in **separate containers with the same numeric UID**. Require executable-bound attestation as well as deployment selectors; verify this with a wrong-executable test inside the trusted deployment. Container identity alone is not sufficient.
- Use **strict mTLS**. No certificate means TLS rejection, not an HTTP response. A valid but unauthorized identity gets an HTTP `403` at the money service.
- Remote UI access requires **HTTPS, OIDC login, explicit operator authorization, and CSRF protection**. Workload identity does not replace human authorization.

The first increment adds a Go test fixture, dependency locks, private Compose fixtures, and an ephemeral SPIRE runner. See [measured attestation evidence](docs/attestation-feasibility.md) and [sanitized results](docs/evidence/attestation-latest.json). The measured layout uses equal UID 1000 and an unprivileged native agent; the proposed UID 10001/containerized agent remains unverified.

Run on the existing Linux host as the existing Docker-enabled user (not root):

```sh
cd /home/ubuntu/Workspace/poc-spire
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go vet ./...
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/attestation.py
```

The runner downloads checksum-pinned SPIRE 1.14.0, builds locally content-addressed static fixtures, starts loopback-only SPIRE and private containers, and removes its own credentials/state/containers/image/network afterward. It publishes no ports. The TCP sentinel tests reachability only; it is not a money API or demo UI. Missing selectors, timeout, infrastructure failure or unexpected identity fail the suite.

Rudi approved protected project-local PoC CA/database/agent retention. The identity substrate, money service and private caller controls are implemented and tested; see [implementation checkpoint](docs/implementation-status.md). Initial private GitHub publication is separately authorized for [bungrudi/poc-spire](https://github.com/bungrudi/poc-spire); no services are publicly exposed. M5 requires a UI UID/agent-layout decision and remote DNS/HTTPS/OIDC/operator configuration.

The next tested internal commands are `python3 scripts/test_identity_restart.py`, `python3 scripts/security_suite.py`, and `python3 scripts/caller_suite.py`. Run them one at a time. They retain only approved identity state and remove their own test grants/resources. See the checkpoint for measured limits and M5 blockers.

The bounded backend acceptance command is `python3 scripts/backend_suite.py`. It
uses isolated short-TTL SPIRE state, removes all test keys/state, and verifies
retained CA/agent state was unchanged. Read [backend acceptance](docs/backend-acceptance.md)
for coverage and limits. Run it separately from other substrate commands.

The extended command is `python3 scripts/backend_extended_suite.py`. It tests
native old-root removal, a reviewed binary registration update and recovery after
all cached credentials expire. It performs explicit re-enrollment only inside its
own temporary fixture; it cannot recover/reset the retained PoC agent. See
[extended coverage](docs/backend-extended-acceptance.md) before running it.

Final bounded transport-fault verification: `python3 scripts/backend_proxy_suite.py`.
This opaque, credential-free proxy runs only inside the internal test network.
Read the [final backend status](docs/backend-final-status.md) for the reconciled
T01–T23 coverage, automatic-pruning limit and remaining M5 decisions.
