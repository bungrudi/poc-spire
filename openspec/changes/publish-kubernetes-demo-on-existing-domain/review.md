# Actual owner authorization — 13 October 2026

The parent orchestrator conveyed Rudi's direct instruction:

> in the vps, kill unnecessary processes and cleanup, and make sure that url directed to k8s. ... probably merge first before we run public? also push

Provenance: parent thread `01a12b31-9ead-7e92-a024-23860cfe5d8f`, explicit delegation
on 13 October 2026. It authorizes merging/pushing the approved Kafka/Kubernetes
work, named `spire.adianto.dev` cutover and removal of precisely identified
superseded PoC runtimes. The parent explicitly instructs recording this existing
owner authorization without a second approval gate. This overrides the generic
proposal skill's planning-only pause for this already authorized concrete scope.

Artifacts in this directory describe that authorized scope; no additional
behavior, host access, HA/controller or global cleanup is authorized. Original
core approval/review and frozen source history remain in the earlier change.
The draft is committed before apply/runtime. Independent final verification is
by the parent; no agent review substitutes for human authorization/acceptance.

[Authorized-scope artifact snapshot](authorized-scope-artifacts.json) captures the
exact initial proposal/design/deltas/tasks before implementation. It records the
owner-authorized scope without inventing review of later agent-authored text.

Implementation: main fast-forwarded to `59e069d` after preserving local WIP.
Planning `be9c6b5`, clean normal build `51a9f85`, bounded ownership lifecycle fix
`e53c616` and verifier `cd3fb62` are declared independently; Go/application code
is unchanged. Named public checks and exact cleanup pass with modes/limits in
[public evidence](../../../docs/evidence/kubernetes-public-current.json).
Final parent route/source/cleanup verification PASS after publication, 13 October 2026.

## Independent final public-cutover review — 13 October 2026

Parent thread `01a12b31-9ead-7e92-a024-23860cfe5d8f` independently verified the live
HTTPS 200/ready/five-menu domain, exact verified Nginx loopback upstream, main-local
run/profile, active route, five ready Pods/zero restarts, eight retired containers
absent and four foreign containers preserved. It reviewed public profile/renewal/
unit/startup ownership, unchanged Go/application source, proof modes, ledger 0 → 2,
route/renewal continuity, twelve-file WIP/stash preservation and exact retirement/
retained-state checks. Its own strict validation passed 13/13, diff/no-PEM checks
passed. Technical public-cutover review is PASS; human acceptance is not inferred.
No additional runtime fixture, rebuild, cleanup or reliability work was performed
for this documentation-only final reconciliation.
