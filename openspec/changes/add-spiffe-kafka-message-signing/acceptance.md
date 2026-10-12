# Proposed acceptance matrix

The reviewed planning body below records the scope frozen at `735b80f`.
[Actual owner approval](review.md) followed on 12 October 2026; draft/unrun
statements in that historical body refer to the time before approval and tests.
Current implementation rules and measured coverage are in the
[stable profile](../../../docs/kafka-signing-profile.md) and
[acceptance/evidence](../../../docs/kafka-signing-acceptance.md).


Every JS case below, including added UI/receipt/Compose cases, is **unrun**. Structural draft validation is not a signature,
Kafka, SPIRE or crash test. Existing [baseline evidence](../../../docs/spec-baseline.md)
is reference coverage only. Future evidence must record versions, tested revision,
bundle/record hashes, classification, ledger snapshots/versions, processed/committed
offsets and cleanup; never keys, full JWS/SVID chains or credentials.

| Case | Requirements | Proposed proof |
| --- | --- | --- |
| JS-01 Trusted original producer | SJ-01,SJ-03; KC-01 | Real narrowly attested trusted binary signs; Money's embedded consumer verifies exact original ID and shared GET /balances reflects one fixed transfer. Sync HTTP/control behavior remains unchanged; combined UI adds the separately labeled Async menu. |
| JS-02 Wrong executable/deployment, no SVID | SJ-03 | Same UID/image/deployment wrong binary and approved binary/wrong deployment reach Workload API but get no identity. Distinguish permission denial from timeout/socket/broker failure. Unexpected issuance fails the suite. |
| JS-03 Wrong valid ID and fake root | SJ-03; KC-07 | UI/test-ID valid SVID denies after authentication. Correct claimed ID signed under supplied fake root fails independent trust. Broker-admitted negative fixtures ensure ACL failure is not substituted for JWS rejection. |
| JS-04 Byte/type/destination/key changes | SJ-01,SJ-04 | Tampered header/body fails signature. Independently re-signed wrong type/environment/audience/stream/topic and changed record key fail binding without mutation/history access. Offsets/timestamps/identity headers never authorize. |
| JS-05 Stripped/malformed/algorithm confusion | SJ-01,SJ-02; KC-04 | Missing signature, none/HS256/ES384, P-384 labeled ES256, DER signature, noncanonical base64url, duplicate/escaped names, trailing/unknown fields, depth/size/chain limits and crit/jku/jwk/x5u are rejected; no fetch or fallback. |
| JS-06 Exact duplicates and cross-ingress conflict | KC-02; FL-02 | Sync→Kafka, Kafka→sync and concurrent same-ID requests mutate once. Current valid changed business fields conflict before another mutation. Original business rejection also replays; no certificate/signature-based namespace. |
| JS-07 Re-sign descriptor and context | SJ-06; KC-03 | Separate private-control retries reuse initialized retained descriptors across rotation, producing new signatures/times but one debit. Changed original creation/deadline conflicts once Kafka-bound. Missing/expired descriptor never silently creates a retry ID/deadline. |
| JS-08 Expiry and clock skew at commit | SJ-05 | Inject clock at +5/+6-second future-issued, exact expires_at, notBefore/notAfter, near-expiry signing, original deadline and lock-contention expiry. Guard failure has no history/binding/mutation. Claimed historical signing time cannot rescue expiry. No host clock is changed. |
| JS-09 Rotation/rollover/outage snapshots | SJ-03,SJ-05,SJ-06; IC-01 | Atomic matched key/leaf rotation; valid cached source works during outage. Missing bundle is transient/no offset; forged chain against available trust is terminal/ack. After updated bundle snapshot, removed-root signatures reject. A captured bounded attempt has its documented snapshot boundary. |
| JS-10 Offset order/ack loss/revocation | KC-04,KC-05 | No commit before ledger decision; commit next offset only. Inject lost ack, timeout, pause and generation revoke/reassign while heartbeats continue. Later records never pass undecided ones; redelivery uses same original ID and live ledger replay. |
| JS-11 Memory-only crash/restart | KC-06; FL-04 | Crash before commit and after ledger commit/before ack, then restart Money against retained broker data/offsets. Confirm balances/history/context reset, offsets do not restore state, and still-valid redelivery can apply anew. No durable guarantee inferred. |
| JS-12 Capacity and business bounds | KC-02,KC-03,KC-05 | Fixed transfer, integer conservation, insufficient funds/overflow, body bounds, 10000-key capacity and context bindings. Existing keys replay at capacity; new keys pause without offset/eviction/binding. Preserve baseline unit and HTTP results. |
| JS-13 Independent interoperability | SJ-07 | RFC7515 Appendix A.3 ES256 codec vector; Go→independent JOSE and reverse v1 envelopes with temporary X.509-SVID fixtures, wrong-curve/altered-signature cases. No byte-equality expectation for randomized ECDSA signatures. Library vectors alone do not prove SPIFFE/Kafka policy. |
| JS-14 Broker admission and single ownership | KC-01,KC-04,KC-07 | Verify stock SSL DN/topic/group ACLs, TLS hostname/current certificate, disabled idempotent-write and no cluster/transactional privileges; consumer auto-commit off, one instance/member/partition. Rogue broker principal denied separately; second Money instance is unsupported, not falsely deduplicated. |
| JS-15 Isolation, reset and evidence | KC-06,KC-07 | No host broker ports, app admin/Docker/key sharing or UI broker network; ephemeral broker PKI/trust/state removed on success/failure. Fresh stream rejects old records; intentionally reused stream retains documented restart weakness. Sanitized evidence does not change live demo or retained identity state. |
| JS-16 Fixed menu and Sync compatibility | UI-07; PC-02–PC-05; KC-01 | Two existing scenarios labeled Sync (HTTP/mTLS), three fixed scenarios Async (Kafka). Original POST /api/attempts and POST /attempt schemas/HTTP outcomes remain intact. New routes reject arbitrary payloads/destinations and unsupported scenarios. |
| JS-17 Authenticated receipt and invalid correlation | KC-08; UI-08 | Observer reads exact-Money mTLS metadata/receipts. Wrong reader gets 403; no cert fails TLS. Invalid records claiming other IDs correlate only by complete topic/key/header/value digest. Mismatched digest/instance/result never proves success; balances/logs/broker ack cannot substitute. |
| JS-18 Receipt retention/capacity and unknown | KC-09; UI-09 | Reserve before effects; 4096 fresh slots are not evicted. TTL 180-second expiry/404 is unknown, not non-commit. Receipt expiry does not remove ledger dedup. Budget/assignment failure creates no terminal receipt; message/certificate expiry is terminal. |
| JS-19 All variants and ambiguous publication | UI-08–UI-09; PC-06 | Lost POST response/broker ack recovers reserved digests. Latest rejected plus earlier missing/expired evidence keeps the logical attempt unknown; any matching-instance authenticated applied result establishes success. Query every potentially submitted variant; no fresh retry ID/deadline. |
| JS-20 Shared UI/UDS defenses | UI-04–UI-07; PC-01–PC-05 | Shared 64-attempt/two-worker/eight-per-minute quotas and cookie/session/CSRF/origin/Host/operator/owner checks cover both groups. Private socket/mount isolation persists; UI has no producer key, broker credential, admin credential or Kafka network. |
| JS-21 Immutable rejection decision under change | KC-05,KC-09 | Reject digest for future time or initially untrusted root, then make it otherwise valid while the receipt remains. Same digest is discarded without effect; new bytes require full verification. Cached positive receipt cannot authorize expired/invalid signature or ledger-history access. |
| JS-22 UI/caller restart and run-state loss | PC-06; UI-09 | Runner creates manifest/index once; atomic fsynced reservation precedes publish. UI/caller restart recovers original deadline, Money nonce and all variants. Delete/corrupt expected state or change nonce: fail closed, no auto-initialization/republication. New stream rejects old browser context; Money reset remains memory-only. |
| JS-23 Combined Compose coexistence/readiness | KC-10 | Validate planned infra folder, stock single-node KRaft TLS/topic/group/ACL readiness and assignment. Own lock/runtime/descriptor/network/socket/port paths keep native SPIRE and live/default resources untouched. Collisions/readiness failure clean only owned resources; /healthz alone is insufficient. |

Relevant existing named unit checks include ledger concurrent retry/conflict,
Money route policy/expiry/history capacity, identity registration guards and
control bounded protocol. See their precise links in the baseline index. Their
previous passes do not count as execution of the JS matrix.
