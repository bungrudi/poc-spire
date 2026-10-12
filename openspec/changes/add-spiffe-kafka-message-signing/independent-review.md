# Independent implementation review — 12 October 2026

**Result: PASS for the approved implementation and JS-01–JS-23 acceptance scope.**
No unresolved review finding or blocker remains. This report is the parent
orchestrator's independent review, distinct from the implementing agent's
[verification](verification.md). It does not claim owner acceptance, public
Kafka deployment or archival of the change.

Reviewer: parent chat `01a12b31-9ead-7e92-a024-23860cfe5d8f`.
Implementer: project chat `01a1201a-4782-7483-80b9-5f019b24cf38`.
Approved scope: `735b80fad783bc0350740b18229f61aee8f8aa5d`.
Independently reviewed and tested candidate:
`111cb1a691ae8583a18b823fc5ef6cdd2d85e7a6`.
Application, dependency, infrastructure, runner and fixture source is identical
to frozen source `81fdb50ecc4879b027c584fc3a87d1802cffccdf`.
The parent reran the checks in a separate detached checkout and compared the
actual normal/test-build source manifests with the frozen candidate.

## Assessment

| Dimension | Independent result |
| --- | --- |
| Completeness | All 18 implementation tasks and 23 approved acceptance cases are covered. Exact delta-to-main-spec comparison found all 34 affected requirements and 86 scenarios, no differences, and all 26 original requirements retained. |
| Correctness | The original Sync HTTP/mTLS routes and default scenarios remain. Three added Async Kafka scenarios use Money's embedded consumer and shared ledger. Original-producer ES256 JWS verification, independent SPIFFE trust, exact identity/context/time authorization, replay/conflict behavior, receipts and private retry boundaries were reviewed and exercised. |
| Coherence | Implementation follows the approved single-Money, bounded memory/recovery design and isolated Compose/native-SPIRE setup. Test-only fault gates remain outside normal builds. Evidence states each real or controlled proof mode and demonstrates the process-restart limitation. |

The requirement-to-source mapping in [implementation verification](verification.md)
and the granular [acceptance mapping](../../../docs/kafka-signing-acceptance.md)
were checked against source and test evidence. Main-spec reconciliation retains
the original contracts, with only approved modifications. All applicable
verification dimensions were completed.

## Independent rerun

The [independent aggregate](../../../docs/evidence/kafka-independent-review.json)
links the separately generated reports:

- [Real acceptance](../../../docs/evidence/kafka-independent-acceptance.json):
  43 checks passed, including actual attestation denials, valid-but-unauthorized
  signer rejection, broker ACL/hostname denial, signed-record negatives,
  cross-ingress idempotency, SVID rotation, cached valid identities during an
  outage, private-state loss and actual pre/post-decision Money SIGKILL cases.
- [Normal-build smoke](../../../docs/evidence/kafka-independent-smoke.json):
  all five fixed scenarios passed (two Sync and three Async Kafka), complete
  readiness passed, and test-only flags were absent. This is seven checks.
- [Deterministic verification](../../../docs/evidence/kafka-independent-deterministic.json):
  145 named Go race tests and 145 named test-build tests passed, with zero skips
  and `POC_JOSE_INTEROP=1` enabled. Bidirectional Go/Node jose 6.1.0 checks passed.
  Go vet and formatting, 21 Python tests, shipped-JavaScript controls and strict
  OpenSpec validation (eight specs plus this change) passed.

The real acceptance report alone intentionally has `full_acceptance: false`:
full approved acceptance is established by the combined real, deterministic,
normal-build and source-review evidence above. Exact clock/capacity boundaries,
bundle snapshot updates and selected lost-ack/reassignment behavior use the
controlled modes documented in the approved matrix; they are not represented as
real production outages.

All temporary runtime cleanup succeeded. The existing public demo's actual
service/PID/HTTPS checks and retained private-identity comparisons passed before
and after the real run. Neither the public runtime nor the separate primary
checkout was changed. Source manifests and application binary/image hashes are
retained in the two runtime reports. Runtime verification finished at
`2026-10-12T08:27:33Z`.

## Findings resolved before sign-off

Earlier parent findings were corrected and rechecked: cleanup failure propagation
and retry ownership; separation of browser and broker credential trust; quoted
Compose tmpfs options; compatible bounded Kafka client options; fixture outcome
and immutable-context/recovery assertions; exact hostname-error verification with
a healthy positive control; and missing real attestation, broker, rotation and
crash/redelivery coverage. No critical or warning finding remains.

The parent checked 330 local Markdown links at the tested revision with no
missing target. The publication scan covered 98 changed files. Its single PEM
pattern hit in `scripts/jose_interop.mjs` is a delimiter used to format runtime
public certificate bytes, not a stored certificate or key; manual review resolved
that false positive. No full signed message or private credential was found in
published evidence. The follow-up publication is limited to documentation and
sanitized evidence; executable source remains frozen.

## Practical limits and unrun checks

Money still has one in-memory ledger. A retained valid record can apply again
after Money restarts; the real crash tests demonstrate this explicitly. Kafka
offsets and caller files do not provide durable exactly-once money transfers.
Broker TLS/ACLs control admission separately from original-producer signatures.
Neither layer protects against an already compromised authorized process/host
or hides message content from the broker.

Rendered-browser visual/manual QA, long-lived public root rollover/removal and
automatic expired-root sweeping, production-TTL observations, scheduled ACME
renewal, arbitrary kernel packet-loss injection and public Kafka deployment were
not run. They are outside the approved bounded acceptance profile. The existing
API and shipped-JavaScript checks do not assert a rendered visual review.
