# Proposal

## Why

The independently reviewed Compose baseline at `98f0fa3` proves the five fixed
SPIRE/Kafka scenarios, but does not measure Kubernetes attestation or a mesh.
The owner requests a complete in-cluster migration, a validated application-plus-
Istio mTLS baseline, then a reproducible freeze for a later controlled comparison.

Rudi approved implementation of the published revision on 13 October 2026;
see [approval and frozen artifact manifest](review.md). This change covers
steps 1–3 only, including its owned disposable installation and isolated tests.
Benchmarking, comparison worktrees and public cutover remain excluded.

## What Changes

- Add one disposable single-node kind cluster on existing Linux Docker, with an
  actually enforcing Calico CNI. Run SPIRE Server, node Agent/CSI, UI, both caller
  Pods, Money, stock Kafka and scoped bootstrap tooling inside Kubernetes.
- Inject Istio sidecars into application/Kafka/control traffic Pods. Use default
  Istiod CA for outer mesh identities and independent SPIRE application SVIDs.
  Inner Money TLS still terminates in Go; Kafka TLS/DN ACLs and original-producer
  JWS remain separate. Mesh/control-plane bootstrap exceptions are explicit.
- Gate migration on real built-in Unix SHA-256 **and** Kubernetes attestation,
  secure verified kubelet access, actual PID/proc visibility and immutable image
  selectors. Preserve UID-1000 callers and same-deployment wrong-executable /
  wrong-deployment approved-executable denials. No downgrade or custom plugin.
- **BREAKING, opt-in Kubernetes control transport only:** replace cross-Pod UDS
  mounts with UI-authenticated private network adapters. Each caller retains its
  own local UDS; its separately attested UID-10002 adapter has only a control
  SVID, verifies exact UI ID and cannot use/lend application credentials.
- Supply the unidentified process with a refreshed authentic **public bundle**,
  never an application SVID/key. Missing/stale trust is infrastructure failure.
- Keep all five fixed UI scenarios, APIs, integer ledger, quotas, original retry
  IDs/deadlines, receipts and loss/unknown classifications. Use disposable state,
  no PVC/HA/backup/durable-recovery feature. Destructive Pod/state loss requires
  an explicit fresh run; never silently recreate or republish an old attempt.
  Kubernetes invalid-run handling stops both Sync and Kafka UI/adapter dispatch;
  the existing Compose Sync-survival-after-Kafka-state-loss result remains intact.
- Require complete migration/parity/security acceptance, actual evidence for
  each TLS layer, resource/isolation/cleanup checks and independent review.
  Commit final source before testing; freeze exact image/component/configuration,
  manifest and topology hashes with sanitized evidence and known limitations.

## Capabilities

### New Capabilities

- `kubernetes-demo-runtime`: owned disposable cluster, full topology/bootstrap,
  state/reset/isolation, readiness and a reproducible validated baseline freeze.
- `mesh-transport-security`: sidecar outer TLS/policy, independent inner identity,
  explicit flow exceptions and measured failure/layer provenance.

### Modified Capabilities

- `workload-attestation`: add gated Unix/Kubernetes executable/deployment binding
  beside the existing measured native/Docker profile (WA-01/WA-03).
- `private-caller-controls`: profile-scoped exact-ID adapters with preserved local
  peer/schema boundaries and disposable state behavior (PC-01/PC-05/PC-06).
- `identity-certificate-lifecycle`: in-cluster authentic public bundle projection
  and explicit disposable identity/reset boundaries (IC-02/IC-04).
- `https-demo-ui`: allow exact-ID adapter transport while retaining read-only UI,
  fixed scenarios, quotas and authenticated unknown-result handling; scope the
  existing Nginx clause to its edge profile (UI-02/UI-05).
- `kafka-signed-command-consumption`: add the one-broker/one-Money Kubernetes
  baseline without changing the existing Compose contract (new KC-11).

## Impact

Future implementation touches deployment/runner assets, a small control adapter
and UI transport selection, caller trust-path configuration and scoped readiness/
run guards. Go SPIFFE application verification, codec/ledger behavior and current
connection mode remain. Proposed pins and feasibility gates are in [references](references.md);
flow/reset details and granular unrun proofs follow in [profile](profile.md) and
[acceptance](acceptance.md). The short approach is in [design](design.md).

Current [contracts](../../specs/), [RFC rationale](../../../docs/rfcs/0001-process-identity.md),
[JS evidence](../../../docs/kafka-signing-acceptance.md) and actual planning instruction
in [review](review.md) govern compatibility. Existing Compose/public endpoint and
retained identity remain untouched. SPIRE-for-Istio integration, OIDC, persistence,
replicas, benchmarking and the future outer-mTLS-disabled variant are out of scope.
