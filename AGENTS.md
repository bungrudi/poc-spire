# Agent instructions

## Current mode: implementation authorized

On 5 October 2026 Rudi instructed the implementation loop to proceed in this existing `personal-vps` project following the RFC/backlog. P1 attestation feasibility is verified in the measured UID-1000/native-agent layout; see `docs/attestation-feasibility.md`. This supersedes the previous documentation-only restriction.

Authorization covers project implementation and isolated testing. On 5 October 2026 Rudi separately authorized initializing Git and the initial publication to a new private `bungrudi/poc-spire` GitHub repository. This does not authorize future pushes, merges, migrations, deployments, collaborators or credential/access changes. Obtain specific approval before creating/retaining persistent credentials or host access, or changing host/network security settings. The ephemeral P1 runner retains no credentials. Rudi separately approved project-local PoC CA/agent state retention on 5 October 2026. That state now exists with owner-only permissions; no unrelated persistent access or deployment is authorized. M2–M4 tests pass. M5 is blocked on UI UID/agent-layout and HTTPS/OIDC configuration decisions; see `docs/implementation-status.md`.

Read in order:

1. `WORKSPACE.md` — confirmed requirements, recommendations, pending configuration
2. `docs/rfcs/0001-process-identity.md` — authoritative technical proposal
3. `TODO.md` — actual task status and acceptance gates
4. `docs/plan.md` — milestone order

`README.md` indexes the workspace; `docs/design.md` is a summary, not a competing specification. Keep documents consistent. The remote-access requirement supersedes the earlier localhost-only assumption.

## Security invariants for future implementation

- Caller-to-money TLS terminates **inside the Go money service**. No identity-terminating sidecar, forwarded identity header, plaintext fallback, or optional client-certificate mode.
- Authenticate real X.509-SVIDs, then authorize exact SPIFFE IDs by route before business logic. Trust-domain membership alone is not transfer permission.
- Clients verify the money service's exact SPIFFE ID. Use Go SPIFFE sources for rotation and verification; never use an application certificate-verification bypass.
- Unidentified clients fail TLS, with no money-service HTTP status. An authenticated but unauthorized identity receives `403`. Keep those cases distinct in logs/UI.
- Same UID, same container, image labels, and binary paths alone are not executable identity. Verify agent-derived executable-digest selectors plus deployment restrictions against a pinned SPIRE release before claiming process attestation.
- The two callers run in separate containers with the same UID/mapping and can reach both money and the Workload API socket. Include a controlled same-deployment wrong-executable test and a wrong-deployment approved-binary test.
- Do not invent plugin options or selector syntax. Prefer verified built-in attestors; a custom attestor requires explicit scope approval. Do not silently downgrade to container identity.
- Remote browser access requires HTTPS, validated OIDC login, backend operator authorization, CSRF/origin checks, and safe sessions. Missing configuration fails closed; no anonymous demo bypass.
- Callers have private, separate UDS trigger channels, not network trigger endpoints. The unidentified caller must not reach the trusted caller's control socket. UI may trigger callers but never borrows their credentials.
- The UI's distinct workload identity can read balances, not transfer. SPIFFE authenticates the process; OIDC authenticates the human. Neither replaces the other.
- Application containers receive no Docker/admin sockets or other workloads' credentials. The agent's required host/Docker privileges belong in the trusted computing base. A read-only Docker socket mount is not a read-only API.
- Fake money uses integer units, atomic updates, and identity-bound idempotency. Do not create a fresh transfer ID when retrying an unknown result.
- Keep credentials, SVID/private-key material, join tokens, OIDC tokens, cookie values, and runtime state out of git/logs/evidence.

## Working practices after coding is authorized

- Follow `TODO.md` gates in order. Prove attestation before building the demo UI; secure the UI before remote exposure.
- Use small Go packages, minimal dependencies, and deterministic tests. Run formatting, `go test ./...`, the race detector where supported, and the real-SPIRE Compose security suite when those exist.
- Preserve negative tests: wrong executable, wrong deployment, no cert, fake identity header, wrong valid SPIFFE ID, unauthorized human, CSRF, duplicate transfer, and expired credentials.
- Document exact versions, selectors, test evidence, and any deviations. Mark tests as unrun if unrun; infrastructure errors do not count as identity rejection.
- Update `TODO.md` only for completed work. Do not mark design recommendations as implemented.
- Keep the scope honest: fake-money PoC, not production payments or protection against host/SPIRE/authorized-process compromise. Executable attestation is not continuous runtime integrity.
