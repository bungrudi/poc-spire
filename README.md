# SPIRE / SPIFFE process-identity PoC

**Status: simplified HTTPS UI implemented; real-SPIRE HTTPS browser integration passes. The owner-approved deployment is live at https://spire.adianto.dev/.**

The Go + Docker Compose demo has a web UI with two buttons. On 11 October 2026 the owner removed OIDC and requested an assumed operator to focus on SPIRE. The page discloses that anyone can trigger fake transfers. See [the current demo and deployment plan](docs/remote-demo.md). Each button triggers a different caller process to attempt the same fake money transfer. The money service authenticates workloads directly using SPIFFE mTLS: the authorized caller succeeds; the unidentified caller fails. No sidecar terminates workload authentication.

## Workspace documents

| Document | Purpose |
| --- | --- |
| [WORKSPACE.md](WORKSPACE.md) | Requirements, recommended decisions, scope, document map, and future repository layout |
| [Technical RFC](docs/rfcs/0001-process-identity.md) | Detailed architecture, attestation, remote access, API contracts, threat model, and test matrix |
| [TODO.md](TODO.md) | Ordered, checkable backlog with acceptance gates |
| [Design summary](docs/design.md) | Short architecture overview |
| [Delivery plan](docs/plan.md) | Milestones and sequencing |
| [AGENTS.md](AGENTS.md) | Rules for future coding agents |
| [Current feature contracts](openspec/specs/) | Six source-derived OpenSpec capabilities with normal and negative scenarios |
| [Baseline and traceability](docs/spec-baseline.md) | Source/test/evidence mappings, current versus historical coverage and limits |
| [OpenSpec workflow](docs/spec-workflow.md) | Pinned local tooling, document authority and owner approval before behavior changes |

## Isolated Kafka signing extension

The combined learning profile adds three **Async (Kafka)** cases alongside the
existing **Sync (HTTP/mTLS)** trusted/unidentified scenarios. Money verifies the
original SPIFFE-backed JWS producer and updates the same fake ledger; the UI
reads authenticated digest receipts and preserves retry IDs/deadlines.
Money remains memory-only across restart. This extension is on the isolated
implementation branch and is not installed at the live public URL.

Start with the [runner runbook](infra/kafka-signing/README.md),
[normative profile](docs/kafka-signing-profile.md),
[acceptance coverage](docs/kafka-signing-acceptance.md) and
[actual approval](openspec/changes/add-spiffe-kafka-message-signing/review.md).

## Spec-driven development

The current features are documented directly as a baseline in `openspec/specs/`.
Before changing behavior, draft a spec delta and short design/plan, resolve
questions, and obtain Rudi's explicit approval before code/apply. Agent reviews
and passing CLI validation do not establish owner approval or security acceptance.
See [the workflow](docs/spec-workflow.md) for the full sequence.

OpenSpec 1.14.1 is a locked project-local development dependency. Use an existing
Node >=20.19.0 runtime (`.node-version` records measured Node 22.23.0), then:

```sh
npm ci
npm run spec:version
npm run spec:list
npm run spec:validate
```

The committed seven Codex skills include verification. `npm run spec:setup`
reproduces them. Use `npm run spec -- <arguments>` for all CLI calls, including
bare commands shown by the official skills. The wrapper isolates user settings
and disables telemetry/update checks; it performs no global installation.
Node/OpenSpec are not needed to build or run the Go demo.

## Recommendations

- Run callers in **separate containers with the same numeric UID**. Require executable-bound attestation as well as deployment selectors; verify this with a wrong-executable test inside the trusted deployment. Container identity alone is not sufficient.
- Use **strict mTLS**. No certificate means TLS rejection, not an HTTP response. A valid but unauthorized identity gets an HTTP `403` at the money service.
- Browser access uses **HTTPS, secure sessions and CSRF protection**. The human operator is explicitly assumed; SPIRE authenticates the caller processes. OIDC is removed for this owner-requested learning demo.

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

Rudi approved protected project-local PoC CA/database/agent retention. The identity substrate, money service and private caller controls are implemented and tested; see [implementation checkpoint](docs/implementation-status.md). On 11 October 2026 the owner also requested public source publication to [bungrudi/poc-spire](https://github.com/bungrudi/poc-spire). Credentials and runtime state remain excluded. The simplified UI uses the measured UID-1000 agent layout; public HTTPS installation is approved, live and verified; see the current deployment plan.

The tested internal commands are `python3 scripts/test_identity_restart.py`, `python3 scripts/security_suite.py`, and `python3 scripts/caller_suite.py`. Establish isolation before running them, one at a time: they use approved retained identity state and remove their own test grants/resources. See the checkpoint for historical coverage and the [baseline index](docs/spec-baseline.md) for current scope and limits.

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
historical T01–T23 coverage and automatic-pruning limit; the current UI profile
and coverage are in the [baseline index](docs/spec-baseline.md).

## Simplified browser demo

`python3 scripts/ui_suite.py` tests the HTTPS browser API with real SPIRE in an
isolated loopback-only stack. It removes its temporary browser/workload keys and
containers and preserves the approved retained identity state. Run it separately
from other SPIRE commands. Results: `docs/evidence/ui-demo.json`.

The existing local owner-login preview is still available through
`python3 scripts/local_demo.py`, then `python3 scripts/local_login.py` in a second
terminal. The new public demo profile assumes an operator and requires a
provisioned browser certificate; it is not the localhost login profile exposed
through a proxy. The owner-approved deployment is live at [spire.adianto.dev](https://spire.adianto.dev/).
See [the deployment runbook](docs/remote-demo.md).

## Live demo

Open [https://spire.adianto.dev/](https://spire.adianto.dev/). The owner explicitly
approved public access with an assumed operator and no human login. Click
**Attempt trusted transfer** for a verified USD 1.00 fake transfer, and
**Attempt unidentified transfer** for no-SVID/TLS rejection with no balance
change. Repeat the same attempt to test deduplication; choose **New logical
attempt** for another transfer.

The service starts after reboot. A full restart resets fake balances and retry
history. See [the operations runbook](docs/remote-demo.md) and
[sanitized public verification](docs/evidence/public-demo.json).

## Illustrated guide

The [process-identity page](https://spire-process-identity.rudiadianto.chatgpt.site/)
explains the services through one fake transfer, with rendered Mermaid diagrams
and excerpts from the Go sender and receiver. Its portable source is in
[docs/explainer](docs/explainer/README.md). It is an illustrative guide; use
[spire.adianto.dev](https://spire.adianto.dev/) to exercise the running demo.
