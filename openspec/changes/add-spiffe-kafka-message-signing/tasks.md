# Tasks

All tasks are unchecked. Draft authorization is not implementation approval.

## 1. Approval and assessed dependencies

- [ ] 1.1 Obtain explicit owner approval of the final delta/profile and short design; record the actual message/date and artifact digest manifest in review.md. Verify pending status is replaced only by real approval, including acceptance of memory-only crash limits.
- [ ] 1.2 Assess/pin go-jose 4.1.5, franz-go 1.19.5 and the Kafka 4.0.2 image digest plus test-only independent JOSE candidate; verify Go 1.24.2 compatibility, dependency/security findings and baseline tests, and document exact accepted versions without global installs.

## 2. Signing profile and deterministic verification

- [ ] 2.1 Implement bounded compact ES256/x5c codec, strict JSON/base64/curve/chain handling and atomic SVID pairing; verify JS-02–JS-05/SJ-01–SJ-03 vectors/fuzz cases and document the wire format/limits.
- [ ] 2.2 Implement configured context/key binding and captured local trust/time validation; verify JS-04/JS-08/JS-09 boundary tests, cached-outage versus missing/rejected trust classification, and update the profile's verification examples.
- [ ] 2.3 Implement immutable descriptor-driven signing/re-signing with original ID/deadline; verify JS-07/JS-08 rotation, expiry and separate-invocation retries and document creation versus retry commands.

## 3. Shared Money effects and producer integration

- [ ] 3.1 Add Kafka-only guarded shared-ledger access and bounded atomic context binding; verify JS-06/JS-07/JS-08/JS-12 cross-ingress concurrency, conflicts, no history on guard failure, and unchanged original ledger/HTTP tests. Document shared key/reset semantics.
- [ ] 3.2 Add explicit Kafka mode to the trusted executable without altering default UDS control commands; verify mode/config tests and PC-03 regressions, then document the required explicit digest-policy update and single trusted grant.

## 4. Embedded Kafka ingress and fixture boundaries

- [ ] 4.1 Add complete opt-in Money Kafka configuration and embedded consumer; verify disabled/default and incomplete startup cases, original POST /transfer/GET /balances statuses/guards and absence of relay/public trigger routes; document isolated enable/disable settings.
- [ ] 4.2 Implement manual one-partition processing/ack sequencing, pauses/heartbeats and assignment fencing; verify JS-10/JS-11 fake-client fault/generation tests and no offset ahead of undecided work, and document runtime Kafka failure/reset behavior.
- [ ] 4.3 Build stock private Kafka TLS/DN ACL and ephemeral SPIRE fixture provisioning, with per-app credentials and disabled producer idempotence; verify JS-14/JS-15 runtime inspection/cleanup and document exact image/ACL/mount/port facts.

## 5. Integration evidence and reconciliation

- [ ] 5.1 Establish fixture isolation without live/retained state changes, run JS-01–JS-12 and JS-14–JS-15 serially with real SPIRE/Kafka, plus formatting/Go test/race/vet and relevant deterministic checks; save sanitized evidence with revision/versions and mark all unrun failures honestly.
- [ ] 5.2 Run JS-13 bidirectional independent JOSE and RFC vectors; verify exact-byte/curve rejection without exporting keys/SVIDs and record the independent tool/version, distinguishing codec from full policy coverage.
- [ ] 5.3 Reconcile current docs/RFC/baseline evidence and approved deltas, promoting this profile/reference material to a stable documentation location and fixing links before sync/archive; verify strict validation and reference checks. Update TODO only for completed work and retain owner-acceptance/unrun limits.

## Workflow follow-up

- Use OpenSpec verification and present evidence for owner review. Sync/archive
  only through the applicable workflow after implementation is actually complete.
- No deployment or public Kafka trigger is included; any later exposure needs its
  own approved spec/plan and applicable deployment authorization.
