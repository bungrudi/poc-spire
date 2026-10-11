# Agent instructions

## Owner OpenSpec adoption and workflow approval — 11 October 2026

Rudi directly instructed the parent chat on 11 October 2026: “ok apply
openspec into the project. you be the orchestrator, control threads in the
poc-spire project itself to implement. commit push at will”. The original owner
message is `01a12b26-776c-7323-8470-c13a5b05ca98` in Codex chat
`01a12b20-5c40-7be0-9ebb-2b23ecfb2d43`.

This authorizes the OpenSpec adoption and gives standing permission to commit
and push authorized poc-spire project work to the existing
`https://github.com/bungrudi/poc-spire.git` origin. Do not ask again for routine
commit/push permission within that authorized scope. This supersedes the earlier
per-release push restriction and remains in effect unless the owner changes it.
It does not approve an unreviewed behavior change or unrelated access changes,
runtime publication, persistent credentials, host-security changes or deployment.
For this adoption the orchestrator coordinates independent verification before
finalizing; that sequencing is not a new permanent publication-approval gate.

The owner requires **review of the spec delta and a short design/plan before
every future behavior change**. Draft the change, resolve relevant questions,
present it to Rudi and wait for explicit approval before code/apply. Record the
actual approval message/date and approved artifact revision in the change's
`review.md`; an agent-written review, checked task, validation pass or permission
to commit/push is not owner approval. A changed behavioral scope needs renewed
review before dependent implementation. This supersedes the earlier open-ended
implementation loop for future behavior changes; the adoption itself is
documentation/tooling work already authorized.

Read [the workflow and document map](docs/spec-workflow.md) and
[the baseline/evidence index](docs/spec-baseline.md). Use the relevant
`openspec/specs/<capability>/spec.md` for current behavior and RFC 0001 for
architecture/security rationale; reconcile both when an approved change lands.
Do not convert historical OIDC plans into current public-demo requirements.

For all OpenSpec CLI invocations, including bare `openspec` examples in shipped
skills, use `npm run spec -- <arguments>` to run the pinned project-local tool
with isolated settings. `npm ci` requires an existing Node >=20.19.0 runtime;
see `.node-version` and the workflow. `npm run spec:setup` regenerates the seven
official Codex skills, including verification. Never install globally. Structural
validation and agent verification reports do not enforce approval or prove
security tests. Keep credentials/runtime/cache/node_modules out of Git.

## Earlier owner publication approval — 11 October 2026

The owner explicitly requested committing and pushing the current project code
to public GitHub. This authorizes publication of this release to the existing
`bungrudi/poc-spire` repository and changing that repository to public after
checking source and history for credentials. It does not authorize future
pushes, unrelated access changes, or publication of runtime state. The static
service-story explainer is exported in `docs/explainer/`; `/sites/` is a separate,
ignored Sites checkout.

## Owner scope update — 11 October 2026

The owner requested deployment at `spire.adianto.dev` and then explicitly
removed OIDC, asking to assume human authentication to focus on SPIRE. The
current HTTPS demo labels its operator as **assumed, not authenticated**;
anyone with browser access may trigger fake transfers. The original OIDC and
verified-human acceptance requirements below are historical and superseded
for this demo. Workload attestation, direct mandatory mTLS, exact-ID Money
authorization, separate caller control mounts, HTTPS and CSRF checks remain.
The isolated UI uses the measured UID-1000/native-agent layout. See
[the current demo and deployment plan](docs/remote-demo.md). The owner specifically approved the HTTPS certificate/key, Nginx route,
renewal hook, persistent service and public access without human login. Those
changes are installed and verified at `https://spire.adianto.dev/`.


## Current mode: implementation and named demo deployment authorized

On 5 October 2026 Rudi instructed the implementation loop to proceed in this existing `personal-vps` project following the RFC/backlog. P1 attestation feasibility is verified in the measured UID-1000/native-agent layout; see `docs/attestation-feasibility.md`. This supersedes the previous documentation-only restriction.

Authorization covers project implementation and isolated testing. On 5 October 2026 Rudi separately authorized initializing Git and the initial publication to a new private `bungrudi/poc-spire` GitHub repository. The original publication approval does not authorize future pushes, merges, migrations, collaborators or unrelated deployment/credential/access changes. The 11 October approval covers only the named demo, protected browser TLS key, Nginx route, renewal hook and startup service. Obtain specific approval before creating/retaining persistent credentials or host access, or changing host/network security settings. The ephemeral P1 runner retains no credentials. Rudi separately approved project-local PoC CA/agent state retention on 5 October 2026. That state now exists with owner-only permissions; no unrelated persistent access or deployment is authorized. M2–M4 and simplified HTTPS UI/public checks pass. The current demo uses an isolated UID-1000 UI and explicitly assumes the human operator; see `docs/remote-demo.md`.

Read in order:

1. `WORKSPACE.md` — confirmed requirements, recommendations, pending configuration
2. `docs/rfcs/0001-process-identity.md` — authoritative technical proposal
3. `docs/spec-baseline.md` and relevant `openspec/specs/` — current contracts and evidence limits
4. `docs/spec-workflow.md` — change workflow and owner-review gate
5. `TODO.md` — actual task status and acceptance gates; original milestone checklist is historical
6. `docs/plan.md` — milestone order

`README.md` indexes the workspace; `docs/design.md` is a summary, not a competing specification. Keep documents consistent. The remote-access requirement supersedes the earlier localhost-only assumption.

## Security invariants for future implementation

- Caller-to-money TLS terminates **inside the Go money service**. No identity-terminating sidecar, forwarded identity header, plaintext fallback, or optional client-certificate mode.
- Authenticate real X.509-SVIDs, then authorize exact SPIFFE IDs by route before business logic. Trust-domain membership alone is not transfer permission.
- Clients verify the money service's exact SPIFFE ID. Use Go SPIFFE sources for rotation and verification; never use an application certificate-verification bypass.
- Unidentified clients fail TLS, with no money-service HTTP status. An authenticated but unauthorized identity receives `403`. Keep those cases distinct in logs/UI.
- Same UID, same container, image labels, and binary paths alone are not executable identity. Verify agent-derived executable-digest selectors plus deployment restrictions against a pinned SPIRE release before claiming process attestation.
- The two callers run in separate containers with the same UID/mapping and can reach both money and the Workload API socket. Include a controlled same-deployment wrong-executable test and a wrong-deployment approved-binary test.
- Do not invent plugin options or selector syntax. Prefer verified built-in attestors; a custom attestor requires explicit scope approval. Do not silently downgrade to container identity.
- The specifically approved learning demo uses HTTPS, CSRF/origin/Host checks and safe demo sessions, and declares an assumed human operator. OIDC is intentionally removed; anyone visiting may trigger fake transfers. Do not describe the assumed human as authenticated. Browser TLS configuration still fails closed.
- Callers have private, separate UDS trigger channels, not network trigger endpoints. The unidentified caller must not reach the trusted caller's control socket. UI may trigger callers but never borrows their credentials.
- The UI's distinct workload identity can read balances, not transfer. SPIFFE authenticates the process; this simplified demo explicitly assumes the human. Never claim SPIFFE authenticates the browser user.
- Application containers receive no Docker/admin sockets or other workloads' credentials. The agent's required host/Docker privileges belong in the trusted computing base. A read-only Docker socket mount is not a read-only API.
- Fake money uses integer units, atomic updates, and identity-bound idempotency. Do not create a fresh transfer ID when retrying an unknown result.
- Keep credentials, SVID/private-key material, join tokens, OIDC tokens, cookie values, and runtime state out of git/logs/evidence.

## Working practices after coding is authorized

- Follow the owner-reviewed OpenSpec change workflow and current `TODO.md` work. Retain the original sequencing principle: prove attestation before dependent UI work and secure the UI before remote exposure; do not revive retired verified-human gates for this profile.
- Use small Go packages, minimal dependencies, and deterministic tests. Run formatting, `go test ./...`, the race detector where supported, and the real-SPIRE Compose security suite when those exist.
- Preserve negative tests: wrong executable, wrong deployment, no cert, fake identity header, wrong valid SPIFFE ID, CSRF, duplicate transfer and expired credentials. Preserve anonymous/viewer/owner checks for the separate local preview; verified-human/OIDC denial is retired for the public assumed-operator profile.
- Document exact versions, selectors, test evidence, and any deviations. Mark tests as unrun if unrun; infrastructure errors do not count as identity rejection.
- Update `TODO.md` only for completed work. Do not mark design recommendations as implemented.
- Keep the scope honest: fake-money PoC, not production payments or protection against host/SPIRE/authorized-process compromise. Executable attestation is not continuous runtime integrity.
