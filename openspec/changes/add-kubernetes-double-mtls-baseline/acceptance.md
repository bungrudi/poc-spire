# Proposed migration acceptance

**Every KB case and every new Kubernetes execution of JS-01–JS-23 below is
NOT RUN.** Draft validation, upstream support tables and the independently
reviewed Compose baseline are not migration/runtime proof. Parent draft review
is separate from owner implementation approval and final runtime acceptance.

Record future source/artifact/configuration/manifest hashes, exact versions and
actual topology/resources, run/Pod ownership, observed selectors/layers, scoped
fixture overlays, classification, ledger/version/offset/receipt observations and
owned cleanup. Never export private keys, SVID/JWS chains, SDS data, tokens,
cookies, Secret payloads, join tokens or persistent credentials. Positive and
negative controls are required; unreachable infrastructure is not denial proof.

## New Kubernetes obligations

| Case | Contract | Required proof / pass boundary | Proposed mode | Status |
| --- | --- | --- | --- | --- |
| KB-01 Owned compatible preflight | KR-01/03/05 | Exact proposed tools/node image/runtime/cgroups/CIDRs/ports/context and budget/reserve; all components owned inside cluster, no host fallback. Refuse inadequate capacity or collision before touching protected resources. | Read-only plus actual owned bootstrap | NOT RUN |
| KB-02 Secure kubelet and node bootstrap | WA-01; KR-02/03 | PSAT audience/Agent SA/node/parent binding, verified secure kubelet CA/hostname/token and actual PID/proc visibility. Validate only own serving CSRs; wrong CA/hostname/unauthorized token denied. No anonymous/insecure/skip-verification fallback. | Real Kubernetes/SPIRE | NOT RUN |
| KB-03 Mesh/control-plane startup | KR-02; MS-03 | Calico then chained Istio CNI/sidecar injection/default Istiod CA; explicit uninjected SPIRE/system/bootstrap; actual SDS/proxy readiness and native-sidecar Job completion/shutdown. No issuer-before-issuer chicken-and-egg. | Real cold start and bounded failure | NOT RUN |
| KB-04 Enforcing isolation | KR-03; MS-02; PC-01 | Actual UI→Kafka and forbidden cross-control reachability denied; DNS/xDS/allowed flows positive. Inspect masks/credentials/local sockets and no app API/admin/host mount. Policy YAML without an enforcing CNI fails. | Real negative network/mount/identity | NOT RUN |
| KB-05 CSI and source placement | KR-01/02; WA-03 | Owned in-cluster Server/Agent/CSI, correct `/workload/api.sock`, read-only inline app mounts, node-local backing; actual caller/adapter PID requests reach API. Missing socket/CSI/issuer classified as infrastructure, not attestation rejection. | Real CSI and process probes | NOT RUN |
| KB-06 Narrow actual selectors | WA-01/02 | UID1000 app and UID10002 adapter SHA-256 with real K8s container/namespace/SA/immutable image/run/owner/Pod selectors and exact parent. Distinguish OCI digest types; tag-only/Pod-wide image/broader/duplicate grant fails. Rebuilt binary denied until exact reviewed update. | Real attestor and registration guards | NOT RUN |
| KB-07 Wrong process/deployment | WA-01/03 | Wrong executable in actual trusted container, approved executable in wrong deployment with same UID/image and required socket/Money reachability. Explicit PermissionDenied/no identity; timeout/network/mesh failure is not pass. Reachable adapter proof distinguishes its valid control-only identity from app grants and denies an ineligible executable. Normal proxy socket/application-key absence is structural inspection only; no unreachable mount counts as issuance denial. | Real callers/adapters plus structural normal-proxy boundary | NOT RUN |
| KB-08 Authentic public trust | IC-02 | Restricted private-admin publisher, complete CA PEM, native permission-before-replacement and read-only whole-directory projected updates; no subPath, SVID/key or admin credential in unidentified app. Wrong/missing/unusable trust is infrastructure; malformed refresh preserves valid data. | Real publisher/projection plus deterministic parser | NOT RUN |
| KB-09 Positive double TLS | MS-01/04 | All inter-workload Money/control/Kafka flows show correlated actual Istio peer/TLS1.3 and independent inner SDK/broker TLS/current identity. Opaque TCP transport, unchanged exact IDs/DNs, no inner termination or forwarded identity. | Real per-flow telemetry/handshakes | NOT RUN |
| KB-10 Mesh policy and exceptions | MS-02/03 | Wrong SA denied on protected ports, exact allowlist positives. Browser direct HTTPS8443/local UDS/controller/system exceptions enumerated; no extra normal app bypass/permissive route. Same-Pod adapter/caller mesh identity explicitly shared. | Real policy/config and negative flows | NOT RUN |
| KB-11 Fixed control positive/limits | PC-01/02/05/07; UI-05 | UI verifies exact selected adapter with own observer source; adapter verifies exact UI then local peer UID10002/0660 group access. Both original Sync and caller-specific Kafka routes/fields/deadlines/body/response/concurrency bounds preserved. | Real UI→adapter→UDS plus unit bounds | NOT RUN |
| KB-12 Control negatives and expiry | PC-07 | No cert TLS denial; valid wrong inner client 403 before UDS; wrong adapter ID client denial; expired credential before auth/after body denies forwarding; arbitrary destination/payload/role/oversize rejected. No borrowed app key/state/broker mount or cross-caller control. | Real mTLS/peer tests plus injected clock | NOT RUN |
| KB-13 Mesh-valid app-invalid | MS-01/04; MT-01/02/03 | Unidentified outer SA successfully authenticated, API grants no app SVID, exact Money server independently verified, inner no-cert TLS denial with no Money HTTP/history/effect. Separately valid wrong app ID gets403 and fake-root/forged-header cases deny at correct layer. | Real mesh plus application TLS/policy | NOT RUN |
| KB-14 Broker admission versus signer | KC-11; MS-01 | Current conventional broker TLS/hostname, rogue DN/topic/group and forbidden UI access denied separately; admitted wrong-ID/fake-root/tampered/unsigned JWS rejected by Money, not substituted by ACL failure. Admin key remains bootstrap-only. | Real Kafka/mesh/SPIRE negatives | NOT RUN |
| KB-15 Valid app without mesh/provenance | MS-03/04 | Controlled eligible trusted app has separately observed valid app source, then a declared test-only outbound mesh bypass to protected Money port is rejected by STRICT before inner HTTP. Correlated Envoy/application evidence distinguishes mesh denial, app TLS/no HTTP, app 403 and JWS disposition; ambiguous client errors stay infrastructure/unknown. Restore/hash normal overlay. | Real layers plus declared temporary probe policy | NOT RUN |
| KB-16 Rotation/cache/expiry | IC-01/02/03; SJ-05/06 | Real app and control source rotation, private same-ID retry; cached valid operations during non-destructive issuer outage until expiry, no availability-based revocation. Missing bundle/socket or expired credentials fail distinctly. Snapshot/root/time guards use declared deterministic fixtures where appropriate. | Real rotation/outage plus clock/bundle fixtures | NOT RUN |
| KB-17 Five browser scenarios | UI-01/02/07/08; KC-11 | Two Sync HTTP/mTLS and three Async Kafka buttons retain fixed 100 USD-minor transfer, no-SVID rejection, unsigned/tampered receipt outcomes and labels. Direct loopback HTTPS, assumed operator, unchanged API/background polling; no Nginx/public cutover. | Real browser APIs and shipped-JS tests | NOT RUN |
| KB-18 Shared UI defenses | UI-03/04/05/09 | Secure cookies/session limits, CSRF/Host/Origin, fixed schemas, ownership/local-role compatibility, 64 attempts/two workers/eight-per-minute/128 sessions, no unsafe eviction or Kafka/control namespace bypass. Explicitly no human authentication claim. | Real HTTPS negatives plus deterministic quotas/roles | NOT RUN |
| KB-19 Intact restart and ambiguity | PC-04/06/07; UI-04/08/09 | Caller/adapter/UI container restart in same Pod/run preserves initialized metadata and original ID/deadline/instance/all variants. Lost control/broker response recovers reserved digests; latest reject plus earlier unknown remains unknown, any authenticated expected applied result may settle. No new retry deadline/key. | Real restarts plus declared lost-response/ack fixtures | NOT RUN |
| KB-20 Destructive state/Pod loss | KR-04; PC-06; IC-04 | Delete/corrupt initialized state or replace guarded caller/broker/issuer Pod. Same-run guard/init/readiness refuses recreation/republication; after detected invalidity, both Sync/Kafka UI creation/retry and adapter forwarding return 503 run_invalid without new caller invocation. Existing submitted outcomes remain unknown/reset; explicit run-wide fresh start required. Missing data is not proof of non-commit. | Real destructive owned fixtures | NOT RUN |
| KB-21 Repeatable fresh start | KR-04/05 | Whole owned run removal/new namespace/cluster/run/trust/stream/instance boundaries; old Kafka stream/browser run context cannot auto-resume. Recreate stores once without PVC/HA/backup requirement. Manually reused Sync IDs retain documented fresh-lifetime weakness. | Real two fresh runs/cleanup | NOT RUN |
| KB-22 Complete readiness | KR-02; IC-03 | Healthy liveness with missing required operation/assignment/adapter/trust is not readiness; mTLS endpoint is not anonymously probed. Current valid caches during issuer-only outage remain usable. Correct init/registration ordering and bounded diagnostics/dispatch gating without credential/data export. | Real cold start/dependency fault plus native readiness tests | NOT RUN |
| KB-23 Stock broker bootstrap | KR-02; KC-11 | Single KRaft broker/controller, TLS-only advertised `kafka:9093`, topic partition/group/DN ACL bootstrap and consumer assignment. Same-Pod controller exception explicit; injected admin job completes and credentials/Pod cleaned. No automatic bootstrap of lost existing-run data. | Real cold broker/job readiness | NOT RUN |
| KB-24 Ledger/receipts/offsets | FL-01–04; KC-01–09 | Shared cross-ingress duplicates/conflicts/business rejections; 10000 history/context and 4096/180s receipts, guards before effects, no invalid-history access. Lost ack/revocation pauses/heartbeats, exact next offset, actual pre/post-decision SIGKILL with retained original valid record and new memory lifetime. | Real crashes/Kafka plus exact clock/capacity/fake-transport fixtures | NOT RUN |
| KB-25 Actual resources/substrate | KR-03 | Actual node/cgroup limits, Pod requests/limits, JVM/emptyDir/buffer bounds, peak/OOM/CPU/host reserve; app capability/mount/SA-token isolation and declared Agent/CSI/CNI privileges. No silent extra resources/host security change. | Real runtime inspection/resource observations | NOT RUN |
| KB-26 Freeze/provenance/drift | KR-06 | Source/config committed before final tests; exact normal/test images, image index/platform/config and binary/component hashes; matching source/build/declared runtime manifest snapshots, generated-data normalization, restored overlays, fixed topology/resources/connection mode. Drift/missing proof blocks freeze; evidence-only follow-up has no source changes. | Source/registry/config comparison plus actual deployed inspection | NOT RUN |
| KB-27 Owned cleanup/coexistence | KR-05 | Startup failure/name/port/context collisions and shutdown remove only owned resources. Unmount app CSI before driver teardown; incomplete cleanup reports failure and retains retry ownership. Actual public PID/HTTPS/retained-state comparisons and unrelated-context/resource preservation pass. | Real cleanup/collision plus failure injection | NOT RUN |
| KB-28 Full parity/independent review | KR-06/07 | Every KB and JS obligation has current-source K8s real/controlled proof; required Go format/tests/race/vet/JOSE and deterministic browser/Python checks pass. Existing Compose defaults/contracts unchanged. Independent review and explicit owner approval provenance precede known-good freeze; no benchmark/comparison worktree/deployment claim. | Full source/spec/evidence review | NOT RUN |

## Existing JS-01–JS-23 migration mapping

These references identify required parity and the explicit Kubernetes-only
JS-22 invalid-run availability delta; they are **not imported passes**. The
existing Compose state-deletion proof that Sync still works remains unchanged.
Kubernetes instead stops both transports after detected destructive run
invalidation, with no new UI/adapter caller dispatch until fresh-run reset.
Every row is NOT RUN for Kubernetes. Real host clock changes, exported keys and
plaintext/authentication bypasses are not acceptable fixture techniques.

| Existing case | Required Kubernetes proof | New cases | Status |
| --- | --- | --- | --- |
| JS-01 | Trusted original producer, shared balances and correct UI terminal receipt through both layers | KB-09/17/24 | NOT RUN |
| JS-02 | Actual K8s same-UID wrong binary and wrong deployment, with reachable API/Money | KB-05/06/07 | NOT RUN |
| JS-03 | Broker-admitted valid wrong original ID/fake root; own source signing, no borrowed credentials | KB-13/14 | NOT RUN |
| JS-04 | Exact protected/payload bytes and environment/topic/type/audience/key/header binding | KB-14/24/28 | NOT RUN |
| JS-05 | All strict encoding/JSON/algorithm/curve/DER/wire/chain bounds and no fallback/fetch | KB-14/28 | NOT RUN |
| JS-06 | Shared cross-ingress identity/ID/business equality, conflicts/concurrency/stable rejections | KB-17/24 | NOT RUN |
| JS-07 | Matched real SVID rotation with unchanged original private descriptor/context | KB-16/19 | NOT RUN |
| JS-08 | +5/+6 future, exact message/path/deadline, near-expiry and actual held-lock guards | KB-16/24/28 | NOT RUN |
| JS-09 | Real cached outage and matched rotation; declared independent bundle snapshot/root/missing-trust tests | KB-08/16/22 | NOT RUN |
| JS-10 | Ordered terminal acknowledgement, loss/pause/heartbeats/revoke/reassign and valid same-ID redelivery | KB-24/28 | NOT RUN |
| JS-11 | Actual SIGKILL before/after effects, retained original record/offsets and reset-memory weakness | KB-20/24 | NOT RUN |
| JS-12 | Business bounds/conservation and exact 10000 keys/context with no eviction/new binding | KB-24/28 | NOT RUN |
| JS-13 | RFC vector/independent Go↔JOSE on exact final source, negatives and actual versions | KB-26/28 | NOT RUN |
| JS-14 | Broker TLS/DN/topic/group/hostname, no idempotent-write/admin grant, one member/partition | KB-14/23/24 | NOT RUN |
| JS-15 | K8s role/mount/policy/secret isolation, fresh versus reused lifetime limits and scoped cleanup | KB-04/20/21/25/27 | NOT RUN |
| JS-16 | All fixed Sync/Kafka menus and schemas remain; added adapter transport is explicitly scoped | KB-11/17/18 | NOT RUN |
| JS-17 | Observer exact-Money mTLS metadata/receipts, wrong reader403/no-cert TLS, digest/instance/result correlation | KB-09/13/17/24 | NOT RUN |
| JS-18 | Receipt 4096/180s exact reserve/expiry/unknown behavior, persistent within-lifetime history and transient guards | KB-19/24/28 | NOT RUN |
| JS-19 | Reserved variants recover unknown responses, query all variants and never refresh logical ID/deadline | KB-11/19/24 | NOT RUN |
| JS-20 | Shared browser quotas/roles/CSRF/Host/session plus exact network adapter and local peer/mount boundary | KB-04/11/12/18 | NOT RUN |
| JS-21 | Retained rejected bytes remain immutable when time/trust changes; positives still need current verification | KB-16/24/28 | NOT RUN |
| JS-22 | Intact container restart recovers. **K8s-only outcome delta:** destructive Pod/Kafka-state loss invalidates the run; both Sync/Kafka UI/adapter dispatch stop with 503 run_invalid and no new caller invocation until explicit fresh run. Existing Compose Sync-survival-after-Kafka-state-deletion result is preserved separately. | KB-19/20/21 | NOT RUN |
| JS-23 | Complete owned K8s bootstrap/readiness, bounded failure/collision cleanup and protected demo continuity | KB-01/03/22/23/27 | NOT RUN |

## Evidence limits

New actual layer observations, attestation, CNI enforcement, CSI/source behavior,
control transport and reset probes must be real on the selected runtime.
Clock/bundle/size/capacity/lost-ack logic can retain deterministic proof where
explicitly declared above, but actual crash/retained-record, cached outage and
rotation cases remain real. Captured policy/snapshot boundaries are not immediate
revocation. Test-only images/overlays are distinct and absent from normal builds.

Rendered visual/manual review, long-duration CA/root sweeping, scheduled public
ACME renewal, arbitrary kernel packet-loss, benchmarks and the future disabled-
outer-TLS variant are outside this bounded change and NOT RUN. No durable or
production-payment guarantee, protection against host/authorized-process
compromise, or continuous executable integrity is claimed. Mesh is Pod-scoped;
inner executable attestation and authorization remain essential.
