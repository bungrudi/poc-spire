# Design

## Context

See [proposal](proposal.md). Current Money authenticates HTTP in Go and owns one
mutex-protected integer ledger/history. The trusted binary has one narrow
executable/deployment grant. There is no Kafka path today. The owner selected
an embedded Money consumer, preserving both existing HTTP routes and public UI.

## Goals / Non-Goals

**Goal:** Money verifies the original workload's signed queued command and calls
the existing transfer operation, sharing sync/Kafka deduplication.
**Non-goals:** relay endpoints, another ledger, public triggers, durable recovery,
archival signatures, broker SPIFFE plugins or deployment. See [profile](profile.md).

## Decisions

1. Use compact single-signature ES256 JWS with protected x5c/type and exact bytes.
   A JWT is unnecessary; a custom signature format adds interoperability risk.
   Capture one Go SPIFFE SVID/key object. Pinned SPIRE defaults to P-256; validate
   the actual leaf/key and never negotiate an alternative algorithm silently.
2. Add an explicit one-shot Kafka mode to the existing trusted executable, using
   the same trusted ID and measured selector conjunction. A second signing
   executable/grant could weaken duplicate-policy boundaries. The harness supplies
   one immutable descriptor on every retry; re-signing refreshes only credentials
   and envelope times within its original deadline.
3. Embed consumption in Money, opt-in only: one instance, member, partition and
   in-flight record. Standard temporary broker TLS/DN ACLs provide admission;
   Money separately derives/authorizes the SVID signer. Disable producer idempotent
   writes and consumer auto-commit; application dedup handles duplicate records.
4. Add a small signing/verification package and an isolated Kafka adapter. Verify
   full X.509/JWS against a captured updating-source bundle outside the ledger
   lock. A Kafka-only guarded ledger entry checks current validity/time and
   assignment under that lock; the existing HTTP guard/response behavior remains.
5. Share producer-ID/request_id/business comparisons with the existing ledger.
   Bind immutable Kafka context on its first terminal ledger decision, in bounded
   memory atomically with that decision. Offset acknowledgement follows the known
   outcome; transient failures pause with heartbeats, revoked assignments fence
   commits, and unknown outcomes preserve the original ID.

## Risks / Trade-offs

- In-memory state versus persisted offsets → explicitly demonstrate reset and
  possible reapplication after restart; no durable exactly-once claim.
- Queues versus short SVID validity → 30-second envelopes, 120-second original
  deadline, no expiry grace; producer can re-sign within that deadline.
- Cached trust versus revocation → bounded snapshot verification; updated bundles
  govern subsequent attempts, not instant revocation. Missing trust is transient;
  available-trust certificate/signature rejection is terminal.
- Opt-in startup failure → incomplete Kafka settings fail startup; established
  runtime Kafka faults pause ingestion while synchronous Money remains available.

## Implementation / Rollback Plan

After explicit approval: assess/pin JOSE and Kafka dependencies; build codec and
guarded shared effects with deterministic tests; add producer/embedded consumer;
prove [acceptance cases](acceptance.md) in isolated real-SPIRE/Kafka fixtures and
independent interoperability; reconcile docs/specs. Default Kafka remains disabled.
Rollback disables ingestion without weakening HTTP mTLS. No live deployment is
included. Relevant source and version facts are in [references](references.md).
