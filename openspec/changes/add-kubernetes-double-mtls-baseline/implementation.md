# Kubernetes implementation — 13 October 2026

Owner approval and its frozen 14-artifact manifest are in [review](review.md).
This is development/preflight evidence, not final migration acceptance.

## Artifact resolution and tools

[scripts/kubernetes_artifacts.py](../../../scripts/kubernetes_artifacts.py)
resolved the 12 reviewed upstream component images from their actual registries.
[Artifact catalog](../../../infra/kubernetes-baseline/artifacts.json) records
separate image index/platform/config digests and primary source checksums. The
explicit kind node index matches the owner-reviewed pin. Linux/amd64 config bytes
were hash-checked, rather than treating a Docker config digest as a K8s selector.

[scripts/kubernetes_tools.py](../../../scripts/kubernetes_tools.py) installed only
into ignored project cache after published checksum verification. Observed tools:
kind 0.33.0 (Go 1.26.7), kubectl 1.35.8 (Go 1.26.5), istioctl 1.31.1. Its pinned
release archive also includes the chart/manifests. No global packages/default
context or credentials were changed. Existing application dependencies and
connection settings are unchanged.

Primary release/support checks reconfirm kind's explicit node 1.35.8 and
Istio 1.31/Calico 3.33 coverage of that Kubernetes minor. The CSI 0.2.13 example
contains obsolete SPIRE/driver image tags and a verification bypass; those tags
and the bypass are not adopted. Its registrar 2.6.0 is resolved as a bounded,
local-only CSI registration dependency, with no app API credential or public
listener. Actual registration/compatibility remains a runtime gate. This is
an upstream-version and deployment-boundary assessment, not a claim of a full
vulnerability scan or measured interoperability. No material pin change occurred.

## Shared-host preflight and declared substrate

Fresh read-only preflight found approximately 6.5 GiB available, over 34 GiB disk
free, cgroup v2/Docker 29.1.3/amd64 and six CPUs. Ports 16443/18089 and Pod/service/
node ranges 10.245.0.0/16, 10.246.0.0/16, 172.31.244.0/24 have no observed collision.
Actual checks run again before creation, rejecting below 6 GiB/20 GiB. The node
is limited at Docker creation to 4 GiB memory/no swap and three CPUs; there is no
period of unlimited startup. A project-local Docker wrapper rejects foreign node
runs and resource overrides. The two ownership/resource regression tests pass.

Kind's required privileged node is part of the declared TCB. Its normal read-only
/lib/modules mount and anonymous owned /var volume are retained; apps receive
neither. Calico/CSI mount/network privileges and Agent hostPID/root/SYS_PTRACE
are confined to that owned node. No physical-host firewall, sysctl or daemon
configuration change is planned. Kubelet anonymous/read-only access is disabled;
serving TLS bootstrap is configured, and verified CSR approval is still NOT RUN.

Ownership markers, isolated kubeconfig and runtime diagnostics are private and
ignored. Teardown verifies ownership before any deletion, removes only owned
Docker resources and checks existing container PIDs/start times, public HTTPS,
default context and retained identity/permission equality. Private fingerprints
stay in the private run directory; sanitized evidence contains booleans only.

## Current gates

Tasks 1.1 and 1.2 are complete. Development bootstrap verified Kubernetes
1.35.8/containerd 2.3.4, ready Calico and Istio/CNI, root cgroup 4 GiB/3 CPU,
and corrected scheduler allocation 2500m/3434Mi with Pod memory.max 3584Mi.
Secure kubelet showed scoped token 200, valid unauthorized 403, malformed/
anonymous 401 and wrong CA/hostname denial. Six ownership/CSR/issuer regression
tests pass. Complete KB/JS runtime acceptance remains NOT RUN; actual executable
attestation, all application components and final resource/layer proofs remain.
Source here is development source;
final source/configuration must still be committed before final acceptance.

## Development corrections and honest pressure record

Kubernetes initially reported physical host capacity despite the Docker cap.
The owned kubelet now reserves the capacity above that cap for scheduling and
enforces the Pod cgroup; no approved node budget was raised. Unused kind storage
provisioner/StorageClass are removed because this profile uses no PVCs. Final
aggregate workload budgets and pressure acceptance are still NOT RUN.

The first disposable run (`994eea440006`) had a recorded memory.events max
counter of 2253, with zero OOM/kill counters. This is evidence of memory-limit
pressure/reclaim, not a no-pressure claim. Its development SPIRE startup failed
because overriding the native entrypoint omitted `run`; no issuance proof was
claimed. Explicit full-run cleanup passed public HTTPS, protected PID/start-time,
retained identity/permission and default-context checks. The corrected fresh
run (`ceb7d4978e30`) has independent trust/namespace/node ownership; its early
budget record reports max 2619 with zero OOM/kill counters. These are development
observations, not final known-good evidence.

Private diagnostic tails are bounded to 64 KiB and kept separately from the
cleanup target. One automatic permission review timed out before source/runtime
execution; splitting source preparation from deployment succeeded within the
same approved scope. No user approval or unsafe-action finding was involved.

The issuer helper records initialization start before creating resources and
refuses subsequent same-run bootstrap even if its Pod/namespace disappears.
A lost or incomplete issuer therefore requires explicit fresh run. The startup
helper checks the expected run/Pod UID and existing initialized issuer files;
apps have no write/API authority for that guard. Real destructive issuer/state
and all application guard behavior still require final acceptance.

## Actual early executable/CSI proof and application work

Fresh development run `b481f7eb59f6` has ready in-cluster SPIRE 1.14/PSAT,
Agent root+SYS_PTRACE, CSI 0.2.13 and verified kubelet. The native SPIRE readiness
commands now use their configured private socket paths; a healthy issuer was
not previously recognized by an incorrect default-socket probe. Explicit full
owned cleanup/fresh run preserved all protected continuity checks.

The actual UID1000 trusted-caller executable obtained its exact trusted SVID
under Unix SHA+K8s requesting-container image/name/namespace/SA/run/role/Pod UID
and exact PSAT node-parent conjunction. It performed a direct verified-Money
transfer (200, one 100-minor debit/credit). A separately registered UID10002
process with only the control ID received Money403. Same-container ineligible
executables received explicit Workload API PermissionDenied.

Bare early Pods have no controller-owner selector; this is NOT full KB-06/07.
The real adapter executable, wrong-deployment approved executable, final owner
conjunction, normal policy enforcement and full layer/migration proof remain.
The raw TCP boolean in the old fixture means local connectivity only: a closed
upstream through Envoy can also accept it. The controlled destination probe now
verifies exact Money using authentic public trust and requires the precise TLS
certificate-required alert with no HTTP. Its closed `money:65534` control keeps
local TCP success but fails destination verification. Temporary helper/public
bundle files in the owned caller control volume are removed afterward.

Docker29 .Id is an OCI index descriptor on this store, not a config. The stable
saved descriptor graph is decoded and hash/size verified through selected
linux/amd64 platform/config. Reconstructed OCI uses the verified filesystem,
User/entrypoint/labels and same config, with distinct actual index/platform
hashes. A digest alias is created only for the matching imported descriptor.
Actual CRI ImageID and final artifact provenance remain separate freeze inputs.

The new exact-ID adapter is work in progress: UID10002/own source, current UI
credential checks before/after bounded body, exact fixed fields/caller role,
shared two slots and original deadlines, own UDS only, bounded correlated reply,
and no redirects/reuse. Focused tests cover wrong peers, duplicate/extra/case-
alias input, expiry/run loss during body and original-ID unknown response.
UI transport selection is opt-in, uses only observer SVIDs, preserves UDS defaults
and shared parsers/quotas. Kubernetes-only guard checks stop Sync/Kafka creation/
retry and caller publication; historical status remains readable. The public-
bundle path flag retains its old default. Whole-directory projection, complete
readiness, state initialization/replacement/reset and all final real acceptance
remain pending. The new publisher parses complete current CA material and uses
verified API HTTPS with one named owned ConfigMap update; it is not deployed yet.

No task beyond approval/artifact resolution is checked by these development
observations. Final source/configuration and exact normal/test artifacts must
still be committed before the final acceptance run and independent review.

## Full-start capacity gate — 13 October 2026

Full-deployment source is committed/pushed at
`c39c13e4bb56d701ab042ff2ff260bbf1c4b82c7`. Go tests/race/vet, all 28 deterministic
Python checks and strict validation of eight specs/two changes pass. This is
source verification, not full application runtime acceptance.

The final development run was removed with verified owned cleanup and protected
HTTPS/container/retained-state/default-context continuity. Full fresh startup
then stopped before creating resources because host available memory was below
the owner-reviewed **6 GiB (6442450944-byte)** startup floor. Six later bounded
samples ranged from 6406586368 to 6432006144 bytes; the subsequent saved sample
was 6440288256 bytes and another exact preflight still rejected startup.
[Sanitized capacity evidence](../../../docs/evidence/kubernetes-preflight-current.json)
records the tested source, unchanged 4 GiB/3 CPU node cap and 2 GiB host reserve.
There is currently no owned active cluster/descriptor.

No unrelated workload was stopped, cache dropped, host setting changed or budget/
floor silently relaxed. Resume the full ordered bootstrap only after the original
floor is met. A different budget/floor/host arrangement would require an explicit
reviewed scope change before dependent execution. Full deployment/KB/JS runtime
acceptance and final independent review remain pending; task 7.1 is unchecked.


## Full development bootstrap and review corrections — 13 October 2026

The original capacity floor subsequently passed unchanged, and development run
`e643347540ac` reached healthy SPIRE/PSAT/CSI and completed/removed the injected
Kafka topic/ACL Job. Full application readiness failed: the exact owner selector
omitted the `ReplicaSet` kind required by the pinned SPIRE plugin. The pinned
upstream source and actual agent selector metadata independently confirmed the
required `pod-owner-uid:ReplicaSet:<uid>` format. Owner binding is retained.
The injector also defaulted application Pods to native sidecars; source now
explicitly requests regular sidecars for apps and reserves native mode for the
Job. This failed run was fully removed; public HTTPS, protected PID/start time,
retained identity/permissions and default context continuity all passed.

Review corrections bind native readiness markers to a live process start and
PID namespace, cap reads before allocation, and verify cached own SVID paths
with the existing SPIFFE SDK. SIGKILL/retained-marker and empty/unrelated/expired
trust checks pass; valid cached paths remain accepted without issuer access.
Normal images exclude adversarial fixtures and export-verified component/file
inventories; test/feasibility artifacts retain explicit fixture separation.

The trusted setup runner now observes original Pod/namespace/owner identities
and initialized caller/broker/issuer stores, preserving a monotonic invalidity
record and updating both read-only guards on detected loss. Caller publication
lock contention and exec/API/issuer availability failures are distinct from
observed destructive loss. Once invalid, same-run promotion is refused. Scoped
Pod annotations prompt projected guard refresh. No app API/admin mount is added.
Native broker readiness requires a verified TLS 1.3 Kafka response, not a local
TCP accept. Browser context binds both transports to the Kubernetes run; stale
keys require explicit new work and preserve the earlier unknown outcome. Default
Compose session/Sync behavior remains unchanged.

CSI mount intent is recorded before a possibly partial apply, so failure cleanup
still unmounts owned app workloads before node/driver removal. The full ordered
runner owns failure cleanup and the ephemeral observation process. These are
source-level/development corrections: real restart/outage/state-loss, exact
propagation, corrected sidecar placement, all KB/JS acceptance and final freeze
are still NOT RUN. Task 7.1 remains unchecked pending final parent review.


Focused review regressions plus the full normal Go race suite with explicit
independent Node JOSE enabled, vet, 37 deterministic Python checks, browser
control execution and strict 10-item OpenSpec validation pass for this source
checkpoint. The observation process and corrected topology still require the
fresh real run; this checkpoint closes no additional acceptance task.


The corrected-source development run `65d8fa0ac145` passed the exact startup
capacity sample, built an export-verified normal image with no test fixtures
from clean source `b11ede7`, and reached SPIRE/PSAT/CSI. It failed during app
registration and was automatically cleaned with all protected continuity checks
passing. Its precise failure was not retained before cleanup; it is not recorded
as an identity rejection or accepted runtime. New failure capture preserves only
bounded Pod/event categories and state metadata, never raw API/credential payloads.

A helper dependency review identified a size risk in the unchanged 32Mi issuer
helper volume. Caller-store checks are now separated from the minimal broker/
issuer checker, so the startup guard does not link the caller/SDK/broker library.
The three actual static issuer helpers total **27876344 bytes**, below the
33554432-byte volume bound. Installation checks that bound before any copying;
neither node nor volume budgets were raised. Exact loss/corruption/nonce and
held-publication-lock regressions remain passing. Actual corrected startup and
full acceptance must still be rerun; the earlier failure's exact cause remains
unconfirmed.
