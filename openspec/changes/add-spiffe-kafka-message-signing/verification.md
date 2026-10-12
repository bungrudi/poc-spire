# Implementation verification — 12 October 2026

Using change `add-spiffe-kafka-message-signing`, stock `spec-driven` schema.
This is the implementing agent's verification, not the independent parent review
or owner acceptance. The actual owner approval and frozen approved revision are
in [review.md](review.md). All concrete proposal/design/task/spec paths returned
by the pinned CLI were inspected; apply context and operation guidance preserve
the owner gate, scoped implementation and current assumed-human demo.

| Dimension | Result |
| --- | --- |
| Completeness | 18/18 implementation tasks; 34 ADDED/MODIFIED requirements and 86 affected scenarios mapped. No removal/rename operation. |
| Correctness | Original direct mTLS/default Sync behavior retained; new signing, consumption, receipts, private recovery and UI behavior map to JS-01–JS-23 real/deterministic evidence. |
| Coherence | The approved short design is followed: one embedded consumer/shared memory ledger, independent trust/current-time guards, fixed separate APIs, bounded initialized recovery, exact-Money observer evidence and fully isolated native SPIRE/Compose. |

Tested application/runner/fixture source is frozen at
`81fdb50ecc4879b027c584fc3a87d1802cffccdf`. Both final normal and `kafkatest`
build reports name that revision, report no source differences, contain matching
source-file manifests and pass owned cleanup. Evidence/documentation follow in a
separate commit with no source changes. The real report compares actual public
service state/PID/HTTPS health before/after and private retained identity
content/permissions in memory; both remain unchanged.

[Aggregate evidence](../../../docs/evidence/kafka-final-validation.json) and
[granular JS coverage](../../../docs/kafka-signing-acceptance.md) identify the
43 real checks, five normal-build scenarios/test-flag absence and fresh Go
formatting/tests/race/vet/test-build, 21 Python checks, shipped-JS controls and
explicit Go↔jose 6.1.0 interoperability. The ordinary Go runs explicitly skip
independent JOSE; the separate enabled run passes it. Strict validation covers
all eight current capability specs and the active change.

## Requirement implementation mapping

| Approved requirement | Source boundary | Principal proof |
| --- | --- | --- |
| SJ-01 | `internal/kafkacmd/{codec,verify}.go` | JS-04/05/13 exact compact ES256 signing bytes and wire negatives. |
| SJ-02 | `internal/kafkacmd/{codec,json}.go` | JS-05 all bounds/ambiguity/Unicode/unknown fields. |
| SJ-03 | `internal/kafkacmd/verify.go`, narrow native registrations | JS-02/03 attestation, independently trusted root and exact-ID denial. |
| SJ-04 | `internal/kafkacmd/verify.go`, immutable startup policy | JS-04 signed destination/key/header negatives. |
| SJ-05 | `internal/kafkacmd/verify.go`, guarded ledger acceptance | JS-08/09 clocks/path intervals/snapshot budget/held lock. |
| SJ-06 | `internal/kafkacmd/codec.go`, `internal/kafkacaller/sender.go` | JS-07 matched SDK leaf/key rotation, immutable original context. |
| SJ-07 | codec/interop tests and sanitized reports | JS-13 RFC vector and independent bidirectional signatures. |
| KC-01 | `cmd/money/main.go`, `internal/kafkaio/client.go` | JS-01/16 opt-in embedded consumer, same balances/default Sync. |
| KC-02 | `internal/ledger/{ledger,kafka}.go` | JS-06 current verification then shared history and business conflict/rejection replay. |
| KC-03 | `internal/ledger/kafka.go` | JS-06/07/12 context binding and no transient insertion/eviction. |
| KC-04 | `internal/kafkaio/{consumer,engine}.go` | JS-10 terminal decision before ordered next-offset acknowledgement. |
| KC-05 | same consumer/engine and ledger guards | JS-09/10/12 missing trust/capacity/ack uncertainty and new-generation valid redelivery. |
| KC-06 | ledger construction/consumer restart and UI nonce checks | JS-11 actual pre/post-decision SIGKILL with exact retained bytes and fresh lifetime. |
| KC-07 | stock broker config/PKI/bootstrap and per-app mounts | JS-14/15 separate TLS/DN ACL admission and cleanup. |
| KC-08 | `internal/money/server.go`, `internal/moneyreader/outcomes.go` | JS-17 exact observer/method/digest resource; real 403 versus no-cert TLS failure. |
| KC-09 | `internal/ledger/kafka.go` | JS-18/21 exact 4096/180s capacity, atomic reservation, immutable rejected bytes and current verification of positives. |
| KC-10 | `infra/kafka-signing/`, isolated runner/native identity | JS-23 readiness, actual collision/coexistence and failure cleanup. |
| FL-02 | shared typed ledger history and Kafka context | JS-06/12 both directions/concurrency/business equality. |
| FL-04 | one memory-only ledger per Money startup | JS-11/22 offsets do not reconstruct history and UI-only restart preserves Money. |
| IC-01 | updating SDK source, fresh HTTP client and message verifier | JS-07/09 real rotation/outage and no expired-signer bypass through broker TLS. |
| MT-01 | Money's mandatory direct Go SPIFFE TLS server | JS-01/17 normal Sync/no-cert and outcomes mTLS. |
| MT-02 | exact-ID method/route policy before ledger | JS-03/17 valid wrong reader denies, observer only balances/receipts. |
| MT-03 | caller/UI exact Money SDK verifier | real verified reads, no-cert probe and reader wrong-server tests. |
| MT-04 | original HTTP body/credential guards | unchanged Money expiry/body/security tests pass. |
| PC-02 | existing peer-UID wrapper and fixed route schemas | JS-16/20 old Sync shape preserved; wrong peer/extra fields fail. |
| PC-03 | caller-owned updating source and unsigned issuance probe | JS-01/02/16 no borrowed key; reachable no identity distinct from API failure. |
| PC-04 | separate control publication and Money receipt evidence | JS-17/19 queued/unknown and no fabricated business/identity status. |
| PC-05 | `internal/kafkacaller/control.go` | JS-16/20 exact new fields, bounded private routes and caller-specific cases. |
| PC-06 | `internal/kafkacaller/{store,sender}.go`, runner-only init | JS-07/19/22 64/four bounds, fsynced reservation, deadline/nonce and lost/corrupt state. |
| UI-04 | existing ID derivation and shared dispatch | JS-16/19/22 stable browser keys and unknown recovery. |
| UI-05 | own observer source and shared session/dispatch quotas | JS-17/20 unchanged ownership, read-only identity, one capacity/rate/concurrency boundary. |
| UI-07 | fixed combined template and strict creation route | JS-16/20 two Sync plus three Async cases, no payload/destination API. |
| UI-08 | `internal/web/kafka.go` correlated all-variant settlement | JS-17/19 any authenticated expected applied result, all variants needed for rejection. |
| UI-09 | bounded worker/read budgets and initialized lookup recovery | JS-18/19/22 404/expiry/reset stays unknown, no deadline refresh/republication. |

Main specs were semantically merged, retaining original unmodified requirements
and every original scenario heading in modified requirements. Stable profile,
references, acceptance, RFC, baseline, README, workspace, plan and TODO are
reconciled. Test-only fixed-case commands/fault gates are isolated by build tag;
normal binaries reject their flags before source/control startup. Small package
structure, formatting, minimal pinned dependencies and credential isolation
follow the existing project patterns.

## Assessment and limits

No critical or warning implementation finding remains in the checks above.
This verification covers the approved implementation and stated acceptance
proofs; it does not constitute independent review or owner acceptance. Both
remain pending in the final review/aggregate records. No archive or public
runtime deployment was performed.

Rendered-browser visual/manual checks, long-lived public root operations,
production-TTL/automatic expired-root sweeping, scheduled ACME renewal and
arbitrary kernel packet-loss are explicitly unrun. They are outside this bounded
acceptance profile, rather than silently counted as passes. The known in-memory
restart weakness is demonstrated, not resolved by offsets or caller files.
