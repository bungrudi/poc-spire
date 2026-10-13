# Tasks

[Review](review.md) records original approval and the owner-approved 13 October
narrowing. The 23 task IDs retain only the core obligations below; no checkbox
was closed by rescoping. Retained core acceptance and parent source/runtime review pass on clean `528bc5f`;
final docs/evidence consistency review passed after the bounded corrections. [Evidence](../../../docs/evidence/kubernetes-core-current.json)
declares actual modes and limits. DEFERRED subcases
in [acceptance](acceptance.md) are outside this batch, never PASS. Update current
contracts/RFC/runbook/evidence as retained tasks actually complete.

## 1. Approval and reproducible preflight

- [x] 1.1 Obtain actual owner implementation approval of the original profile/delta/short plan and approved narrowed core scope; record message/date and frozen artifact manifest, separate from parent drafting review/publication permission.
- [x] 1.2 Resolve/check exact proposed tools, images/charts/manifests and registry index/platform/config digests; assess current compatibility/security, keeping existing app dependency/connection pins; document KB-01/KB-26 assumptions and failure gates.
- [x] 1.3 Verify capacity/reserve, unused ports/CIDRs/names, owned kubeconfig/context and declared kind/CNI/CSI/Agent privilege boundary; pass KB-01/KB-25 or stop without touching shared services.

## 2. Owned cluster and independent mesh substrate

- [x] 2.1 Add scoped disposable kind bootstrap/ownership/cleanup with the pinned one-node image and explicit resource limits; install enforcing Calico, prove KB-01/KB-04/KB-27 and retain Compose/public continuity.
- [x] 2.2 Configure signed kubelet serving certificates with narrowly validated node CSRs, verified CA/hostname/token and minimal API/kubelet RBAC; pass KB-02 secure-access negative/positive proof, without anonymous/skip-verification fallback.
- [x] 2.3 Install pinned Istio sidecar/CNI/default Istiod CA and explicit system injection exclusions; verify injection/SDS/current credentials, opaque TCP STRICT/ISTIO_MUTUAL and documented exact flows/exceptions in KB-03/KB-09/KB-10, including bootstrap Job completion.

## 3. In-cluster SPIRE and feasibility gate

- [x] 3.1 Deploy owned ephemeral SPIRE Server and node Agent/CSI with PSAT, scoped private admin/bootstrap RBAC and node-local socket backing; pass KB-02/KB-05 reachability/bootstrap/permission checks with no native host SPIRE.
- [x] 3.2 Establish expected run/Pod UID barriers without an ImageID/SVID-before-container-start deadlock; derive observed immutable requesting-container/image/owner selectors and exact parents, refuse broader/duplicate grants; prove KB-05/KB-06 and explicit binary-update denial.
- [x] 3.3 Prove Unix executable SHA-256 plus K8s conjunction for UID 1000 apps and UID 10002 adapters under the actual container runtime; execute reachable wrong-binary/wrong-deployment and adapter app-grant/ineligible-executable tests in KB-06/KB-07; inspect normal proxy Workload API/application-key absence structurally, without claiming unreachable issuance denial. Stop dependent migration if the gate fails.
- [x] 3.4 Implement restricted authentic public-bundle publication/projection and configured caller trust path with existing defaults intact; verify authentic publication/projection and focused missing/wrong trust/expiry guards with declared deterministic modes in KB-08/KB-16 and IC-02/IC-03.

## 4. Private network controls and disposable application state

- [x] 4.1 Add separately attested UID 10002 control adapters with exact UI authorization, exact adapter server-ID checks, local UID-checked sockets, schemas/deadlines/limits and expiry/unknown-response guards; prove KB-11/KB-12 plus unchanged PC/Go control tests.
- [x] 4.2 Add Kubernetes-only UI control transport selection without caller state/socket/key mounts or API schema/ID/quota changes; verify fixed scenarios, malformed input/roles/CSRF/Host/Origin and cross-caller denial in KB-11/KB-17/KB-18.
- [x] 4.3 Preserve bounded caller-only initialization, existing useful run guards, original IDs/deadlines/unknown outcomes and browser run context; document operator whole-cluster stop/recreate on failed/corrupt runs, verify basic fresh setup in KB-21 and original 64/four bounds. Automatic monitoring extensions and exhaustive KB-19/KB-20 recovery are DEFERRED.

## 5. Full broker/application deployment and readiness

- [x] 5.1 Add in-cluster stock one-node Kafka, bounded disposable data, separate ephemeral broker/browser PKI, TLS hostname/DN ACL/topic bootstrap and bounded injected Job lifecycle; prove KB-09/KB-14/KB-23 and no UI/adapter broker/admin credential.
- [x] 5.2 Deploy one Money/UI/two caller Pods and adapters with hardened app contexts, private state/credential separation and no app Kubernetes/admin mounts; preserve embedded consumer/shared ledger and unchanged inner HTTP/JWS guards; prove KB-04/KB-13/KB-17/KB-25.
- [x] 5.3 Verify complete native initial dependency readiness and dispatch gating, including current-source/trust and authenticated UI reads in KB-03/KB-05/KB-22; retain cached-valid versus expired-material classification with focused tests. Exhaustive runtime outage/restart/readiness-fault injection is DEFERRED.

## 6. Migration acceptance and honest layer evidence

- [x] 6.1 Commit final source, fixtures, dependencies and normal/test configuration before runtime evidence; build/pin normal and separately identified test images, verify absence of test/benchmark controls in normal builds, and record KB-26 manifests.
- [x] 6.2 Run observed positive and negative layer proofs: mesh-valid/app-invalid, valid-app/no-mesh, wrong valid app ID 403, fake root/headers, broker admission versus JWS; correlate sanitized actual TLS observations for KB-09–KB-15 without exporting keys/SVID/JWS/SDS/cookies.
- [x] 6.3 Run all five UI scenarios and retained focused attestation/authorization/JWS/shared-ledger checks in acceptance.md serially; verify basic repeatable setup/owned cleanup/coexistence and caps. Preserve existing Compose contracts, identify deterministic modes and explicitly DEFER full Kubernetes JS reliability re-proving, crash/ack/recovery and exhaustive fault injection.
- [x] 6.4 Run formatting, Go tests/race/vet, relevant deterministic Python/browser/independent JOSE tests and structural/link/delta checks; prove existing Compose defaults/contracts remain and save exact revision/version evidence for KB-28.

## 7. Known-good freeze and handoff

- [x] 7.1 Obtain independent source/spec/retained-core-acceptance review; resolve findings and re-freeze/retest any changed source/config rather than attaching results to an untested revision; verify KB-26/KB-28 and preserve owner-approval provenance.
- [x] 7.2 Reconcile approved deltas/current specs/RFC/runbooks/TODO; record exact tested artifacts/resources/topology, unchanged/restored runtime hashes, source/build provenance, sanitized limitations and cleanup/coexistence. Publish the evidence-only freeze after proving no source drift (KB-26/KB-27/KB-28).
- [ ] 7.3 Present the known-good baseline for owner acceptance and later comparison planning; identify the separate review needed for outer-TLS/mesh-principal policy overrides while retaining sidecars/inner TLS. Create no comparison worktree, benchmark connection mode, performance run or public cutover in this change.

## Deferred coverage — not completed tasks

- Automatic state-loss monitor extensions and issuer database/key integrity/lineage.
- Exhaustive same-Pod/crash-loop/restart/outage and lost-response/ack recovery.
- Exhaustive cleanup/failure injection and full Kubernetes reliability re-proving
  of JS cases. Mixed-case core security, bounds, receipt/unknown semantics and
  basic owned cleanup remain required in [acceptance](acceptance.md).

These deferrals do not waive source-before-final-tests, exact attestation,
independent application authorization/JWS, actual double TLS or final review.

Parent source/runtime/core and conditional final docs review PASS, 13 October 2026.
The three documentation corrections and strict 12/12 validation passed; tasks
7.1/7.2 close in the evidence-only publication. Task 7.3 owner presentation and
human acceptance remain separate; no comparison work is started.
