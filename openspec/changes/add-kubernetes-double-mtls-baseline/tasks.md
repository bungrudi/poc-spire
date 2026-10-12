# Tasks

Owner approval of the full delta/short plan and frozen revision is recorded in
[review](review.md). Planning/structural readiness alone did not approve runtime
execution. Mark remaining tasks only after their complete proofs pass. See
[acceptance](acceptance.md) for KB proofs;
all new runtime cases are NOT RUN. Update affected source/tests/current
contracts/RFC/runbook and sanitized evidence as each approved task completes.

## 1. Approval and reproducible preflight

- [x] 1.1 Obtain actual owner implementation approval of this complete profile/delta/short plan; record message/date and frozen artifact manifest, separate from parent drafting review/publication permission.
- [ ] 1.2 Resolve/check exact proposed tools, images/charts/manifests and registry index/platform/config digests; assess current compatibility/security, keeping existing app dependency/connection pins; document KB-01/KB-26 assumptions and failure gates.
- [ ] 1.3 Verify capacity/reserve, unused ports/CIDRs/names, owned kubeconfig/context and declared kind/CNI/CSI/Agent privilege boundary; pass KB-01/KB-25 or stop without touching shared services.

## 2. Owned cluster and independent mesh substrate

- [ ] 2.1 Add scoped disposable kind bootstrap/ownership/cleanup with the pinned one-node image and explicit resource limits; install enforcing Calico, prove KB-01/KB-04/KB-27 and retain Compose/public continuity.
- [ ] 2.2 Configure signed kubelet serving certificates with narrowly validated node CSRs, verified CA/hostname/token and minimal API/kubelet RBAC; pass KB-02 secure-access negative/positive proof, without anonymous/skip-verification fallback.
- [ ] 2.3 Install pinned Istio sidecar/CNI/default Istiod CA and explicit system injection exclusions; verify injection/SDS/current credentials, opaque TCP STRICT/ISTIO_MUTUAL and documented exact flows/exceptions in KB-03/KB-09/KB-10, including bootstrap Job completion.

## 3. In-cluster SPIRE and feasibility gate

- [ ] 3.1 Deploy owned ephemeral SPIRE Server and node Agent/CSI with PSAT, scoped private admin/bootstrap RBAC and node-local socket backing; pass KB-02/KB-05 reachability/bootstrap/permission checks with no native host SPIRE.
- [ ] 3.2 Establish expected run/Pod UID barriers without an ImageID/SVID-before-container-start deadlock; derive observed immutable requesting-container/image/owner selectors and exact parents, refuse broader/duplicate grants; prove KB-05/KB-06 and explicit binary-update denial.
- [ ] 3.3 Prove Unix executable SHA-256 plus K8s conjunction for UID 1000 apps and UID 10002 adapters under the actual container runtime; execute reachable wrong-binary/wrong-deployment and adapter app-grant/ineligible-executable tests in KB-06/KB-07; inspect normal proxy Workload API/application-key absence structurally, without claiming unreachable issuance denial. Stop dependent migration if the gate fails.
- [ ] 3.4 Implement restricted authentic public-bundle publication/projection and configured caller trust path with existing defaults intact; test complete atomic refresh, valid cache, missing/wrong trust and expired sources in KB-08/KB-16 and IC-02/IC-03.

## 4. Private network controls and disposable application state

- [ ] 4.1 Add separately attested UID 10002 control adapters with exact UI authorization, exact adapter server-ID checks, local UID-checked sockets, schemas/deadlines/limits and expiry/unknown-response guards; prove KB-11/KB-12 plus unchanged PC/Go control tests.
- [ ] 4.2 Add Kubernetes-only UI control transport selection without caller state/socket/key mounts or API schema/ID/quota changes; verify fixed scenarios, malformed input/roles/CSRF/Host/Origin and cross-caller denial in KB-11/KB-17/KB-18.
- [ ] 4.3 Initialize bounded caller-only state once for expected run/Pod UIDs; implement failure/run guards that stop both Sync/Kafka UI creation/retry and adapter forwarding with 503 run_invalid after destructive loss, plus explicit fresh start and browser stale-run context while preserving intact container restart recovery; prove KB-19/KB-20/KB-21 and original 64/four/deadline bounds.

## 5. Full broker/application deployment and readiness

- [ ] 5.1 Add in-cluster stock one-node Kafka, bounded disposable data, separate ephemeral broker/browser PKI, TLS hostname/DN ACL/topic bootstrap and bounded injected Job lifecycle; prove KB-09/KB-14/KB-23 and no UI/adapter broker/admin credential.
- [ ] 5.2 Deploy one Money/UI/two caller Pods and adapters with hardened app contexts, private state/credential separation and no app Kubernetes/admin mounts; preserve embedded consumer/shared ledger and unchanged inner HTTP/JWS guards; prove KB-04/KB-13/KB-17/KB-25.
- [ ] 5.3 Implement complete native dependency readiness/exec checks and dispatch gating, preserving valid cached credential behavior during issuer-only outage; prove cold-start ordering, partial-readiness/failure and cached-expiry classifications in KB-03/KB-05/KB-16/KB-22.

## 6. Migration acceptance and honest layer evidence

- [ ] 6.1 Commit final source, fixtures, dependencies and normal/test configuration before runtime evidence; build/pin normal and separately identified test images, verify absence of test/benchmark controls in normal builds, and record KB-26 manifests.
- [ ] 6.2 Run observed positive and negative layer proofs: mesh-valid/app-invalid, valid-app/no-mesh, wrong valid app ID 403, fake root/headers, broker admission versus JWS; correlate sanitized actual TLS observations for KB-09–KB-15 without exporting keys/SVID/JWS/SDS/cookies.
- [ ] 6.3 Run all five UI scenarios and full JS-01–JS-23 Kubernetes mapping serially with the explicit JS-22 invalid-run availability delta and separate unchanged Compose expectation, including guarded timing/trust/assignment/capacity, all variants/response loss, restart/reset/SIGKILL and run-owned failure cleanup; complete KB-17–KB-27 with every proof mode/unrun case explicit.
- [ ] 6.4 Run formatting, Go tests/race/vet, relevant deterministic Python/browser/independent JOSE tests and structural/link/delta checks; prove existing Compose defaults/contracts remain and save exact revision/version evidence for KB-28.

## 7. Known-good freeze and handoff

- [ ] 7.1 Obtain independent source/spec/full-acceptance review; resolve findings and re-freeze/retest any changed source/config rather than attaching results to an untested revision; verify KB-26/KB-28 and preserve owner-approval provenance.
- [ ] 7.2 Reconcile approved deltas/current specs/RFC/runbooks/TODO; record exact tested artifacts/resources/topology, unchanged/restored runtime hashes, source/build provenance, sanitized limitations and cleanup/coexistence. Publish the evidence-only freeze after proving no source drift (KB-26/KB-27/KB-28).
- [ ] 7.3 Present the known-good baseline for owner acceptance and later comparison planning; identify the separate review needed for outer-TLS/mesh-principal policy overrides while retaining sidecars/inner TLS. Create no comparison worktree, benchmark connection mode, performance run or public cutover in this change.
