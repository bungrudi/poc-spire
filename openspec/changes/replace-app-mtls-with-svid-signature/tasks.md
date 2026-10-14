# Tasks

## 1. Owner review

- [x] 1.1 Record Rudi's explicit approval of this proposal, the spec deltas and `design.md` in `review.md`, including the approval date, message reference and approved revision. Pending review, validation and commit permission do not complete this task.
- [x] 1.2 Stop before any behavior-changing code if approval is missing or the approved scope changes.

## 2. Mesh HTTP signature path

- [x] 2.1 After approval, teach Kubernetes UI, trusted caller and Money to send plaintext HTTP inside the existing Istio tunnel and to sign and verify REST payloads per SP-01–SP-10. Do not remove Compose direct mTLS.
- [x] 2.2 Verify trusted transfer, UI read-only 403, missing-signature 401, forged-chain 401, route replay rejection and same-ID retry. Verify tampered payload bytes return 401, a non-verifying signature returns 401, and an expired, not-yet-valid or elapsed-envelope signature returns 403 `credential_not_current`. Verify a previous SVID still inside its certificate lifetime and envelope is not rejected as obsolete. Confirm Envoy does not authorize from the JWS.

## 3. Caller and UI outcomes

- [x] 3.1 Update mesh caller and control-adapter evidence so signature rejection, mesh denial and 403 stay distinct. Keep non-mesh TLS rejection unchanged.
- [x] 3.2 Update mesh observer reads to require Money's exact response signature. Keep browser HTTPS, CSRF and UDS controls unchanged.

## 4. Proof and contracts

- [x] 4.1 Run formatting, `go test ./...`, race and vet, then the relevant real-SPIRE Kubernetes checks one at a time in isolation. Mark unrun checks as unrun.
- [x] 4.2 Reconcile the approved deltas, RFC note and baseline links only after those checks. Do not treat this plan as implementation.
