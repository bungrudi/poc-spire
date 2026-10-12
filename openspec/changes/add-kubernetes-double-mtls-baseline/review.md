# Planning and owner review record

Status: **owner implementation approval recorded; implementation in progress**.
Approval date: 13 October 2026, Asia/Jakarta.
The 12 October planning review below is historical; it did not grant this approval.
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

## Actual owner implementation approval — 13 October 2026

Rudi replied to the published, independently reviewed change with:

> go implement

- Direct owner message: `01a13221-82e9-72a0-a1ae-f3ed4505b4be`.
- Date: **13 October 2026, Asia/Jakarta** (parent turn started 05:00:19 +07:00).
- Source parent chat: `01a12b31-9ead-7e92-a024-23860cfe5d8f`.
- Approved artifact revision: `68351e7bbe655544c95ba9d845e1045cecf5aa6c`.
- Frozen normative artifact hashes: [approved-artifacts.json](approved-artifacts.json).
  The 14 entries are read directly from that Git revision; historical review and
  validation audit records are excluded. Progress/status edits do not replace
  the frozen owner-reviewed scope.

The parent explicitly relayed this approval and independently confirmed that it
covers the complete reviewed steps 1–3: project-scoped artifact/tool resolution,
owned disposable kind/Calico/Istio installation and isolated execution, in-cluster
SPIRE/CSI and real executable/deployment attestation, private control adapters,
full Kafka/application migration and all KB/JS acceptance, then reproducible
known-good freeze. The parent’s subsequent instruction to resume confirms that
the intervening progress question did not pause or cancel this approval.

The approved maximum remains one 4 GiB/3 CPU node, with the documented host
reserve. Physical-host firewall/sysctl/daemon changes, retained identity/public
runtime mutation, persistent access, comparison/outer-TLS-disabled variants and
benchmarking remain excluded. A material pin, privilege, budget or behavioral
rescope requires review before dependent execution. Standing commit/push
permission applies to authorized work on this branch. Independent final review
is still required: task 7.1 remains pending until the parent actually passes it.
