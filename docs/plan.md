# Delivery plan

## Current source change — 18 October 2026

The approved `remove-direct-app-mtls` change uses SVID-signed HTTP in Compose and
Kubernetes, with separate SPIRE-issued Envoy SVIDs for mesh mTLS. Current contracts
are in OpenSpec; the older milestones below retain their original observations.
Implementation and focused checks precede a disposable mesh issuer/rotation run.
No live cutover is authorized by this source change.

## Owner scope update — 11 October 2026

The owner requested deployment at `spire.adianto.dev` and then explicitly
removed OIDC, asking to assume human authentication to focus on SPIRE. The
current HTTPS demo labels its operator as **assumed, not authenticated**;
anyone with browser access may trigger fake transfers. The original OIDC and
verified-human acceptance requirements below are historical and superseded
for this demo. Workload attestation, direct mandatory mTLS, exact-ID Money
authorization, separate caller control mounts, HTTPS and CSRF checks remain.
The isolated UI uses the measured UID-1000/native-agent layout. See
[the current demo and deployment plan](remote-demo.md). The owner specifically approved the HTTPS certificate/key, Nginx route,
renewal hook, persistent service and public access without human login. Those
changes are installed and verified at `https://spire.adianto.dev/`.

The earlier checkpoints below record their original scope and results.


**Current simplified demo: UID-1000/native-agent M1–M4 and HTTPS UI tests pass; the specifically approved spire.adianto.dev deployment is live and verified. Human authentication is explicitly assumed.**

The [current contracts](../openspec/specs/) define implemented feature behavior;
the [RFC](rfcs/0001-process-identity.md) retains architecture/security rationale.
[TODO.md](../TODO.md) tracks current work separately from the historical milestone
checklist. This document preserves the original dependency order. The simplified
remote UI/deployment has recorded passing evidence; original verified-human/full
acceptance is retired and is not claimed. See [the baseline index](spec-baseline.md)
for exact limits and [the workflow](spec-workflow.md) for future owner-reviewed changes.

| Milestone | Deliverable | Exit gate |
| --- | --- | --- |
| M0: Design review | Workspace brief, RFC, backlog, topology/security decisions, deployment configuration choices | Owner reviews design and separately authorizes coding (Gate A) |
| M1: Attestation feasibility | Pinned runtime/SPIRE versions, observed selectors, same-UID eligible/ineligible processes, wrong-executable and wrong-deployment proof | Real evidence for executable **and** deployment discrimination (Gate B) |
| M2: Identity infrastructure | Go/Compose scaffold, SPIRE bootstrap, narrow registrations, protected secrets/state, SDK identity sources | Reproducible identity substrate, no remote exposure yet (Gate C) |
| M3: Money API | In-process mTLS, exact-ID route authorization, atomic in-memory ledger, idempotency, security tests | Negative identity tests and ledger invariants pass (Gate D) |
| M4: Callers and control | Separate caller processes, private trigger sockets, trusted transfer, no-SVID TLS probe, structured outcomes | Two intended outcomes demonstrated without UI (Gate E) |
| M5: Simplified remote UI | Browser HTTPS, declared assumed operator, sessions, CSRF, limits, two buttons, read-only balances | Real-SPIRE UI and public caller tests pass; original human Gate F retired by owner |
| M6: Verification and handoff | Bounded current-profile evidence/runbook and explicitly unrun observations | Owner reviews scope and evidence; original full verified-human Gate G is not claimed |

## Sequencing rules

Before each future behavior change, prepare the spec delta and short plan and
obtain Rudi's explicit approval before code/apply. Within approved work, retain
these dependency rules:

1. Resolve attestation feasibility before spending effort on a polished UI. Docker-only attestation cannot satisfy the stronger executable requirement.
2. Prefer verified built-in attestors. If a custom plugin is needed, stop for a separate design/scope decision.
3. Build direct mutual identity verification and exact authorization before business behavior.
4. The current public assumed-operator profile is an explicit owner scope change, not a configuration fallback. Retain HTTPS, disclosure and browser controls. A future restricted profile needs its own verified-human gate.
5. Keep the two main callers at the same UID, with access to the Workload API and money service, so the demo measures identity rather than incidental reachability.
6. Never equate a network error with a successful security test. Capture the rejection stage and source of evidence.
7. Add runnable instructions only after they exist and have been tested. No estimated completion dates are asserted in this plan.

## Stop conditions

- Coding is now authorized. Persistent credentials/access and host/network security changes require specific approval before execution.
- Executable-bound selectors cannot be safely verified: request a scope/design decision; do not silently produce a container-only demo.
- Missing or invalid browser HTTPS configuration: fail startup. Human authentication is explicitly assumed only in this approved profile.
- Trusted/unidentified behavior differs only because of socket mounts, DNS, or network policy: fix the test rather than claim process authentication works.
- A transfer based on a claimed header, broad trust-domain membership, unverified workload or exposed caller network trigger blocks acceptance. The owner-approved public browser session deliberately assumes its human.

## Approved Kafka iteration — 12 October 2026

The isolated OpenSpec iteration implements signing/shared guards and receipts,
private retained recovery, fixed combined UI, native-SPIRE/Kafka Compose and
serial real/deterministic acceptance. See [coverage](kafka-signing-acceptance.md)
and [tasks/review](../openspec/changes/add-spiffe-kafka-message-signing/tasks.md).
Final source is frozen before the evidence run; documentation/evidence are
reconciled afterward and sent for independent review. No public rollout is
included, and the original durable-state/production limitations remain.
