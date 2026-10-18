# Tasks

## 1. Review

- [x] 1.1 Record Rudi's approval of the identified proposal, design and spec revision in `review.md`; verify the actual owner message and artifact digest before runtime edits.

## 2. One signed application path

- [ ] 2.1 Make the existing signed Money handler and HTTP client the only workload HTTP path; migrate callers, UI balance/receipt readers and remote control adapters together. Remove direct workload mTLS helpers/handlers and mode switches, migrate dependent development/security binaries, and reject legacy configuration. Verify S01/S02/S05, `go test ./...` compilation and a source audit showing no selectable workload-mTLS implementation. Update the affected capability contracts and transport documentation.
- [ ] 2.2 Document and exercise independent SVID-chain plus ES256 signature verification in `internal/httpjws`, including public `x5c`, trusted-bundle sourcing, exact SPIFFE ID and current time. Retain bounded wire parsing and same request/response correlation; verify S03-S07 and update the verification documentation. Do not introduce caller-controlled trust or an unsigned fallback.
- [ ] 2.3 Add a local freshness guard to the HTTP atomic ledger decision before history/effects, preserving the Kafka guard and existing identity-bound idempotency. Verify deterministic expiry-while-waiting and lost-response/same-ID tests in S04/S08; update SP-06/MT-04 and ledger boundary documentation.

## 3. Profiles and user outcomes

- [ ] 3.1 Change private Compose workload URLs/readiness/fixtures to signed HTTP and remove obsolete transport flags from Kubernetes renderers. Preserve browser HTTPS, broker TLS, exact mesh flows and UDS isolation. Verify rendered configuration tests plus S01/S02/S09/S10; update Compose, Kubernetes and shared-cluster runbooks without changing running deployments.
- [ ] 3.2 Update Sync labels and caller outcome handling to signature/freshness/authorization failures for every profile. Preserve unexpected-SVID issuance errors, unauthenticated response uncertainty and same-ID recovery. Verify UI/control tests for S02/S05/S08/S10 and reconcile the Pages review specification.
- [ ] 3.3 Replace obsolete application-TLS acceptance fixtures and assertions with real-SPIRE signing/bundle/rotation checks; adapt the opaque backend fault fixture to the signed protocol or remove it with replacement lost-response coverage. Keep browser/broker TLS tests separately scoped. Verify each retained runner still builds and names its actual guarantees; update the RFC T02/T06/T07 mappings and evidence index without relabeling old passes.

- [ ] 3.4 Connect Istio proxies to SPIRE SDS through CSI, use `poc.internal` mesh identities/policies, and register actual proxy containers before readiness (including the Kafka admin Job). Preserve separate application grants and fail closed without a proxy grant. Verify S11 with focused renderer/registration/bootstrap tests; do not mutate shared cluster-wide mesh configuration or deploy live.

## 4. Integration and reconciliation

- [ ] 4.1 Run formatting, Go tests with JOSE interoperability, race and vet, Python deterministic tests, and scoped strict OpenSpec validation. Record commands and outcomes in `verification.md`; distinguish pre-existing unrelated validation failures and infrastructure restrictions from this change's failures.
- [ ] 4.2 Establish fresh isolated SPIRE fixtures, then run relevant Compose and Kubernetes acceptance serially for S01-S11 when the local runtime is available. Preserve retained identity/live state; record actual source revision, observed result and cleanup. Mark any unavailable runtime evidence unrun.
- [ ] 4.3 Reconcile current capability purposes, RFC, README, workspace brief, plan, TODO, `AGENTS.md` transport invariants and OpenSpec context to the approved single protocol. Verify links and consistency; record final evidence limits and leave unchecked work incomplete. No live cutover is included.

## Acceptance matrix

These are required checks for the new behavior, not recorded passes.

| ID | Check and expected result |
| --- | --- |
| S01 | Compose and Kubernetes trusted signed transfers succeed; UI signed reads work; application mTLS is absent and legacy selection fails. |
| S02 | Reachable unidentified callers get no SVID and Money returns signature 401 without mutation; forged identity headers do not help. |
| S03 | Tampered payload/header, wrong key, algorithm confusion and self-signed/attacker-root chains fail without effects; valid unauthorized IDs get 403. |
| S04 | Expired/not-yet-valid SVID, elapsed envelope, clock boundary and expiry while waiting for the ledger lock fail with the specified freshness outcome and no history/effects. |
| S05 | Missing/tampered/wrong-ID/uncorrelated Money or adapter responses are rejected and cannot establish success; browser/broker TLS tests still pass. |
| S06 | New requests use rotated SVIDs; previous current signatures remain verifiable; independent bundle rollover preserves valid verification. |
| S07 | Missing/unusable trust fails closed; a valid cached credential during issuer outage is distinguished from revocation; caller-provided roots never establish trust. |
| S08 | Same-ID retries and concurrent HTTP/Kafka submissions produce one process-lifetime effect; payload conflict remains 409 and unknown outcomes retain IDs. |
| S09 | Wrong-executable/wrong-deployment issuance negatives remain explicit denials with socket reachability; socket/issuer outages are infrastructure failures. |
| S11 | Mesh uses SPIRE SDS and namespace/SA identities with distinct app grants/keys; no proxy grant blocks readiness; bootstrap registers before waiting; trust-domain policies match. Live issuer/rotation evidence is required before claiming runtime acceptance. |
| S10 | Private controls, exact adapter/UI permissions, browser session/CSRF/Host checks, Kafka original-producer verification and all five demo scenarios remain valid with accurate labels. |

## Workflow follow-up

- Archive only after implementation/evidence reconciliation and the applicable review requirements; archive does not authorize deployment.
