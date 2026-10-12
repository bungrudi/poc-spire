# Kafka signing acceptance and evidence

The approved JS-01–JS-23 matrix is exercised with real isolated SPIRE/Kafka
checks and deterministic security/fault checks. Final results and tested source
revision are in [aggregate evidence](evidence/kafka-final-validation.json).
The [real acceptance](evidence/kafka-acceptance-current.json),
[normal-build smoke](evidence/kafka-smoke-current.json) and
[deterministic checks](evidence/kafka-final-deterministic.json) remain separate.
The parent completed [independent final review](../openspec/changes/add-spiffe-kafka-message-signing/independent-review.md)
with **PASS** on 12 October 2026. Its [separate review/rerun evidence](evidence/kafka-independent-review.json)
confirms all 23 approved cases with no unresolved findings or blockers. Owner
acceptance remains pending.
Intermediate attempts explicitly retain their earlier status; their source tree
may differ from the named commit. A smoke pass alone is not full acceptance.

Source was frozen before the final run. Evidence/docs are committed afterward;
no application, dependency, runner or fixture source changes occur between the
frozen revision and the evidence commit. The reports contain actual source-file
hashes, app binary/image hashes and versions. Full JWS, certificate chains,
private keys, cookies, CSRF tokens and native parent/join tokens are excluded.
Private-state content/permission equality is compared in memory and only its
boolean result is reported.

| Case | Evidence and specific assertions | Mode |
| --- | --- | --- |
| JS-01 | `fixed-kafka-trusted` and five normal-build scenarios: exact trusted signer, authenticated receipt and shared balance/version. | Real SPIRE/Kafka/HTTPS |
| JS-02 | `attestation-wrong-executable` / `attestation-wrong-deployment`: same image/UID and actual Money/Workload API reachability, explicit no matching identity. | Real SPIRE |
| JS-03 | `wrong-valid-original-signer` / `negative-fake-root`: UI SVID signed through an isolated process with no broker key/network, separate producer admits its bytes; unauthorized ID versus untrusted chain. | Real SPIRE/Kafka |
| JS-04 | Destination/key/header/tampering negatives; `TestTrustedSignatureAndExactRecordBindings` includes changed protected bytes and payload without re-signing. Digest excludes broker metadata. | Real + deterministic |
| JS-05 | Sixteen admitted negative cases; `TestStrictJSONAmbiguityUnicodeAndDepth`, `TestProfileLimitsAndAlgorithmHeaders`, `TestInboundCurveDERAndEveryWireBound`: P-384 labeled ES256, DER, canonical encoding, decoded duplicate names, forbidden headers and every wire/chain bound. | Real + deterministic |
| JS-06 | `sync-kafka-shared-id-and-conflict`; `TestKafkaSharesSyncHistoryAndImmutableContext`, `TestConcurrentKafkaAndSyncOneEffect`, `TestOriginalKafkaBusinessRejectionReplaysAcrossIngresses`: both directions, concurrency, unchanged fields/rejections and valid changed-field conflict. | Real + deterministic |
| JS-07 | `real-svid-rotation-private-retry`: observed changed real leaf, original creation/deadline/nonce and one effect; `TestMatchedSVIDRotationPreservesDescriptorAndFourVariantLimit` checks a matched SDK object and bounded variants. | Real + deterministic |
| JS-08 | `TestFreshnessClockBudgetAndAssignmentBoundaries`, `TestSigningAndVerificationEnforceLeafNotBeforeWithoutGrace`, `TestRealVerifiedGuardCrossesExpiryWhileWaitingForLedgerLock`: +5/+6 seconds, exact expiry/notBefore/notAfter, short signing window, deadline and actual mutex contention. No host clock changes. | Injected clock + real expired-envelope rejection |
| JS-09 | `cached-valid-sources-during-spire-outage`; `TestBundleSnapshotsRotationAndMissingTrustClassification`, `TestMissingTrustPausesPendingValidRecordUntilSnapshotRecovers`: matched rotation, immutable old/new bundles, available rejected trust versus missing transient trust and valid cached sources during paused own SPIRE children. | Real outage + deterministic bundle updates |
| JS-10 | Real pre-ack/pre-decision gates and exact offsets; `TestValidDecisionLostAckReassignsWithSameIDAndCurrentGeneration`, `TestTerminalInvalidRecordsCommitInOrderThroughLostAck`, `TestPausedRecordRevocationNeverAcknowledgesOldAssignment`: decision precedes ack, next offset only, pauses/heartbeats, lost ack, old epoch revoked and valid same-ID redelivery under new member/generation. | Real + fake transport |
| JS-11 | `sigkill-after-ledger-before-ack`, `sigkill-before-decision`, `money-restart-offsets-and-ui-nonce`: actual SIGKILL, exact original retained digest without republishing, new Money lifetime/version 1 and retained committed offset comparisons. | Real process crashes/Kafka |
| JS-12 | `TestExactReceiptAndHistoryCapacityLimits`: actual 10000 history/context keys, existing replay at capacity and no eviction/binding/receipt for a new key; `TestHistoryCapacityPausesWithoutPollingLaterRecordsOrBlockingReplay` and existing atomic conservation/funds/overflow tests. | Deterministic exact capacities |
| JS-13 | `TestRFC7515AppendixA3ES256` and explicit `TestIndependentJOSEBidirectionalES256`: Go→jose and jose→Go, RFC vector, altered bytes/curve negatives. Node process reports installed package version; private keys stay in their own process. | Independent Node jose 6.1.0 + Go |
| JS-14 | `broker-principal-denied`, `broker-topic-denied`, `broker-group-denied`, `broker-wrong-host`; constructor test instantiates real clients with bounded options. Actual single Money container, one topic partition and exact group offset checked; rendered replica count above one rejected. No cluster/transactional grant. | Real broker + configuration checks |
| JS-15 | Actual container boundaries, normal readiness/cleanup, fresh stream/context negatives and reused-stream new-lifetime weakness. Before/after live service state/PID/HTTPS and protected retained state equality. | Real |
| JS-16 | Five fixed Sync/Async scenarios; `TestKafkaBrowserSchemaAndSecurityShareExistingBoundary`, `TestNewKafkaRoutesKeepPeerBoundaryAndOldSyncSchema`; shipped JS fixed-menu/key controls. Normal binaries reject test-only flags before startup. | Real + deterministic browser/control |
| JS-17 | `observer-resource-wrong-reader` is 403; `observer-resource-no-certificate` verifies exact Money then receives TLS rejection with no HTTP status. `TestReceiptReadIsBoundedCorrelatedAndSeparatesHTTPStatus` / UI mismatch tests deny altered instance/digest/result. Invalid signatures remain unattributed. | Real mTLS + deterministic correlation |
| JS-18 | Actual 4096 receipt slots and exact 180-second expiry; `TestReceiptCapacityRetentionAndPositiveImmutability`, guards/engine pause tests: no effect before reservation, no fresh eviction, expiry does not remove history, transient failure has no terminal receipt. | Deterministic exact capacity/clock |
| JS-19 | `TestPublicationReservationSurvivesAckLossAndCallerRestart`, `TestAllVariantsSettleAndInvalidRecordsStayUnattributed`, `TestEarlierQueryFailureCannotHideAnotherAuthenticatedAppliedVariant`: prepublication reservation, ambiguous ack, every digest, latest reject plus earlier unknown, earlier applied result despite another read error. | Private state/fake IO + real restart recovery |
| JS-20 | Actual HTTPS CSRF/Origin/Host/fixed-schema and cross-caller mount denial. Existing session/owner/peer tests plus `TestKafkaAndSyncShareConcurrencyAndRateSlots` prove one quota boundary; local viewer/owner tests apply to the local preview, not human authentication in the assumed-operator profile. | Real boundary + deterministic quotas/roles |
| JS-21 | `TestRejectedDigestRemainsImmutableWhenTimeOrTrustBecomesValid`: real verifier then ledger rejects future time/untrusted root, subsequently valid same bytes still have no effect, new bytes fully verify, cached positive receipt cannot authorize an expired message. | Deterministic time/bundle updates |
| JS-22 | Real UI/caller restart preserves all variants/deadline; actual caller-state deletion plus UI restart remains unknown with no automatic publication or initialization, Sync still works. Private-state tests cover checksum/manifest/index corruption, nonce mismatch, 64 descriptors/four variants and deadline expiry. | Real + deterministic private state |
| JS-23 | Full broker TLS/topic/ACL + Money assignment + HTTPS readiness; actual port collision leaves the active stack intact and removes only the colliding temporary state. Own busy lock/alias/marker tests; cleanup timeout/nonzero/native failure propagates and safe retry retains ownership. Actual public PID/health and retained identity stay unchanged. | Real + failure injection |

Named tests live in [codec](../internal/kafkacmd/codec_test.go),
[independent interoperability](../internal/kafkacmd/interop_test.go),
[shared ledger](../internal/ledger/kafka_test.go),
[held-lock guard](../internal/ledger/kafka_boundary_test.go),
[consumer](../internal/kafkaio/engine_test.go),
[client constructor](../internal/kafkaio/client_test.go),
[caller state](../internal/kafkacaller/store_test.go),
[Money reader](../internal/moneyreader/outcomes_test.go),
[Money routes](../internal/money/kafka_test.go),
[controls](../internal/control/kafka_test.go), and
[web](../internal/web/kafka_test.go). The real harness is
[scripts/kafka_acceptance.py](../scripts/kafka_acceptance.py); infrastructure and
cleanup checks are in the three `scripts/test_kafka_*.py`/identity-isolation files.

## Practical limits

These checks cover the approved learning profile, not production payments.
Money has no durable ledger, replicated state or atomic offset/business database.
The SIGKILL tests intentionally prove that an unacknowledged valid record can
apply again in a new lifetime. A second Money replica is unsupported. Signatures
provide provenance/integrity; broker TLS is separate admission, and neither
provides confidentiality from the broker or protection from host/authorized
process compromise. Executable attestation is not continuous runtime integrity.

Browser acceptance above exercises the actual HTTPS APIs and deterministic
shipped-JavaScript controls. Rendered-browser visual/manual testing is unrun.
Real root removal/rollover of the long-lived public deployment, production-TTL
observations, automatic expired-root sweeping, scheduled ACME renewal and
arbitrary kernel packet-loss injection are unrun here. Bundle update/snapshot
and exact clock/capacity cases use controlled local fixtures. No public Kafka
runtime deployment or new persistent credential/access change is performed.

## Reproduce

Use Go 1.24.2 and an existing Node >=20.19.0; this host uses Node 22.23.0. Install
only locked local dependencies with `npm ci`. Run formatting checks, Go tests,
`go test -race ./...`, `go vet ./...`, `go test -tags kafkatest ./...`, Python
`python3 -m unittest discover -s scripts -p 'test_*.py'`, and
`node scripts/kafka_browser_controls.cjs`. Explicitly run interoperability with
`POC_JOSE_INTEROP=1 go test ./internal/kafkacmd -run TestIndependentJOSE -count=1`.

Run `python3 scripts/kafka_acceptance.py` and then
`python3 scripts/kafka_smoke.py` serially with Docker available, an unused
loopback 18088 port and the measured native UID-1000 layout. Temporary resources
are removed at completion. Run `npm run spec -- validate --all --strict` for
structural validation. None of these commands deploys the public demo. See the
[runner runbook](../infra/kafka-signing/README.md) for interactive start/stop and
[the profile](kafka-signing-profile.md) for reset and retry semantics.
