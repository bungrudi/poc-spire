# Design

## Context

See [proposal](proposal.md). Existing Compose uses cross-container UDS/native
SPIRE. Kubernetes adds verified executable/deployment attestation and a private
authenticated control hop.

## Goals / Non-Goals

Establish a reproducible functional double-mTLS baseline. Keep inner application
identity independent of Pod mesh identity. No durable state, public cutover,
SPIRE-issued Istio credentials, benchmark or comparison variant is included.

## Decisions

1. Use one owned pinned kind 0.33.0/Kubernetes 1.35.8 node, enforcing Calico
   3.33.0 and Istio 1.31.1 sidecars/default Istiod CA, after capacity/security
   preflight. Keep the reviewed shared-host caps/reserve.
2. Run SPIRE 1.14.0 Server plus node Agent/SPIFFE CSI 0.2.13 in Kubernetes.
   Built-in Unix digest and Kubernetes selectors must jointly bind UID,
   executable, requesting container image, namespace/SA/role and observed owner.
   Secure kubelet serving certificates need scoped CSR approval. Actual PID/proc,
   cgroup-v2 and selector denials are an early feasibility gate, never a downgrade.
3. Pass inner TLS as opaque TCP through outer `ISTIO_MUTUAL`/`STRICT` TLS.
   Go Money keeps exact SPIFFE authorization; stock Kafka keeps its own PKI/ACLs
   and producer JWS. NetworkPolicies and mesh principals restrict documented
   flows. Browser HTTPS, local UDS and infrastructure bootstrap have explicit
   exceptions. CNI avoids networking capabilities in application containers.
4. Use one UID-10002 control adapter in each UID-1000 caller Pod. UI verifies
   its exact control-only SVID; adapter admits only UI and mirrors fixed schemas
   over its own local UID-checked UDS. GID 1000 permits the 0660 socket; private
   state/keys are not mounted into adapters. Mesh identity is shared within that
   Pod, so it cannot replace executable or control authorization. The unidentified caller gets no app SVID.
5. Publish authentic public SPIRE bundles from a restricted in-cluster publisher
   into a read-only, whole-directory ConfigMap projection. A profile-configured
   bundle path avoids non-refreshing subPath mounts and preserves Compose
   defaults. Callers receive no keys. Source loss remains infrastructure failure.
6. Use bounded emptyDir/ephemeral state and preserve existing useful startup/run
   guards. Money is memory-only. If a run fails or is corrupt, the operator stops
   the whole owned cluster and explicitly recreates it with fresh trust/stream.
   No durability, automatic integrity monitoring or exhaustive restart/crash
   recovery is required. Keep original IDs/unknown-result semantics; do not claim
   continuity or automatically republish old attempts. Compose contracts remain.
7. The owner narrowed acceptance on 13 October; see [review](review.md).
   Finish the five scenarios, exact attestation/authorization/JWS negatives,
   correlated inner/outer TLS, pinned setup/basic cleanup/coexistence and relevant
   tests. Reliability subcases are explicitly deferred in [acceptance](acceptance.md).

## Risks / Trade-offs

Unproven containerized attestation/mesh-CNI composition → named real feasibility
and negative gates before migration acceptance. Shared-host pressure → measured
limits/reserve, owned cleanup and live-demo continuity. Two authorities/opaque
TLS → per-layer observed evidence; ambiguous errors stay infrastructure/unknown.
Disposable reset → operator whole-cluster stop/recreate, without recovery guarantees.

## Migration Plan

Approved sequence: pinned preflight → owned cluster/CNI/mesh → SPIRE/CSI/
attestation → controls/public bundles → five scenarios and focused security/layers.
Commit source/config before final tests; independently review, then freeze.
Deferred reliability is not a prerequisite. Reset deletes only this run; Compose/
public remain untouched. Future outer-TLS comparison needs separate review.
[Profile](profile.md), [acceptance](acceptance.md) and [references](references.md)
define details. Development proofs are provisional; final core acceptance is pending.
