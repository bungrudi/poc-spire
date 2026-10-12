# Planning and owner review record

Status: **planning authorized; implementation approval pending**.
Date: 12 October 2026, Asia/Jakarta.
Draft base: `98f0fa3bd1d26da80ca5cc283269533994315dfa`.
Planning branch: `codex/kubernetes-double-mtls-baseline`.

The parent orchestrator relayed Rudi's instruction:

> caller control i follow your suggestion. state, i dont care about durability of anything at the moment. prepare openspec for 1-3

Source parent chat: `01a12b31-9ead-7e92-a024-23860cfe5d8f`.
The direct owner message ID was not supplied in this delegation; none is
invented. This quote authorizes preparation of the plan. It is **not** approval
to implement/install/deploy/run a cluster or benchmark. The old Kafka approval
and standing commit/push authorization do not approve this new behavior.

Steps 1–3 are full Kubernetes placement (including Kafka and in-cluster SPIRE),
double-mTLS parity/security baseline, then a verified/pinned known-good freeze.
The owner accepted separate caller Pods/private authenticated controls and
disposable state. The later variant retains application TLS and Istio sidecars;
no comparison worktree or outer-TLS disable is part of this draft.

## Review unit

Review [proposal](proposal.md), [design](design.md), [profile](profile.md),
[seven deltas](specs/), [tasks](tasks.md), [acceptance](acceptance.md),
[references](references.md) and [structural validation](validation.md) together.
Decisions include one owned kind node/enforcing Calico, default Istiod CA,
independent SPIRE 1.14 built-in process+K8s gate, UID10002 control-only adapters,
public-bundle projection, disposable guarded reset and exact freeze/proof limits.

## Independent parent drafting review — PASS

The parent orchestrator independently read the complete proposal, short design,
profile, all seven deltas, tasks, acceptance mapping, references and validation
record on 12 October 2026. The scope covers steps 1–3 only and is ready for owner
review. No unresolved drafting finding remains.

Resolved findings preserve cached valid SVID behavior during issuer outage,
scope the existing Nginx clause separately from direct Kubernetes loopback HTTPS,
retain native bundle permission-before-replacement and authenticated Kafka
metadata reads, explicitly declare the Kubernetes-only JS-22 invalid-run outcome,
and separate real reachable issuance denial from normal-proxy mount inspection.
Kubernetes invalidation stops new Sync/Kafka UI/adapter dispatch with 503
run_invalid; the existing Compose Sync-survival result remains unchanged.

Independent checks passed: full-project strict validation (eight specs and two
changes), all 70 local links, whitespace, unchanged tracked source/main specs,
and exact preservation of 20 original scenario bodies across nine modified
blocks. The final draft contains 22 requirements, 70 scenarios and 23 unchecked
implementation tasks. All 28 KB cases and new Kubernetes executions of 23 JS
cases remain NOT RUN. Primary-source checks support the proposed pins; actual
attestation, secure kubelet, CNI/mesh/CSI and capacity remain implementation gates.

This PASS authorizes publication of these planning artifacts on the named branch.
It does not approve implementation, installation, runtime testing, deployment,
benchmarking or final baseline acceptance.

## Approval fields — pending

- Actual owner implementation-approval message/date: **none yet**.
- Approved artifact revision/manifest: **none yet**; planning publication is not implementation approval.
- Runtime/host installation/deployment authorization: **not granted by planning**.
- Material default/behavior rescope: requires review before dependent execution.

Before any implementation, present the full spec delta and short plan to Rudi,
resolve relevant scope questions and record the actual approving message/date
and frozen artifact revision. Parent PASS, a checked task, CLI apply readiness,
structural validation or permission to publish docs cannot substitute. No source,
main spec, prior approved artifact, primary checkout, public demo or retained
identity was changed to prepare this new change. Documentation publication is authorized by the independent parent review above;
implementation approval remains pending.
