# Delivery plan

**Implementation authorized on 5 October 2026. M1 feasibility passes for the measured UID-1000/native-agent layout; M2–M4 internal tests pass; M5 awaits UI UID/agent-layout and HTTPS/OIDC configuration.**

The [RFC](rfcs/0001-process-identity.md) defines technical behavior. [TODO.md](../TODO.md) tracks completion. This document defines the dependency order. The current status claims measured M1–M4 internal results; remote UI and full acceptance remain open.

| Milestone | Deliverable | Exit gate |
| --- | --- | --- |
| M0: Design review | Workspace brief, RFC, backlog, topology/security decisions, deployment configuration choices | Owner reviews design and separately authorizes coding (Gate A) |
| M1: Attestation feasibility | Pinned runtime/SPIRE versions, observed selectors, same-UID eligible/ineligible processes, wrong-executable and wrong-deployment proof | Real evidence for executable **and** deployment discrimination (Gate B) |
| M2: Identity infrastructure | Go/Compose scaffold, SPIRE bootstrap, narrow registrations, protected secrets/state, SDK identity sources | Reproducible identity substrate, no remote exposure yet (Gate C) |
| M3: Money API | In-process mTLS, exact-ID route authorization, atomic in-memory ledger, idempotency, security tests | Negative identity tests and ledger invariants pass (Gate D) |
| M4: Callers and control | Separate caller processes, private trigger sockets, trusted transfer, no-SVID TLS probe, structured outcomes | Two intended outcomes demonstrated without UI (Gate E) |
| M5: Remote UI | Browser HTTPS, OIDC, roles, sessions, CSRF, limits, two buttons, read-only balances | Unauthorized humans cannot trigger the trusted process (Gate F) |
| M6: Verification and handoff | Full security matrix, sanitized evidence, runbook, reset/diagnostics, documented limitations | Owner accepts evidence and behavior (Gate G) |

## Sequencing rules

1. Resolve attestation feasibility before spending effort on a polished UI. Docker-only attestation cannot satisfy the stronger executable requirement.
2. Prefer verified built-in attestors. If a custom plugin is needed, stop for a separate design/scope decision.
3. Build direct mutual identity verification and exact authorization before business behavior.
4. Implement the remote security gate before publishing the UI. A temporary public anonymous mode is not an acceptable development shortcut.
5. Keep the two main callers at the same UID, with access to the Workload API and money service, so the demo measures identity rather than incidental reachability.
6. Never equate a network error with a successful security test. Capture the rejection stage and source of evidence.
7. Add runnable instructions only after they exist and have been tested. No estimated completion dates are asserted in this plan.

## Stop conditions

- Coding is now authorized. Persistent credentials/access and host/network security changes require specific approval before execution.
- Executable-bound selectors cannot be safely verified: request a scope/design decision; do not silently produce a container-only demo.
- Remote HTTPS/OIDC/operator mapping is missing: do not expose the UI remotely.
- Trusted/unidentified behavior differs only because of socket mounts, DNS, or network policy: fix the test rather than claim process authentication works.
- Any path can transfer based on a header, broad trust-domain membership, or an unauthenticated trigger: block acceptance.
