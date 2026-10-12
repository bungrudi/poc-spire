# Design

## Context

See [proposal](proposal.md). The source baseline is `98f0fa3`; the current caller
and UI contracts use cross-container UDS, native SPIRE and disposable Compose.
Kubernetes needs verified process attestation and a new private control hop.

## Goals / Non-Goals

Establish a reproducible functional double-mTLS baseline. Keep inner application
identity independent of Pod mesh identity. No durable state, public cutover,
SPIRE-issued Istio credentials, benchmark or comparison variant is included.

## Decisions

1. Propose one owned kind 0.33.0/Kubernetes 1.35.8 node using its pinned image,
   Calico 3.33.0 enforcing policy and Istio 1.31.1 sidecars/default Istiod CA.
   A single node limits shared-host pressure; multi-node/HA adds no required
   durability. Install only after approval and capacity/security preflight.
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
   Pod, so it cannot replace executable or control authorization. Giving the
   unidentified process a control/application SVID would weaken its negative case.
5. Publish authentic public SPIRE bundles from a restricted in-cluster publisher
   into a read-only, whole-directory ConfigMap projection. A profile-configured
   bundle path avoids non-refreshing subPath mounts and preserves Compose
   defaults. Callers receive no keys. Source loss remains infrastructure failure.
6. Use bounded emptyDir/ephemeral cluster state. Container restart may retain
   volumes; Money always resets its memory. Bind startup to expected Pod/run IDs;
   destructive replacement/loss refuses same-run reinitialization and requires
   explicit run-wide fresh start; invalidity stops Sync/Kafka UI/adapter dispatch,
   unlike Compose Sync survival. Preserve IDs/deadlines and no automatic republication.

## Risks / Trade-offs

Unproven containerized attestation/mesh-CNI composition → named real feasibility
and negative gates before migration acceptance. Shared-host pressure → measured
limits/reserve, owned cleanup and live-demo continuity. Two authorities/opaque
TLS → per-layer observed evidence; ambiguous errors stay infrastructure/unknown.
Disposable reset → explicitly demonstrated loss, without recovery guarantees.

## Migration Plan

After owner approval: preflight/pins; owned cluster/CNI/mesh; SPIRE/CSI/attestation;
adapters/bundles/state guards; all five scenarios and parity/security matrix;
commit source before testing, independent review, then freeze verified artifacts.
Rollback deletes only this run; Compose/public remain available. Future outer-TLS
comparison needs its own reviewed policy changes; both variants retain sidecars
and inner TLS. Detailed flows/gates are in [profile](profile.md), [acceptance](acceptance.md)
and [references](references.md). All new runtime checks are NOT RUN.
