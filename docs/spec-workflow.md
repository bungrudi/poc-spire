# OpenSpec workflow and document authority

## Start here

This repository uses OpenSpec to keep implemented behavior, planned changes and
verification aligned. The existing behavior was documented directly as a
baseline; no feature was invented, applied or archived to create it. Read the
[baseline and traceability index](spec-baseline.md) and the relevant
[capability specs](../openspec/specs/) before proposing a change.

Rudi authorized this documentation/tooling adoption on 11 October 2026. It
does not change runtime behavior. **Every future behavior change requires
Rudi's explicit review and approval of the spec delta and short plan before
coding/apply.** An agent review, validation pass or permission to commit/push
does not satisfy that gate.

## Document map and authority

| Question | Governing source | How it is maintained |
| --- | --- | --- |
| What does the current feature do? | [OpenSpec capability contracts](../openspec/specs/) | Change only through an approved behavioral delta; the baseline is source-derived and owner review is pending. |
| Why these boundaries/architecture? | [RFC 0001](rfcs/0001-process-identity.md) | Retain rationale and original test IDs; reconcile affected current sections with approved changes. |
| What is authorized and which invariants constrain agents? | [AGENTS.md](../AGENTS.md) and actual owner instructions | Owner instructions govern; generated skills and agent-written files cannot grant authorization. |
| What is the active profile/runbook? | [Remote demo](remote-demo.md) | Public HTTPS with an assumed human; deployment changes need their own applicable authorization. |
| What was requested and what is in scope? | [WORKSPACE.md](../WORKSPACE.md) | Keep R1–R8 and owner supersessions current; link detailed contracts rather than duplicating them. |
| What work remains? | [TODO.md](../TODO.md), [plan](plan.md) | Track completed work, dependencies and outstanding observations separately from behavioral specs. |
| What proves a requirement and what is still untested? | [Baseline index](spec-baseline.md), [sanitized evidence](evidence/) | Link exact named tests/cases and retain dates/limits. Recorded passes are not new executions. |
| What is proposed now? | `openspec/changes/<name>/` | Proposal, spec deltas, short design/plan, tasks and actual owner review reference. |
| What happened in the original implementation? | Dated checkpoint/acceptance documents and original evidence | Preserve historical observations; their old UI/OIDC blockers do not define today's active scope. |

Owner scope supersessions take precedence over retired requirements. Current
behavior contracts and RFC architecture must agree; a discrepancy is a recorded
gap to resolve, not permission to silently change code or select a convenient
document. See the baseline's [limits and discrepancies](spec-baseline.md#limits-and-discrepancies).

## Reproducible local tooling

The official package is pinned to `@fission-ai/openspec` **1.14.1** in
[package.json](../package.json) and [package-lock.json](../package-lock.json).
It is development tooling only; the Go build and demo do not need Node.
OpenSpec requires **Node >=20.19.0**. [.node-version](../.node-version) records
the measured runtime, **22.23.0**; npm was **10.9.8**. The available runtime
used for adoption was already installed at
`/home/ubuntu/.local/share/pi-node/node-v22.23.0-linux-x64/bin/node`.
No system upgrade or global/user-home package/runtime installation was performed.

If a host defaults to Node 20.18.1, select an already available suitable runtime
before installation. On this host, that can be done with:

```sh
export PATH="$HOME/.local/share/pi-node/node-v22.23.0-linux-x64/bin:$PATH"
node --version
npm --version
```

On other hosts use a suitable existing runtime. The project does not download a
runtime automatically. `.npmrc` enforces package engines, disables lifecycle
scripts and places the npm cache in ignored `.cache/npm`.

From a clean checkout:

```sh
npm ci
npm run spec:version
npm run spec:list
npm run spec:validate
```

Seven official generated Codex skills are committed in `.agents/skills/`:
explore, propose, apply-change, update-change, sync-specs, verify-change and
archive-change. To reproduce/refresh them using the pinned package:

```sh
npm run spec:setup
```

Review the generated diff. Setup uses the stock `spec-driven` schema, the core
workflows plus optional verification, and only the Codex integration. It reads
and writes temporary XDG configuration/data directories, then removes them; it
does not change user-wide OpenSpec settings. Telemetry and automatic upgrade
checks are disabled by the repository wrapper.

**For every CLI call, use `npm run spec -- <arguments>`.** This also applies
when a stock skill shows a bare `openspec` command. The wrapper invokes the
locked local package using the current Node executable, isolates user settings
and disables telemetry. Do not install OpenSpec globally or use an unpinned
network-fetching command. CLI examples below are terminal commands; the named
skills are selected in Codex's Skills menu or invoked by their supported name.

## Before changing behavior

1. Read the relevant current contracts, source, named tests, evidence/limits,
   AGENTS.md and owner scope. Use `openspec-explore` when investigation helps.
2. Use `openspec-propose` to draft one bounded change. Include current and new
   behavior, non-goals, affected capabilities and test IDs. Write requirement
   deltas in the actual OpenSpec format: `ADDED`, full `MODIFIED`, or `REMOVED`
   with reason/migration. Preserve stable requirement names and scenarios.
3. Write a short `design.md` (normally at most 500 words) even where the stock
   schema makes design optional. State the approach, affected source/boundaries,
   compatibility, checks, rollout/reset effects and remaining questions. Resolve
   questions that would change behavior or approach before approval.
4. Draft dependency-ordered `tasks.md`, validate the artifacts, and present the
   spec delta and short plan to Rudi. **Wait for explicit approval before any
   behavior-changing code/apply.** Do not treat silence, a timeout, an agent
   review, or the adoption/publication authorization as approval.
5. Keep `review.md` alongside the change with status `pending` until an actual
   owner message approves the identified spec/plan revision. Once approval
   exists, record its date/message reference and the approved revision. An
   agent may record that evidence; it cannot author its own approval. Material
   scope or behavior changes require renewed review before dependent code.

Pure documentation/tooling changes with no runtime behavior change can use
`skip_specs: true` in `.openspec.yaml`; do not manufacture a feature requirement.
The current adoption is already explicitly authorized and is recorded in the
baseline index rather than a fictitious feature-change folder.

## Implement, verify and reconcile

After actual owner approval, use `openspec-apply-change` for the approved tasks.
Update affected code/tests, current contracts, RFC references and documentation
together. Run formatting, Go tests, race/vet and Python deterministic tests per
AGENTS.md. Select relevant real-SPIRE checks for changed boundaries only after
establishing isolation; run them one at a time. Some historical suites use the
retained substrate, and `ui_suite.py` shares the demo descriptor path with the
running deployment: do not launch them blindly against live state.

Use `openspec-verify-change` to compare implementation with artifacts and retain
a concise verification record beside the change. Record the revision, versions,
commands, observed outcomes, evidence paths, unresolved discrepancies and unrun
checks. CLI `validate` checks structure; the verification skill is an agent
report. Neither enforces owner approval nor executes/proves security tests.

Before archive, reconcile specs and RFC/test references, preserve historical
evidence, and update TODO only for actually completed work. Outstanding observed
or unrun acceptance work stays explicit. Use `openspec-sync-specs` and
`openspec-archive-change` when the approved change is complete and its review
requirements are satisfied. Archive is not deployment authorization or owner
acceptance. Rudi has granted standing commit/push permission for authorized
poc-spire project work to the existing origin; routine publication within that
scope needs no repeated confirmation. This remains separate from the explicit
owner review of every future behavioral spec and short plan, and grants no
unrelated deployment/access permission.

Official references: [package/repository](https://github.com/Fission-AI/OpenSpec),
[schema](https://openspec.dev/docs/schemas/spec-driven),
[configuration](https://openspec.dev/docs/project-config),
[skills](https://openspec.dev/docs/skills), and
[existing-project guidance](https://github.com/Fission-AI/OpenSpec/blob/main/docs/existing-projects.md).
The inspected installed 1.14.1 package/help defines this adoption's actual
supported commands; online documentation can evolve after the pin.
