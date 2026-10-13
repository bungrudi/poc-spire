# Bounded Kubernetes PoC acceptance

The owner narrowed scope on 13 October 2026; see [review](review.md). Retained
core obligations are **NOT RUN for final-source acceptance** until proved.
[Development readiness/five-scenario evidence](implementation.md) is provisional.
Reliability subcases marked **DEFERRED** are outside this completion scope, never
PASS. Existing Compose contracts/evidence remain unchanged and are not imported
Kubernetes results. Mixed rows below retain their core security obligations.

Record future source/artifact/configuration/manifest hashes, exact versions and
actual topology/resources, run/Pod ownership, observed selectors/layers, scoped
fixture overlays, classification, ledger/version/offset/receipt observations and
owned cleanup. Never export private keys, SVID/JWS chains, SDS data, tokens,
cookies, Secret payloads, join tokens or persistent credentials. Positive and
negative controls are required; unreachable infrastructure is not denial proof.

## Retained core and deferred Kubernetes coverage

| Case | Contract | Core proof / deferred subcase | Proof mode | Status |
| --- | --- | --- | --- | --- |
| KB-01 Owned compatible preflight | KR-01/03/05 | Exact proposed tools/node image/runtime/cgroups/CIDRs/ports/context and budget/reserve; all components owned inside cluster, no host fallback. Refuse inadequate capacity or collision before touching protected resources. | Read-only plus actual owned bootstrap | PASS (declared runtime/deterministic modes) |
| KB-02 Secure kubelet and node bootstrap | WA-01; KR-02/03 | PSAT audience/Agent SA/node/parent binding, verified secure kubelet CA/hostname/token and actual PID/proc visibility. Validate only own serving CSRs; wrong CA/hostname/unauthorized token denied. No anonymous/insecure/skip-verification fallback. | Real Kubernetes/SPIRE | PASS (declared runtime/deterministic modes) |
| KB-03 Mesh/control-plane startup | KR-02; MS-03 | Calico then chained Istio CNI, regular app sidecars/default Istiod CA, explicit uninjected SPIRE/system paths, actual SDS/proxy readiness and native-sidecar admin Job completion/shutdown. Initial bootstrap is bounded; exhaustive startup-failure injection is DEFERRED. | Real cold start; extended fault injection DEFERRED | CORE PASS (declared modes); reliability DEFERRED |
| KB-04 Enforcing isolation | KR-03; MS-02; PC-01 | Actual UI→Kafka and forbidden cross-control reachability denied; DNS/xDS/allowed flows positive. Inspect masks/credentials/local sockets and no app API/admin/host mount. Policy YAML without an enforcing CNI fails. | Real negative network/mount/identity | PASS (declared runtime/deterministic modes) |
| KB-05 CSI and source placement | KR-01/02; WA-03 | Owned in-cluster Server/Agent/CSI, correct `/workload/api.sock`, read-only inline app mounts, node-local backing; actual caller/adapter PID requests reach API. Missing socket/CSI/issuer classified as infrastructure, not attestation rejection. | Real CSI and process probes | PASS (declared runtime/deterministic modes) |
| KB-06 Narrow actual selectors | WA-01/02 | UID 1000 app and UID 10002 adapter SHA-256 with real K8s container/namespace/SA/immutable image/run/owner/Pod selectors and exact parent. Distinguish OCI digest types; tag-only/Pod-wide image/broader/duplicate grant fails. Rebuilt binary denied until exact reviewed update. | Real attestor and registration guards | PASS (declared runtime/deterministic modes) |
| KB-07 Wrong process/deployment | WA-01/03 | Wrong executable in actual trusted container, approved executable in wrong deployment with same UID/image and required socket/Money reachability. Explicit PermissionDenied/no identity; timeout/network/mesh failure is not pass. Reachable adapter proof distinguishes its valid control-only identity from app grants and denies an ineligible executable. Normal proxy socket/application-key absence is structural inspection only; no unreachable mount counts as issuance denial. | Real callers/adapters plus structural normal-proxy boundary | PASS (declared runtime/deterministic modes) |
| KB-08 Authentic public trust | IC-02 | Restricted private-admin publisher, complete CA PEM, native permission-before-replacement and read-only whole-directory projected updates; no subPath, SVID/key or admin credential in unidentified app. Wrong/missing/unusable trust is infrastructure; malformed refresh preserves valid data. | Real publisher/projection plus deterministic parser | PASS (declared runtime/deterministic modes) |
| KB-09 Positive double TLS | MS-01/04 | All inter-workload Money/control/Kafka flows show correlated actual Istio peer/TLS1.3 and independent inner SDK/broker TLS/current identity. Opaque TCP transport, unchanged exact IDs/DNs, no inner termination or forwarded identity. | Real per-flow telemetry/handshakes | PASS (declared runtime/deterministic modes) |
| KB-10 Mesh policy and exceptions | MS-02/03 | Wrong SA denied on protected ports, exact allowlist positives. Browser direct HTTPS8443/local UDS/controller/system exceptions enumerated; no extra normal app bypass/permissive route. Same-Pod adapter/caller mesh identity explicitly shared. | Real policy/config and negative flows | PASS (declared runtime/deterministic modes) |
| KB-11 Fixed control positive/limits | PC-01/02/05/07; UI-05 | UI verifies exact selected adapter with own observer source; adapter verifies exact UI then local peer UID 10002/0660 group access. Both original Sync and caller-specific Kafka routes/fields/deadlines/body/response/concurrency bounds preserved. | Real UI→adapter→UDS plus unit bounds | PASS (declared runtime/deterministic modes) |
| KB-12 Control negatives and expiry | PC-07 | No cert TLS denial; valid wrong inner client 403 before UDS; wrong adapter ID client denial; expired credential before auth/after body denies forwarding; arbitrary destination/payload/role/oversize rejected. No borrowed app key/state/broker mount or cross-caller control. | Real mTLS/peer tests plus injected clock | PASS (declared runtime/deterministic modes) |
| KB-13 Mesh-valid app-invalid | MS-01/04; MT-01/02/03 | Unidentified outer SA successfully authenticated, API grants no app SVID, exact Money server independently verified, inner no-cert TLS denial with no Money HTTP/history/effect. Separately valid wrong app ID gets403 and fake-root/forged-header cases deny at correct layer. | Real mesh plus application TLS/policy | PASS (declared runtime/deterministic modes) |
| KB-14 Broker admission versus signer | KC-11; MS-01 | Current conventional broker TLS/hostname, rogue DN/topic/group and forbidden UI access denied separately; admitted wrong-ID/fake-root/tampered/unsigned JWS rejected by Money, not substituted by ACL failure. Admin key remains bootstrap-only. | Real Kafka/mesh/SPIRE negatives | PASS (declared runtime/deterministic modes) |
| KB-15 Valid app without mesh/provenance | MS-03/04 | Controlled eligible trusted app has separately observed valid app source, then a declared test-only outbound mesh bypass to protected Money port is rejected by STRICT before inner HTTP. Correlated Envoy/application evidence distinguishes mesh denial, app TLS/no HTTP, app 403 and JWS disposition; ambiguous client errors stay infrastructure/unknown. Restore/hash normal overlay. | Real layers plus declared temporary probe policy | PASS (declared runtime/deterministic modes) |
| KB-16 Rotation/cache/expiry | IC-01/02/03; SJ-05/06 | Current-source/trust/expiry and signing snapshot/time guards remain security requirements; availability alone is not revocation. Use focused declared clock/bundle tests. Real rotation/private-retry and prolonged cached-outage/recovery matrices are DEFERRED. | Focused clock/bundle/source tests; runtime lifecycle matrix DEFERRED | CORE PASS (declared modes); reliability DEFERRED |
| KB-17 Five browser scenarios | UI-01/02/07/08; KC-11 | Two Sync HTTP/mTLS and three Async Kafka buttons retain fixed 100 USD-minor transfer, no-SVID rejection, unsigned/tampered receipt outcomes and labels. Direct loopback HTTPS, assumed operator, unchanged API/background polling; no Nginx/public cutover. | Real browser APIs and shipped-JS tests | PASS (declared runtime/deterministic modes) |
| KB-18 Shared UI defenses | UI-03/04/05/09 | Secure cookies/session limits, CSRF/Host/Origin, fixed schemas, ownership/local-role compatibility, 64 attempts/two workers/eight-per-minute/128 sessions, no unsafe eviction or Kafka/control namespace bypass. Explicitly no human authentication claim. | Real HTTPS negatives plus deterministic quotas/roles | PASS (declared runtime/deterministic modes) |
| KB-19 Intact restart and ambiguity | PC-04/06/07; UI-04/08/09 | Preserve original IDs/deadlines, bounded correlated responses and honest unknown outcomes with focused existing control/receipt tests. Real caller/adapter/UI restart and all lost-control/broker-response variant recovery permutations are DEFERRED. | Focused existing deterministic/control tests; recovery matrix DEFERRED | CORE PASS (declared modes); recovery DEFERRED |
| KB-20 Destructive state/Pod loss | KR-04; PC-06; IC-04 | Automatic state/Pod-loss detection, issuer integrity/lineage and exhaustive same-Pod/crash-loop propagation tests are DEFERRED. Failed/corrupt runs use operator whole-cluster stop/recreate; basic reset/cleanup remains required in KB-21/KB-27. | Extended destructive-state fixtures outside core scope | DEFERRED |
| KB-21 Repeatable fresh start | KR-04/05 | Basic owned stop/recreate and repeatable pinned setup use fresh run/trust/stream/browser context and initialize bounded stores once. No PVC/HA/backup or cross-run exactly-once promise; manually reused Sync IDs retain the memory-lifetime limitation. Exhaustive interrupted-reset permutations are DEFERRED. | Basic real fresh setup/cleanup plus shipped-JS context tests | CORE PASS (declared modes); extended reset DEFERRED |
| KB-22 Complete readiness | KR-02; IC-03 | Prove complete initial dependency readiness, current identity/trust, private controls, broker assignment and authenticated UI reads. Mandatory mTLS is not anonymously probed. Focused native tests retain validity/infrastructure classifications. Exhaustive dependency/outage/restart fault matrices are DEFERRED. | Real cold start/native readiness plus focused tests | CORE PASS (declared modes); reliability DEFERRED |
| KB-23 Stock broker bootstrap | KR-02; KC-11 | Single KRaft broker/controller, TLS-only advertised `kafka:9093`, topic partition/group/DN ACL bootstrap and consumer assignment. Same-Pod controller exception explicit; injected admin job completes and credentials/Pod cleaned. No automatic bootstrap of lost existing-run data. | Real cold broker/job readiness | PASS (declared runtime/deterministic modes) |
| KB-24 Ledger/receipts/offsets | FL-01–04; KC-01–09 | Preserve shared integer ledger/idempotency/conflicts, authenticated exact-record receipts, guards before effects and no invalid-history access; retain focused relevant deterministic bounds/unknown tests. Actual pre/post-decision SIGKILL, lost-ack/revoke/reassign/redelivery and exhaustive capacity/fault re-proving are DEFERRED. | Focused real ingress/JWS/receipt checks and existing deterministic tests | CORE PASS (declared modes); recovery DEFERRED |
| KB-25 Actual resources/substrate | KR-03 | Observe actual node/cgroup caps, Pod requests/limits, bounded JVM/emptyDir/buffers, peak/OOM/CPU/host reserve and public coexistence; inspect app mount/capability/SA isolation and declared substrate privileges. No silent resource/host-security changes. Exhaustive stress/pressure failure injection is DEFERRED. | Real runtime/resource inspection; stress matrix DEFERRED | CORE PASS (declared modes); stress DEFERRED |
| KB-26 Freeze/provenance/drift | KR-06 | Source/config committed before final tests; exact normal/test images, image index/platform/config and binary/component hashes; matching source/build/declared runtime manifest snapshots, generated-data normalization, restored overlays, fixed topology/resources/connection mode. Drift/missing proof blocks freeze; evidence-only follow-up has no source changes. | Source/registry/config comparison plus actual deployed inspection | PASS; independent source/runtime/docs review |
| KB-27 Owned cleanup/coexistence | KR-05 | Basic owned startup/stop cleanup and preflight refusal preserve foreign resources/default context/public HTTPS/retained identity. Unmount app CSI before node/driver removal and report incomplete cleanup honestly. Exhaustive build/export/import/partial-failure injection is DEFERRED. | Basic real owned cleanup/coexistence and existing ownership tests | CORE PASS (declared modes); fault injection DEFERRED |
| KB-28 Core acceptance/independent review | KR-06/07 | Every retained core obligation has final-source proof with explicit modes; relevant Go format/tests/race/vet/JOSE and Python/browser checks pass. Existing Compose defaults/contracts remain. Independent review and approval provenance precede freeze; deferred reliability is never PASS or a prerequisite. No benchmark/comparison/public cutover. | Bounded core source/spec/evidence review | PASS; independent source/runtime/docs review |

## Existing JS-01–JS-23 traceability

This is a scoped compatibility map, not mandatory full Kubernetes reliability
parity. Existing Compose results/contracts, including JS-22 Sync survival, remain
unchanged. Core security portions remain required even in mixed rows. Full new
Kubernetes reliability re-proving is DEFERRED; no historical result is a new PASS.
Do not change the host clock or export keys, chains, JWS, tokens or cookies.

| Existing case | Retained core / deferred Kubernetes proof | New cases | Status |
| --- | --- | --- | --- |
| JS-01 | Trusted original producer, shared balances and correct UI terminal receipt through both layers | KB-09/17/24 | PASS (declared runtime/deterministic modes) |
| JS-02 | Actual K8s same-UID wrong binary and wrong deployment, with reachable API/Money | KB-05/06/07 | PASS (declared runtime/deterministic modes) |
| JS-03 | Broker-admitted valid wrong original ID/fake root; own source signing, no borrowed credentials | KB-13/14 | PASS (declared runtime/deterministic modes) |
| JS-04 | Exact protected/payload bytes and environment/topic/type/audience/key/header binding | KB-14/24/28 | PASS (declared runtime/deterministic modes) |
| JS-05 | All strict encoding/JSON/algorithm/curve/DER/wire/chain bounds and no fallback/fetch | KB-14/28 | PASS (declared runtime/deterministic modes) |
| JS-06 | Shared cross-ingress identity/ID/business equality, conflicts/concurrency/stable rejections | KB-17/24 | PASS (declared runtime/deterministic modes) |
| JS-07 | Real rotation/private-descriptor restart recovery is deferred; current credential/signature validity remains core in KB-16. | KB-16/19 | DEFERRED; validity core retained |
| JS-08 | Existing focused future/message/path/deadline/expiry/snapshot tests remain core; exhaustive runtime timing/held-lock fault re-proving is deferred. | KB-16/24/28 | CORE PASS (deterministic/runtime as declared); timing matrix DEFERRED |
| JS-09 | Independent trust snapshot/root/missing-bundle security tests remain core; real rotation and cached-outage/recovery matrix is deferred. | KB-08/16/22 | CORE PASS (deterministic/runtime as declared); lifecycle DEFERRED |
| JS-10 | Ordered acknowledgement semantics stay unchanged; Kubernetes loss/pause/heartbeat/revoke/reassign/redelivery re-proving is deferred. | KB-24/28 | DEFERRED; existing contracts unchanged |
| JS-11 | Real pre/post-effect SIGKILL and retained-record/offset recovery re-proving is deferred; memory-only ledger limits remain documented. | KB-20/24 | DEFERRED |
| JS-12 | Relevant existing business/conservation/history/no-eviction bounds tests remain core; exhaustive Kubernetes capacity/fault re-proving is deferred. | KB-24/28 | CORE PASS (deterministic/runtime as declared); stress DEFERRED |
| JS-13 | RFC vector/independent Go↔JOSE on exact final source, negatives and actual versions | KB-26/28 | PASS (declared runtime/deterministic modes) |
| JS-14 | Broker TLS/DN/topic/group/hostname, no idempotent-write/admin grant, one member/partition | KB-14/23/24 | PASS (declared runtime/deterministic modes) |
| JS-15 | Role/mount/policy/credential isolation, resource caps and basic owned cleanup remain core; exhaustive destructive-loss/recovery variants are deferred. | KB-04/20/21/25/27 | CORE PASS (deterministic/runtime as declared); recovery DEFERRED |
| JS-16 | All fixed Sync/Kafka menus and schemas remain; added adapter transport is explicitly scoped | KB-11/17/18 | PASS (declared runtime/deterministic modes) |
| JS-17 | Observer exact-Money mTLS metadata/receipts, wrong reader403/no-cert TLS, digest/instance/result correlation | KB-09/13/17/24 | PASS (declared runtime/deterministic modes) |
| JS-18 | Authenticated exact-record/instance correlation and honest unknown/expiry semantics remain core with focused existing receipt tests; exhaustive capacity/transient recovery re-proving is deferred. | KB-19/24/28 | CORE PASS (deterministic/runtime as declared); reliability DEFERRED |
| JS-19 | Original ID/deadline, reserved digest/variant and unknown-response semantics remain core through focused existing tests; every lost-response/restart variant permutation is deferred. | KB-11/19/24 | CORE PASS (deterministic/runtime as declared); recovery DEFERRED |
| JS-20 | Shared browser quotas/roles/CSRF/Host/session plus exact network adapter and local peer/mount boundary | KB-04/11/12/18 | PASS (declared runtime/deterministic modes) |
| JS-21 | Current verification and immutable rejection semantics remain core through relevant existing guard tests; exhaustive runtime trust/time transition re-proving is deferred. | KB-16/24/28 | CORE PASS (deterministic/runtime as declared); lifecycle DEFERRED |
| JS-22 | Kubernetes failed/corrupt run policy is operator whole-cluster stop/recreate. Automatic invalidation and intact/crash-loop recovery re-proving are deferred. Existing Compose Sync-survival-after-state-deletion contracts/results remain unchanged. | KB-19/20/21 | DEFERRED; basic reset core retained |
| JS-23 | Complete owned bootstrap/readiness, basic pinned cleanup/preflight refusal and public coexistence remain core; exhaustive failure/collision injection is deferred. | KB-01/03/22/23/27 | CORE PASS (deterministic/runtime as declared); fault injection DEFERRED |

## Evidence limits

Actual retained attestation, CNI/mesh authorization, control transport and layer
checks must use the selected runtime. Clock/bundle/encoding/bounds/unknown-result
logic may use explicitly declared existing deterministic tests. Those modes are
not full runtime recovery proof. Test images/overlays are distinct from normal,
restored and hashed; valid cached credentials are not immediately revoked by
issuer availability. Source/config is committed before final tests and independent
review; missing core proof or unexplained drift blocks freeze.

Automatic integrity monitoring, issuer lineage, exhaustive crash/restart/outage/
cleanup injection and full JS reliability re-proving are DEFERRED. Rendered/manual
visual review, long-duration root sweeping, public ACME renewal, arbitrary kernel
packet-loss, durability/HA, benchmarks and the outer-disabled comparison are also
outside this completion scope. No host/authorized-process-compromise protection or
continuous executable integrity is claimed. Mesh is Pod-scoped; separate inner
attestation/authorization remains essential. Failed/corrupt run means operator
whole-cluster stop/recreate, with no recovery guarantee or invented PASS.


## Final retained core evidence — 13 October 2026

[Current report](../../../docs/evidence/kubernetes-core-current.json) records
the executed retained core checks and updated statuses on clean `528bc5f`.
KB-01–18,21–25,27 retained core checks PASS with the real/deterministic modes below;
KB-19/24 ambiguity/expiry/bounds use focused existing deterministic tests, not a
new runtime recovery matrix. KB-20 and every deferred subcase stay DEFERRED.
KB-26/28 source/artifact/runtime and parent technical review pass; final docs
consistency review passes after strict 12/12; evidence-only publication follows. No owner acceptance is inferred.

Real run: cold bootstrap/readiness; five scenarios; exact executable/deployment;
actual normal caller/adapter wrong-binary and UID-checked UDS denials; Money/control
TLS/exact-ID negatives; broker DN/topic/group/hostname and original-signer/JWS
receipts; normal wrong-role control RBAC/network and UI→Kafka isolation; all seven
correlated inner/outer TLS paths; bounded old-run cleanup/fresh setup and restored
normal grants/configuration/protected coexistence. The public endpoint is unchanged.
Deterministic mode: existing format/encoding/time/trust/source/expiry/unknown/bounds,
shared-ledger/conflict/concurrency, browser quotas/roles/retry and guard tests,
uncached Go race/tagged/vet/JOSE, 42 Python checks and shipped JavaScript. Existing
Compose results are preserved; full Kubernetes JS reliability is not rerun.

The final browser wrapper measured ledger 2 → 3 because Sync replayed a prior key;
valid JWS reached4. Generic transport errors require saved Envoy/RBAC/current-source
correlation. OpenSSL observers verify authentic roots/exact peer and are not claimed
SPIRE-attested. Initial fixture/proxy-allowlist/assertion errors are disclosed;
corrected focused evidence replaces them, without another full passing-matrix run.
