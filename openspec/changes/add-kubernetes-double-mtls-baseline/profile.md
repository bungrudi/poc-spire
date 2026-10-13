# Proposed Kubernetes double-mTLS baseline

**Owner approved implementation on 13 October 2026; see [review](review.md).
Kubernetes proofs remain NOT RUN until their actual checks complete.**
The [proposal](proposal.md), [short design](design.md), [deltas](specs/),
[acceptance](acceptance.md), [tasks](tasks.md), [references](references.md) and
[review](review.md) are one review unit. Existing main specs/approved artifacts
and the public deployment are unchanged.

## Scope and proposed runtime

Steps 1–3 are: complete Kubernetes placement; prove the five scenarios and focused
core security with both TLS layers; freeze a reproducible known-good PoC baseline.
The owner's 13 October narrowing in [review](review.md) supersedes exhaustive
Kubernetes reliability parity. Failed/corrupt run means operator whole-cluster
stop/recreate. Automatic store-integrity monitoring, exhaustive recovery and
failure injection are DEFERRED; existing useful guards need not be removed. This
change does not create the later comparison worktree, disable outer TLS, run a
benchmark, change connection modes, provide HA/durability or cut over the public
endpoint. Both eventual variants retain application TLS and Istio sidecars.

Propose one uniquely named, disposable kind 0.33.0 control-plane node, schedulable
for workloads, Linux/amd64 Docker provider with bundled containerd. Explicitly use
`kindest/node:v1.35.8@sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0`;
never the release's newer default. Use Calico 3.33.0 standard VXLAN/iptables,
Kubernetes API datastore, with kind's default CNI disabled. It must actually
enforce NetworkPolicy. Istio 1.31.1 runs sidecar mode with its CNI chained after
Calico; ambient/ztunnel and SPIRE-for-Istio are excluded. Component images,
charts/manifests and tool checksums must resolve to immutable artifacts before
implementation runs; [references](references.md) separates proposed pins from proof.

Read-only host observation: 6 CPUs, 11675 MiB RAM, 6564 MiB available, no swap,
35 GiB free on the root disk; Docker 29.1.3, cgroup v2, overlayfs, amd64 and Linux
6.14.0-37-generic. kind/kubectl/Helm/istioctl are absent. This is not a capacity
pass. Proposed startup floor is 6 GiB available/20 GiB free, preserving a 2 GiB
host reserve. Bound the owned node to at most 4 GiB/3 CPUs, requests plus control
plane/bootstrap/fixture peak below that budget, Kafka JVM heap 256–512 MiB,
bounded emptyDir sizes and ephemeral storage. Actual allocatable/cgroup limits,
RSS/peak/OOM/CPU pressure and public-demo continuity must pass KB-01/KB-25.
If not, stop; do not kill other workloads, alter host firewall/sysctl/daemon
settings or silently add nodes/resources. This one-node topology makes no
performance, production or cross-node claim.

## Placement, identity and mount boundaries

Namespaces are owned per run: `poc-<run>` (apps) and `spire-<run>` (SPIRE).
Istio and Calico system namespaces belong only to this disposable cluster.
A unique cluster name, kubeconfig/descriptor, Docker node/network, namespace
UIDs and run/stream nonces define ownership. No default kubecontext mutation,
public LoadBalancer/NodePort/Ingress, host broker/API binding or physical-host
SPIRE process is used. Kubernetes browser access is direct Go UI HTTPS through loopback forwarding,
without installing/reusing the existing public Nginx edge. That edge retains its
existing upstream verification/renewal contract (UI-02).
The cluster API and browser forwarding bind literal
127.0.0.1; browser default is separate loopback HTTPS port 18089.

| Component | Proposed layout | Mesh injection / credentials |
| --- | --- | --- |
| UI | One Pod, UID/GID 1000, own SA `ui` | Sidecar; observer SVID only, browser server key; no caller state/UDS/broker/admin credentials. |
| Trusted caller | One Pod, UID/GID 1000 `caller` container, SA `caller-trusted` | Sidecar; exact trusted app SVID, own broker cert/state/local UDS. |
| Unidentified caller | Separate Pod, UID/GID 1000 `caller`, SA `caller-unidentified` | Sidecar; own conventional broker cert/state/local UDS, public SPIRE bundle; **no app SVID/key**. |
| Control adapters | One additional container in each caller Pod, UID 10002/GID 1000 | Shared Pod sidecar/SA; own narrowly attested control SVID/CSI mount, own local UDS only; no broker key or private caller state mount. |
| Money | One Pod, UID/GID 1000, SA `money` | Sidecar; exact Money app SVID and own broker client cert; one embedded consumer/member/partition/shared memory ledger. |
| Kafka | One stock JVM Pod, UID/GID 1000, SA `kafka` | Sidecar; conventional broker/server/controller TLS; one KRaft node, one topic partition, bounded disposable data. |
| Broker bootstrap | Bounded Job, SA `kafka-admin`, own temporary admin cert | Sidecar/native-sidecar Job lifecycle; broker network only, no application SVID or transfer permission. |
| SPIRE Server | One Pod, ephemeral CA/SQLite/key state, private admin UDS | Explicitly uninjected; PSAT verifier, scoped TokenReview/node metadata RBAC, no app mounts. |
| SPIRE Agent/CSI | DaemonSet on the owned node, PSAT, built-in Unix/K8s attestors | Explicitly uninjected trusted substrate; node PID/proc visibility and verified kubelet, CSI node registrar. |
| Registration/bundle bootstrap | Bounded admin operation inside Server Pod plus restricted bundle publisher | Uninjected; private SPIRE admin socket, scoped registration and one public ConfigMap write. No dependency on app mesh identity. |
| Istiod/CNI/Calico/Kubernetes control plane/DNS | Standard pinned in-cluster system components | Explicit injection exclusions; their own native security protocols/RBAC, not business application identities. |

App containers drop all capabilities, disallow privilege escalation, use read-only
roots and RuntimeDefault seccomp, with no hostPID/hostNetwork/admin mounts/API
credentials. Projected Istio tokens are mounted only into the proxy, not apps.
Adapters/callers share Pod networking and mesh SA identity; mesh cannot isolate
these two processes. Keep process-specific inner SVIDs, no shared process namespace,
no credential lending and separate mounts. GID 1000 permits caller socket mode 0660
without changing SO_PEERCRED: caller still allows adapter UID 10002. Its state
is an init-created child directory 0700/files 0600, owned UID 1000; adapter never
mounts it. UI mounts neither caller socket nor state.

Kind nodes, Calico, Istio CNI and SPIRE Agent/CSI are the trusted substrate.
Node-local hostPath/socket/proc/kubelet plugin mounts required by CSI are confined
to the owned Docker node, not physical-host mounts into apps. Socket backing
must be node-local hostPath, not an Agent Pod emptyDir that breaks existing CSI
bind mounts on restart. Proposed Agent privilege starts with root, hostPID,
read-only proc and CAP_SYS_PTRACE needed to inspect different-UID executables;
CSI/CNI mount/network privileges are explicit. Verify exact least-required
security contexts in KB-02/25. Any broader privilege or unplanned physical-host
security change needs renewed review, not a silent feasibility workaround.

## Application attestation feasibility gate

Use pinned SPIRE 1.14.0 server/agent images, `k8s_psat` node attestation, bounded
projected token audience/lifetime and the exact owned cluster/Agent SA/node UID.
No join-token bootstrap or native host daemon. Registration binds exact attested
parent plus built-in `unix:uid`, `unix:sha256` with `discover_workload_path=true`
and a finite positive binary size bound. Add `k8s:ns`, `k8s:sa`,
`k8s:container-name`, requesting-container `k8s:container-image`, run/role labels,
and observed Pod/owner UID. Values come from actual attestation; do not use
`k8s:pod-image` (another container) or Docker image-config-digest syntax.
Distinguish OCI index, platform manifest, config digest and the Image/ImageID
selector actually exposed by Kubernetes. Reject a tag-only/nonimmutable grant.

The K8s cgroup-v2 locator is enabled, but documentation is not proof of actual
peer PID visibility. Before dependent UI/migration work, observe UID/hash/container
selectors and real narrowly granted SVIDs for app UID 1000 and adapter UID 10002.
Run a wrong binary inside the trusted app container, and approved binary in a
wrong SA/deployment, retaining same caller UID/image, actual CSI socket and
Money reachability under narrowly declared test policies. Wrong binary/role must
receive explicit no-matching-identity PermissionDenied, not timeout/permission/
mesh/network failure. Reachable adapter tests must show only its control identity, never UI/trusted app
identity, and an ineligible executable in its container must be denied. The normal
proxy has no Workload API/CSI socket or application SVID/key mount: inspect that
structural boundary, not a fictitious PermissionDenied from an unreachable socket.
Its own Istio mesh credentials are expected. A reachable proxy issuance-denial
claim would require a separately declared test fixture; none is proposed here.
Rebuilt binaries require explicit digest/owner updates; no broader duplicates.

Kubelet access uses secure 10250, authenticated bounded API-audience token and
verified CA/hostname. Propose kubeadm `serverTLSBootstrap=true`; scoped bootstrap
approves only serving CSRs for this node, expected signer/subject/SANs. Default
self-signed kubelet certificates are not assumed valid under the API CA. Prove
actual serving-chain/hostname trust and least RBAC. No insecure port, anonymous
authentication or `skip_kubelet_verification`. If these checks fail, stop the
implementation at a named gate; no container-only downgrade/custom attestor.

## Exact flow, TLS and authorization contract

Inner TLS is independent of outer mesh. For Money, TLS 1.3/X.509-SVID terminates
inside Go; exact app IDs retain route permissions before history. For adapters,
UI's own observer SVID verifies exact `spiffe://poc.internal/control/caller/trusted`
or `/unidentified`; adapter accepts trusted-domain mTLS, then authorizes only
`spiffe://poc.internal/ui` before body parsing/UDS access (wrong valid ID 403,
no cert TLS failure). Adapter identity cannot transfer or read Money; a valid
control SVID is not the trusted application signer. Go sources rotate normally.

Outer identities are default Istiod-CA identities such as
`spiffe://cluster.local/ns/poc-<run>/sa/ui`; trust domains are separate. Require
namespace `PeerAuthentication STRICT`, exact service/port `DestinationRule
ISTIO_MUTUAL`, mesh minimum TLS 1.3, and default-deny L4 authorization with explicit
SA/port allowlists. Declare encrypted application ports as opaque TCP; do not
terminate/inspect inner HTTP or JWS in Envoy or authorize from forwarded headers.
Use no mesh HTTP retries, application TLS origination, wildcard permissive route
or application-port bypass in the normal baseline.

| Flow | Inner protocol/permission | Outer / isolation |
| --- | --- | --- |
| Browser→UI8443 | Server-auth HTTPS, temporary browser CA; assumed human/session/CSRF/Host/Origin remain | Local forwarding only. Explicit UI8443 mesh exception (`DISABLE` inbound mTLS plus scoped frontend allow); this is not inter-workload traffic. |
| UI→Money8443 | Own observer SVID verifies exact Money; GET balances/outcomes only | ISTIO_MUTUAL/STRICT; mesh allows UI SA; application denies POST transfer403. |
| Trusted caller→Money8443 | Own trusted app SVID verifies exact Money; fixed POST transfer | ISTIO_MUTUAL/STRICT; caller-trusted SA allowed. |
| Unidentified caller→Money8443 | Authentic public bundle verifies exact Money, no client cert; TLS rejection/no HTTP | Same outer mesh succeeds for caller-unidentified SA, so app rejection is measurable. |
| UI→each control9001 | Exact adapter/UI app SVIDs, fixed private control schemas | ISTIO_MUTUAL/STRICT; only UI SA normal ingress; UI verifies which adapter. |
| Adapter→its caller | Local per-Pod UDS, kernel UID 10002; fixed routes/timeouts | No TCP/mesh TLS needed; no other caller control mount. |
| Each caller/Money→Kafka9093 | Stock conventional TLS 1.3, hostname `kafka`, own DN/topic/group ACL | ISTIO_MUTUAL/STRICT; exact caller/Money SAs. UI→broker is denied by enforcing NetworkPolicy and mesh; same-Pod adapters have no broker client key/grant despite shared Pod reachability. |
| Scoped admin Job→Kafka9093 | Own admin conventional TLS, exact topic/ACL bootstrap | Injected, mesh/admin SA allowed only for bounded job; no app admin credential. |
| Kafka→own controller9094 | Same-Pod/local conventional authenticated controller TLS | Explicit local-only exception; no external Service for9094. No replicas/controller inter-Pod flow. |
| Apps/adapters→Workload API | Read-only CSI-injected node socket, peer process attestation | Local UDS, no Istio. Proxy lacks this mount. |
| Agent→SPIRE Server / Server→API / Agent→kubelet | Native SPIRE TLS/PSAT and verified Kubernetes HTTPS/RBAC | Uninjected system paths; not required to obtain Istio/SPIRE app credentials first. |
| Proxies→Istiod15012; node components/API/DNS | Native authenticated xDS/SDS/Kubernetes protocols, explicit RBAC/network flows | Explicit system allowances, not application transfer permission. |

Because both caller containers and adapters share a Pod, NetworkPolicy cannot
prevent that Pod's adapter from opening an otherwise permitted broker/Money
connection. Own inner identity and absent broker credentials enforce that boundary;
never claim network isolation between same-Pod processes. Normal app namespace
allows DNS, sidecar xDS/SDS, specified Money/control/broker flows and nothing else.
Enforcing CNI negatives must demonstrate UI→broker and cross-caller controls deny.
Temporary negative-probe policies are named, narrowly scoped, restored and hashed.

## Fixed adapters, public bundle and compatibility

Adapters have no generic proxy/signing/destination API. Mirror only POST `/attempt`
with request_id; POST `/attempt/kafka` with request_id/scenario/money_instance_id;
GET `/attempt/kafka/{id}`. Same strict 1024-byte input, ID/field/role restrictions,
8192-byte correlated responses, two shared in-flight controls, POST 10s/GET 2s
end-to-end caps remain. Forward only the fixed own UDS, with remaining deadline;
no inner TLS termination/credential borrowing on the Money path. UI keeps its
64 attempts/two workers/eight dispatches per minute/128 sessions and existing
12s Sync / 30s Kafka worker behavior. Broker ack remains publication diagnostics;
only exact-Money mTLS/all-variant receipts prove Kafka outcome. Invalid JWS is
unattributed, query HTTP status is separate from business disposition.

Current callers hardcode `/workload/api.sock` and unidentified bundle
`/workload/bundle.pem`. CSI supplies the socket filename at `/workload/api.sock`;
verify this mount in the feasibility gate. Add a deployment-configured bundle
path flag with existing default unchanged; Kubernetes uses `/trust/spire/bundle.pem`.
A restricted publisher beside SPIRE Server obtains the authentic public bundle
through its private admin UDS, validates complete CA PEM for this run/domain,
then updates only the named public ConfigMap. Mount its whole directory read-only
(no subPath) so kubelet's atomic projection follows updates. No private key,
SVID or API/admin credential is mounted into the unidentified caller. Failed
refresh preserves last complete valid data; bounded staleness/readiness is
reported explicitly. Missing/wrong/out-of-date trust cannot become claimed
attestation or no-certificate rejection. Consumer/UI use independent SDK bundles.

## Bootstrap and readiness dependency edges

1. Scoped host preflight/owned descriptor and immutable pins → create owned kind
   node with loopback API, budget limits and serving-certificate bootstrap.
2. Enforcing Calico/DNS/node readiness → scoped kubelet serving CSR approval;
   verify API/kubelet HTTPS and NetworkPolicy negative control.
3. Istiod/CNI/injection admission and SDS readiness → app namespace policies.
   SPIRE namespace/system bootstrap is explicitly excluded from injection.
4. SPIRE Server CA/private admin readiness → public bundle publication and scoped
   Agent PSAT registration/attestation → node Workload API/CSI registration.
5. Create app Pods with init barriers; collect their fixed Pod/owner UIDs, set
   expected run guard, then release bounded initialization/startup waiting for
   registrations. Observe the started requesting container Image/ImageID and
   exact process selectors, establish/verify narrow entries and prove issuance.
   Do not require an ImageID/SVID from an app that is still blocked in init.
   Kubernetes startup may use a bounded profile-specific wait; existing Compose
   timeout remains. Prove app/adapter attestation before depending on UI behavior.
6. Temporary conventional broker/browser PKI → Kafka storage/advertised Service,
   TLS and sidecar readiness → injected admin Job topic/partition/DN ACL bootstrap.
   The bootstrap Job needs Istio identity, not application SPIRE SVID; use pinned
   native-sidecar job lifecycle and bounded deadline/cleanup, not a hanging Job.
7. Caller private manifest/index init, exact sources/control mTLS and local UDS →
   Money source/own broker TLS/consumer assignment → UI own source, adapter lookup,
   exact-Money balances/outcomes/stream metadata and browser TLS → full readiness.

Liveness is not dependency readiness. Initial startup must establish issuing
infrastructure, but later issuer/API watch failure alone does not invalidate
valid cached SVIDs or independent accepted bundles. Such outage is degraded,
not identity rejection: cached operations remain usable until their actual
validity/guard boundary (IC-03). Readiness fails for genuinely unavailable required
operations/material or detected destructive run reset, not merely a disconnected
issuer. Missing trust/expired credentials still fail closed.

Mandatory-Money-mTLS endpoints cannot be
probed anonymously with kubelet HTTPS and counted healthy. Add scoped native
readiness plus an in-image exec checker/marker (no transfer/signing capability)
that reports current source/run/control/consumer state; UI readiness additionally
uses its own authenticated reads. Verify sidecar ready/SDS current, correct socket,
initialized state, TLS/ACL/topic, assignment and all required reads. Dependency
failure disables readiness/dispatch with accurate unknown/infrastructure status,
not plaintext fallback. Readiness/init gates must not require an identity grant
before Pods exist to supply their attestation metadata.

## Disposable state and reset unit

One run owns its cluster, namespaces, temporary trust, stream/topic, caller stores
and browser context. Keep bounded state and existing useful startup/run guards;
applications have no authority to amend them. If the run fails or is corrupt,
the operator MUST stop the whole owned cluster and explicitly recreate a fresh
run/trust/stream. Do not silently repair state, broaden registrations, recreate
old attempts or claim continuity. Old outcomes may remain unknown.

Automatic destructive-loss detection, issuer database/key integrity or lineage,
recovery through every same-Pod/crash-loop variant and exhaustive fault injection
are DEFERRED. Existing guards are defense in depth, not a recovery guarantee.
Availability alone does not revoke valid cached credentials; actual missing trust
or expired credentials still fails closed. Real rotation/outage recovery matrices
are deferred, while the existing validity rules and focused security tests remain.

| Event | Bounded PoC contract |
| --- | --- |
| Failed/corrupt caller, broker, issuer or other run component | Operator stops and explicitly recreates the whole owned run; no same-run recovery promise. |
| Money lifetime changes | Memory-only balances/history/receipts can reset; offsets do not restore them. No durable or cross-run exactly-once claim. |
| Explicit fresh start | Remove only owned resources; use new run/trust/stream/context, initialize bounded state once and require explicit new logical work. |

Caller bounds remain 64 descriptors/four variants/16MiB, receipts 4096/180s and
history 10000, without unsafe eviction. No PVC, backup, durable ledger/outbox,
HA or new recovery/controller feature is included. Sync ID derivation and existing
unknown-result semantics stay unchanged. Existing Compose restart contracts and
its JS-22 Sync-survival result remain unchanged. Exhaustive Kubernetes JS-22
recovery/automatic-invalidation re-proving is DEFERRED, not an imported PASS.

## Acceptance, evidence and freeze

All five UI cases and the retained core obligations in [acceptance](acceptance.md)
require final-source proof: exact executable/deployment attestation, independent
SPIFFE authorization/JWS, positive and negative layer evidence, private controls,
role/mount/network boundaries, shared ledger/receipt semantics, pinned startup,
resource caps and basic owned cleanup/public coexistence. Relevant existing
Go/race/vet/JOSE, Python and shipped-JS checks remain required. Full Kubernetes
re-proving of JS-01–JS-23 reliability cases is DEFERRED. Mixed rows explicitly
retain core security checks. Compose evidence is background, never a K8s pass.

Actual TLS evidence requires observed outer peer/authentication/protocol/config
and inner Go or Kafka handshake/verified identity, correlated by flow/connection,
port/time and sanitized outcome. `STRICT` YAML, sidecar presence, encrypted packet
shape, broker ack or a successful button alone does not prove both layers.
Do not dump SDS secrets, TLS session keys, SVIDs/JWS, full ConfigMaps/Secrets,
tokens/cookies or credentials. Restrict diagnostics to versions, hashes, verified
IDs, policy outcomes, connection fields and counters. App-invalid/mesh-success,
mesh rejection before app TLS, authenticated app 403 and JWS rejection have
separate proof. If the client cannot distinguish an opaque mesh error, report
infrastructure/unknown; harness may classify only from correlated layer evidence.

Freeze after final source/fixtures/dependencies are committed, then build/pin
normal app and separately tagged test images. Save exact index/platform/config
image and binary hashes, tool/component versions, normal rendered manifest/
configuration and deployment hashes, node/runtime/resource/topology details,
focused core results, explicit deferred coverage, basic owned cleanup/coexistence and independent
review. Manifests before/after final testing must match, except declared temporary
fixture overlays that are restored and rechecked. Redacted Secret payloads and
generated run IDs/UIDs require an explicit normalization rule; static artifact
hashes are reproducible, while each fresh run has its own recorded runtime hash.
Two runtime reports must use identical source/build/config manifests for their
respective declared profiles; unexplained drift or a missing proof blocks freeze.
Subsequent evidence-only commit must contain no tested source/config change.

Future comparison must separately review removing outer TLS **and** adapting
mesh policies that depend on authenticated mesh principals; `DISABLE` alone would
not preserve their behavior. Retain sidecars/inner TLS/JWS and freeze connection
mode, resources/topology and declared policy differences. Identify possible tools
later, but make no throughput/latency claim or benchmark-only code here.
