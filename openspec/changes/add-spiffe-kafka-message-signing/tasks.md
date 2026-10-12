# Tasks

All tasks are unchecked. This expanded revision needs fresh independent review
and explicit owner approval; the prior published draft PASS is historical.

## 1. Approval and assessed dependencies

- [ ] 1.1 Obtain explicit owner approval of this expanded delta/profile and short plan; record message/date and artifact manifest in review.md, including receipt/deadline/reset limits. Verify no prior agent PASS or publication is substituted.
- [ ] 1.2 Assess/pin go-jose 4.1.5, franz-go 1.19.5, Kafka 4.0.2 image digest and independent JOSE test candidate; verify Go 1.24.2 compatibility/security and baseline tests, documenting exact versions without global installation.

## 2. Signed commands and shared effects

- [ ] 2.1 Implement bounded ES256/x5c codec, strict parsing and matched SVID/key snapshots; verify JS-02–JS-05/JS-13 vectors and document wire limits.
- [ ] 2.2 Implement context/key/time checks with independent cached trust snapshots and Kafka-only guarded shared-ledger/context access; verify JS-06–JS-09/JS-12 cross-ingress and expiry/budget/assignment distinctions, keeping original HTTP guards/tests unchanged.
- [ ] 2.3 Implement fixed signed/unsigned/tampered record builders without an arbitrary-payload API; verify JS-04/JS-05/JS-16 expected classifications and document fixed menu semantics.

## 3. Authenticated Money outcome resource

- [ ] 3.1 Implement complete-record digest framing, instance metadata and observer-only mTLS GET outcome reads; verify JS-17 wrong-reader/no-cert/mismatched-digest/invalid-claim cases and document query versus business statuses.
- [ ] 3.2 Add 4096-slot/180-second receipt reservation and atomic effect/context/result publication, plus retained rejection-digest discard; verify JS-12/JS-18/JS-21 capacity, duplicate time/trust changes and transient guards. Document receipt expiry separately from ledger history.
- [ ] 3.3 Embed the opt-in consumer with manual ordered commits, pauses/heartbeats and assignment fencing; verify JS-10/JS-11/JS-14 fake-client fault/reset cases and unchanged Sync service availability.

## 4. Private callers and bounded recovery

- [ ] 4.1 Add new fixed Kafka publish/lookup controls on existing private sockets while preserving POST /attempt; verify PC-02/PC-04 and JS-16/JS-20 peer/schema/mount regressions, documenting exact request fields.
- [ ] 4.2 Implement runner-initialized caller-only manifest/index, atomic pre-publish variant reservation, original-deadline retention and 64/four limits; verify JS-07/JS-19/JS-22 response loss, restart, corruption/capacity/reset cases and no SVID private-key storage. Document explicit new-attempt versus retry.

## 5. Fixed combined UI

- [ ] 5.1 Preserve Sync labels/APIs and add the three Async scenarios through a separate strict creation API sharing quotas/ownership/CSRF; verify original web tests plus JS-16/JS-20, documenting transport versus background execution.
- [ ] 5.2 Implement authenticated Money metadata/all-variant receipt polling, nonce/reset checks and caller lookup recovery; verify JS-17–JS-19/JS-21/JS-22 queued/terminal/unknown and UI restart cases, including latest-reject/earlier-unknown. Document bounded polling and evidence provenance.

## 6. Compose iteration and native identity isolation

- [ ] 6.1 Add infra/kafka-signing/ Compose/config/templates/helpers/README with reused hardened Go images and stock private KRaft TLS/DN ACL bootstrap; verify JS-14/JS-15/JS-23 compose configuration and port/network/key boundaries, including no UI broker/admin credential.
- [ ] 6.2 Factor a validated fully ephemeral identity/runner mode with own lock/descriptor/socket paths, preserving retained/default lock behavior; verify JS-23 existing-demo coexistence, busy-lock/path collision and failure cleanup. Document native SPIRE and separate loopback browser TLS.
- [ ] 6.3 Add bounded broker/topic/ACL/assignment/HTTPS readiness and owned-resource cleanup; verify JS-15/JS-23 start/stop/failure and caller-state retention within a run, documenting generated ignored paths and reset semantics.

## 7. Integration evidence and reconciliation

- [ ] 7.1 Establish isolated real SPIRE/Kafka/combined-UI fixtures and run JS-01–JS-23 serially plus relevant formatting/Go test/race/vet/deterministic checks; save revision/versioned sanitized evidence and leave every unrun case explicit.
- [ ] 7.2 Run bidirectional independent JOSE interoperability separately from policy tests; verify JS-13 raw-byte/curve negatives without exporting keys/SVIDs and record actual independent versions.
- [ ] 7.3 Reconcile approved deltas/current docs/RFC/evidence, promote normative material to stable documentation and check links before sync/archive; verify strict validation and update TODO only for completed work.

## Workflow follow-up

- Present implementation evidence for independent and owner review. Sync/archive
  only after actual implementation and required approval.
- This revision creates no running stack or live/public deployment. Enabling or
  publishing the combined demo needs applicable later deployment authorization.
