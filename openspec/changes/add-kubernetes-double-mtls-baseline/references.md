# Sources, observations and proposed pins

Read on 12 October 2026, Asia/Jakarta. These are planning inputs, **not measured
Kubernetes compatibility** or installation/security acceptance. No tool/image was
installed or cluster started. Recheck supported patches/security and resolve all
artifact checksums/index/platform/config digests before approved execution.

## Host and code observed read-only

Base source/spec/evidence revision is `98f0fa3bd1d26da80ca5cc283269533994315dfa`.
The isolated implementation worktree was clean before this new planning branch.
Current [control](../../../internal/control/control.go) uses SO_PEERCRED, two
controls, 1024-byte input, 10s execution/12s writing and socket 0660; caller flags
allow operator UID 1000/10002. [Trusted](../../../cmd/trusted-caller/main.go) and
[unidentified](../../../cmd/untrusted-caller/main.go) hardcode `/workload/api.sock`;
unidentified uses `/workload/bundle.pem` and no source key. [Caller code](../../../internal/caller/caller.go)
requires observed no-identity denial, independent exact-Money verification and
no Money HTTP status for the no-cert case. [Identity](../../../internal/identity/identity.go)
uses updating sources, TLS1.3 and fresh nonresumed HTTP/1.1 handshakes.

[Private state](../../../internal/kafkacaller/store.go) requires owner-only
initialized files, stable descriptor/nonce and 64/four bounds. [UI Kafka](../../../internal/web/kafka.go)
uses authenticated metadata/all-variant receipts and unknown/reset rules.
[Native registrations](../../../scripts/identity.py) use Unix+Docker; their
config-digest selectors are not Kubernetes selectors. [Current acceptance](../../../docs/kafka-signing-acceptance.md)
and [independent review](../add-spiffe-kafka-message-signing/independent-review.md)
cover the existing profile only. They do not prove Kubernetes behavior.

Observed host: Linux 6.14.0-37-generic amd64; Docker 29.1.3/cgroup v2/overlayfs;
6 CPUs; 11675 MiB RAM/6564 MiB available/no swap; 35 GiB disk free. Docker already
serves other workloads/public demo. kubectl/kind/k3d/minikube/Helm/istioctl are
absent. Installation/capacity/privileges remain named future gates, not a pass.
Firecrawl lookup failed saved authorization; primary HTTPS lookup was used without
changing credentials or installing anything.

## Proposed reproducible defaults

| Component | Proposed pin | Basis / remaining gate |
| --- | --- | --- |
| kind | 0.33.0, Linux/amd64 Docker provider, one owned schedulable node | [Release](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0) provides explicit older node-image pins; default node version is deliberately not used. |
| Kubernetes/kubectl | 1.35.8 | `kindest/node:v1.35.8@sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0` from that release; verify image platform and API/tool checksum at KB-01. |
| containerd/runc | Bundled immutable node image; build defaults 2.3.4/1.4.3 | [Pinned Dockerfile](https://github.com/kubernetes-sigs/kind/blob/v0.33.0/images/base/Dockerfile) is source evidence, not runtime version observation. Inspect actual binaries/cgroup/PID mapping before claiming these versions. |
| Calico | 3.33.0, VXLAN/iptables, Kubernetes datastore | [Release](https://github.com/projectcalico/calico/releases/tag/v3.33.0) and [requirements](https://docs.tigera.io/calico/latest/getting-started/kubernetes/requirements) list tested 1.35–1.37. [kind configuration](https://kind.sigs.k8s.io/docs/user/configuration/) supports disabling default CNI. NetworkPolicy enforcement and Istio chaining still need real negative proof. |
| Istio/istioctl/CNI/proxy | 1.31.1, sidecar mode, default Istiod CA | [Patch](https://istio.io/latest/news/releases/1.31.x/) and [supported versions](https://istio.io/latest/docs/releases/supported-releases/) include Kubernetes 1.35 in the 1.31 range. Pin resolved images/revision and verify chained [CNI](https://istio.io/latest/docs/setup/additional-setup/cni/) composition. |
| SPIRE Server/Agent | Existing 1.14.0 retained | Built-in pinned [Unix](https://github.com/spiffe/spire/blob/v1.14.0/doc/plugin_agent_workloadattestor_unix.md), [K8s](https://github.com/spiffe/spire/blob/v1.14.0/doc/plugin_agent_workloadattestor_k8s.md) and [PSAT](https://github.com/spiffe/spire/blob/v1.14.0/doc/plugin_server_nodeattestor_k8s_psat.md) source defines candidate selectors/protocol. Actual kind/UID/process hashing compatibility is unproven. |
| SPIFFE CSI | 0.2.13 | [Release](https://github.com/spiffe/spiffe-csi/releases/tag/v0.2.13), [pinned README](https://github.com/spiffe/spiffe-csi/blob/v0.2.13/README.md). Resolve example/registrar/image digests before installation; CSI projects a read-only node socket directory and needs node-local hostPath backing. |
| Kafka | Existing apache/kafka 4.0.2 JVM image | [Current digest](../../../infra/kafka-signing/compose.yaml): index `sha256:836cafdad9f4825880d7cf1d5a21202915ae2527bd0ef1c3600c526ed7814d1f`; current stock TLS/DN ACLs remain. Real K8s/mesh/controller/bootstrap proof is required. |
| App/toolchain | Existing Go 1.24.2, Go SPIFFE 2.6.0, go-jose 4.1.5, franz-go 1.19.5/kmsg1.11.2; Node 22.23.0/jose 6.1.0/OpenSpec 1.14.1 for checks | [go.mod](../../../go.mod), [package lock](../../../package-lock.json). New adapter/image source must be committed before final evidence; no benchmark-only connection mode or unrelated dependency upgrades. |

Version compatibility in upstream support tables is a candidate, not a guarantee
that privileges, CSI/cgroups, secure kubelet or resource budgets work on this
host. Unknown exact registrar/image digests and realized limits are bounded pin/
feasibility tasks before execution/freeze. No fictitious checksum or tested result
is supplied. Re-resolving a material pin/layout must be recorded/reviewed.

## Security/behavior sources and named implementation gates

- [Kubeadm serving certificates](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/#kubelet-serving-certs): default self-signed kubelet certificates need explicit serving-TLS bootstrap and validated CSR approval; verify CA/hostname and authenticated access at KB-02.
- [SPIRE 1.14 K8s attestor](https://github.com/spiffe/spire/blob/v1.14.0/doc/plugin_agent_workloadattestor_k8s.md): secure10250/CA/token options, cgroup-v2 locator default and requesting-container Image OR ImageID semantics. Observe actual selectors; `pod-image`/Docker config digest cannot stand in for executable binding (KB-05–07).
- [Istio TLS connections](https://istio.io/latest/docs/ops/configuration/traffic-management/tls-configuration/) and [mesh TLS option](https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/): explicit inner TCP plus outer ISTIO_MUTUAL; `meshMTLS.minProtocolVersion: TLSV1_3` is the proposed outer minimum. Verify actual two-layer connections, not only configuration (KB-09/13–15).
- [Istio resource annotations](https://istio.io/latest/docs/reference/config/annotations/): native-sidecar injection is supported but marked alpha; scoped bootstrap Jobs must prove completion/shutdown on this exact pin (KB-03/23). App Pods retain regular sidecar mode.
- [CSI design/limitations](https://github.com/spiffe/spiffe-csi/blob/v0.2.13/README.md): node socket-directory lifetime and removal ordering are important; application Pods receive ephemeral inline volumes rather than hostPath. CSI removal before workload unmount can prevent clean termination (KB-05/27).
- [ConfigMap mounted updates](https://kubernetes.io/docs/concepts/configuration/configmap/#mounted-configmaps-are-updated-automatically): whole-directory projection updates eventually; subPath does not. Bound freshness/readiness and preserve valid cached trust without lending credentials (KB-08/16).
- [emptyDir lifetime](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir): container crash can retain volume, Pod replacement loses it. This is a run-scoped behavior, not durable recovery (KB-19–21).

Selected defaults are definite proposals, not behavior-changing open questions.
The real attestation, secure-kubelet, CNI/mesh/CSI composition and shared-host
budget checks are explicit implementation stop gates. Failure requires reporting
and reviewed rescope rather than weakening invariants. No runtime gate is run now.
