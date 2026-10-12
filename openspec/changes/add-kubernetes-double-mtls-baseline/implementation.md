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

Task 1.1 is complete. Artifact resolution is complete; actual node limits,
Calico/mesh/CSI, secure kubelet and executable attestation are not yet verified.
All KB/JS runtime acceptance remains NOT RUN. Source here is development source;
final source/configuration must still be committed before final acceptance.
