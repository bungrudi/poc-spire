# Design

## Context

See [proposal](proposal.md). Existing browser jobs already run in the background,
but service transfers use HTTP/mTLS and must be labeled **Sync**. The owner wants
additional **Async (Kafka)** scenarios in a Compose iteration, with consumption
and original-producer validation inside Money's existing shared ledger.

## Goals / Non-Goals

**Goal:** a small fixed menu with trustworthy Money outcomes and safe bounded retry.
**Non-goals:** Kafka-to-HTTP forwarding, another ledger, generic workflows,
durable Money recovery, custom broker identity plugins or live deployment.

## Decisions

1. Keep compact ES256/x5c JWS and current-validity checks from [profile](profile.md).
   Capture matched SVID/key and an independent updating-source bundle snapshot.
   Verify outside the ledger lock; guard time/assignment inside it. HTTP guards
   remain unchanged; no online trust-fetch subsystem is introduced.
2. Add three Kafka scenarios: trusted, unidentified/unsigned and tampered. Keep
   existing POST /attempt unchanged; new fixed publish/lookup routes use separate
   private caller sockets. Caller-only, initialized per-run state stores at most
   64 descriptors/four variants each, atomically before submission. Lost state
   fails closed; retry/restart never recreates a later original deadline.
3. Money embeds one consumer/member/partition and uses the same producer-ID/
   request_id/business history as Sync. Reserve receipt capacity before effects;
   record business result/context/receipt atomically. A bounded observer-only
   GET /kafka/outcomes metadata/receipt resource proves results over exact-Money mTLS.
   Digest covers complete submitted record fields; invalid JWS cannot authenticate
   its claimed ID. Balances, broker acks and logs cannot substitute for this proof.
4. Preserve the Sync browser API; add a separate fixed Kafka creation API and use
   existing attempt polling/ownership/CSRF limits. Kafka workers query all retained
   submitted digests for at most 30 seconds. Any authenticated applied receipt can
   settle success; rejection requires every potentially submitted variant settled.
   Missing/expired evidence stays unknown. UI restart recovers caller metadata;
   a changed stream requires an explicit new attempt.
5. Propose infra/kafka-signing/ with stock single-node KRaft Compose, TLS/DN ACL
   templates, helper assets and README, reusing scratch Go images/hardening.
   UI has no broker/admin/signing credentials. A separately named loopback stack
   uses unique ephemeral paths/locks and native SPIRE, leaving live resources alone.
   Current helpers' global lock/shared descriptors require an explicit isolated
   mode, not bypassing a busy lock. Broker idempotent writes and auto-commit stay off.

## Risks / Trade-offs

- Receipts versus retention → 4096 slots/180 seconds; reserve before effects,
  pause at capacity; expiry is unknown, not non-commit evidence.
- Queues/rotation → immutable 120-second deadline, 30-second envelopes; re-sign
  only within it. Cached trust can work during outage; rejected trust is terminal.
- Money restart → ledger/history/receipts reset; offsets cannot restore effects.
  Cross-replica safety and durable replay protection remain unclaimed.

## Implementation / Rollback Plan

After owner approval: codec/shared guards and receipts; private controls/recovery;
fixed UI/polling; isolated Compose readiness/cleanup; proposed integration and
independent JOSE checks. Reconcile specs/docs. Disable the combined profile for
rollback; preserve Sync behavior. Detailed mechanisms and tests are in
[profile](profile.md) and [acceptance](acceptance.md).
