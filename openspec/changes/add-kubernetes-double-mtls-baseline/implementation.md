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
