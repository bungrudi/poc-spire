# Kubernetes/Istio SPIRE baseline

Public supersession, 13 October 2026: the named domain now serves the
[main-local public Kubernetes profile](kubernetes-public.md). This file retains
the earlier private-baseline checkpoint and its separately scoped operations.

The approved isolated profile runs all demo services, SPIRE/CSI and Kafka inside
one owned kind cluster. Ordinary Istio sidecars use the default Istiod CA for
outer TLS; Go Money/control TLS and original-producer JWS use independent SPIRE
identity. Kafka retains conventional TLS/hostname/DN ACLs. Callers share UID 1000
in separate Pods; each UID 10002 adapter has a control-only SVID and its own local
UDS. Pod mesh identity is shared by caller/adapter and grants no transfer identity.

[Approval and core scope](../openspec/changes/add-kubernetes-double-mtls-baseline/review.md),
[profile/pins](../openspec/changes/add-kubernetes-double-mtls-baseline/profile.md),
[acceptance](../openspec/changes/add-kubernetes-double-mtls-baseline/acceptance.md)
and [current evidence](evidence/kubernetes-core-current.json) identify the exact
contracts, artifacts and modes. Tested source is `528bc5f`; this documentation
follow-up changes no tested runtime source. Parent source/runtime review passed;
final docs consistency review passed; owner acceptance remains separate.

## Operation

Use the isolated implementation checkout, existing pinned tools under `.cache/`
and existing Go/Node/Python prerequisites. Start requires at least 6 GiB available
host RAM and 20 GiB disk; node limits remain 4 GiB / 3 CPU with 2 GiB host reserve. No
resource expansion, shared-workload eviction, global installation or public cutover.

```sh
python3 -B scripts/kubernetes_demo.py preflight
python3 -B scripts/kubernetes_baseline.py start
```

The helper records the owned descriptor in ignored
`.runtime/kubernetes-baseline/active.json`, its kubeconfig/context, run/namespace
and exact Pod/owner/container bindings. Use only that kubeconfig and
`kind-poc-k8s-<run>` context for inspection. API binds literal127.0.0.1:16443;
forward `service/ui`8443 to literal127.0.0.1:18089 and use HTTPS with the owned
browser CA. The human is explicitly assumed, not authenticated. The existing
public demo is unchanged. There is no application/Kafka port publication.

```sh
python3 -B scripts/kubernetes_baseline.py stop
```

On a failed/corrupt run, stop the whole owned cluster and start a fresh one with
new trust/run/stream. This resets fake ledger/history; retain unknown outcomes
honestly and never automatically republish old attempts. Cleanup removes owned
app CSI mounts before node/driver teardown and verifies public/retained/default
context continuity. Do not remove or reuse another checkout's runtime.

## Verified coverage and limits

Five scenarios, exact executable/deployment and route permissions, adapter/UDS
boundaries, broker admission versus JWS receipts and seven correlated actual
inner/outer TLS flows pass. The final five-scenario wrapper observed ledger 2 → 3
because Sync replayed an already committed key; trusted JWS then reached4.
Two authorized preliminary Sync dispatches preceded wrapper assertion corrections.
Generic network errors are classified through Envoy connection/RBAC evidence;
outer OpenSSL observers verify public trust and are not SPIRE-attested processes.

Normal/test images have distinct hashes; normal contains 10 components and no
fixture controls. The separately declared stripped security probe has no SVID
grant. Temporary entries/policies/files/JWS are removed and normal hashes restored.
The current normal run remains ready for independent inspection. Zero restarts/OOM
coexist with memory-limit reclaim; no absence-of-pressure claim is made.

Automatic integrity/lineage, exhaustive crash/restart/outage/ack/rebalance/fault
injection, full Kubernetes JS reliability re-proving and performance comparison
are DEFERRED. Existing Compose contracts/results remain unchanged. No durability,
HA, continuous integrity or host/authorized-process-compromise protection is claimed.
