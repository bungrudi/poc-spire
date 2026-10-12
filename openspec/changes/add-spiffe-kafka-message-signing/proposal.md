# Proposal

The reviewed planning body below records the scope frozen at `735b80f`.
[Actual owner approval](review.md) followed on 12 October 2026; draft/unrun
statements in that historical body refer to the time before approval and tests.
Current implementation rules and measured coverage are in the
[stable profile](../../../docs/kafka-signing-profile.md) and
[acceptance/evidence](../../../docs/kafka-signing-acceptance.md).


## Why

The current UI presents trusted and unidentified callers through background
attempt creation/polling, while their Money transfer is a synchronous HTTP/mTLS
call. Keep these scenarios labeled **Sync (HTTP/mTLS)** and add **Async (Kafka)**
scenarios alongside them. Queued commands need original-producer JWS/SVID
validation inside Money and authenticated terminal evidence for the UI.

This is a proposal only. The [owner review](review.md) remains pending; current
source, dependencies, deployed UI and [baseline contracts](../../specs/) stay
unchanged until the spec and short plan are explicitly approved.

## What Changes

- Add an explicit opt-in producer → Kafka → Money ingestion path. Money embeds
  the consumer, validates the original producer itself, and invokes the same
  ledger instance and fixed transfer operation as synchronous POST /transfer.
  There is no Kafka-to-HTTP forwarding route or separate consumer ledger.
- Use one compact JWS with ES256 and a protected X.509-SVID chain. Independently
  validate the producer certificate, exact identity and signed command context
  before effects. [The normative profile](profile.md) defines wire mapping,
  limits, timing, retry, bundle/policy snapshots and acknowledgement rules.
- Bind a fixed transfer to environment, run, topic, consumer, command type and
  logical command ID. Bound command lifetime; do not accept expired SVIDs based
  on a claimed signing time. Re-sign retries retain immutable command content.
- Share original producer ID + request_id deduplication/conflicts across sync
  and Kafka within the Money process lifetime. Money restart resets balances
  and history; broker offsets cannot restore them. No durable or exactly-once
  payment claim is introduced.
- Add three fixed UI scenarios: Kafka trusted transfer, unidentified/unsigned
  rejection and tampered-signature rejection. Preserve existing buttons, attempt
  ownership, CSRF/origin/session controls and concurrency/rate/history limits.
- Keep POST /attempt unchanged; add bounded private Kafka publish/lookup controls
  on each caller's own socket. Per-run caller state retains immutable deadlines
  and at most four submitted-record variants, including ambiguous broker acks.
- Add observer-only GET /kafka/outcomes metadata and digest receipts over Money's mandatory
  mTLS. It supplies terminal receipts for exact record bytes, including invalid
  JWS, without trusting claimed producer IDs or inferring effects from balances.
  Missing/expired evidence or Money restart remains unknown, not a rejection.
- Propose infra/kafka-signing/ for stock single-node KRaft Compose configuration,
  TLS/ACL templates, helper assets and README. The opt-in combined stack gets its
  own project/network/runtime paths, keeping native SPIRE and the live demo intact.
- Specify negative, lifecycle, crash and independent interoperability checks in
  [the proposed acceptance matrix](acceptance.md); none have run yet.

## Capabilities

### New Capabilities

- `spiffe-jws-message-signing`: bounded JWS encoding, X.509-SVID validation,
  original-producer authorization and current-validity/re-signing rules.
- `kafka-signed-command-consumption`: embedded Money ingestion, record mapping,
  shared-ledger idempotency, authenticated outcome receipts, Compose boundaries,
  offset ordering and failure/restart scope.

### Modified Capabilities

- `money-transport-authorization`: scope existing HTTP policy/input rules to
  HTTP; preserve the transfer/balance routes and add a read-only observer receipt
  route alongside separately validated Kafka ingestion.
- `fake-ledger-idempotency`: make sync/Kafka sharing and reset limits explicit.
- `identity-certificate-lifecycle`: distinguish existing fresh HTTP handshakes
  from Kafka connections; verify message credentials for every command.
- `private-caller-controls`: preserve default caller/control behavior while the
  same approved executables gain separate fixed Kafka publish/lookup controls.
- `https-demo-ui`: preserve existing Sync scenarios/background polling and add
  the fixed Async menu, authenticated Money outcome display and stable recovery.

WA-01–WA-04 and unchanged UI security contracts remain compatibility constraints. Their exact
source/test/evidence links are in [the baseline index](../../../docs/spec-baseline.md).

## Impact

Proposed changes: private fixed Kafka controls in both caller binaries, opt-in
consumer/receipt startup in cmd/money, a fixed UI menu and outcome polling,
small signing/consumer packages, a guarded shared ledger/receipt entry,
infra/kafka-signing/ and a serial isolated suite.
One approved trusted executable/registration continues to serve its exact ID;
its rebuilt digest needs an explicit narrow policy update, not duplicate grants.
Only the combined stack's loopback browser HTTPS port is published. UI receives
neither producer private keys nor Kafka/admin credentials/network access. Added
scenario buttons are part of this opt-in learning UI, not permission to change
the currently deployed public demo.

Later approved implementation will assess a direct go-jose/v4 upgrade from
current indirect 4.1.2 to 4.1.5 and pin a Go-compatible Kafka client/broker image.
The proposed candidates and checked primary references are in [references.md](references.md).
No dependency is changed by this draft. Kafka TLS/admission/ACLs are separate
from end-to-end JWS provenance; JWS supplies integrity, not confidentiality.

Non-goals: deployment or modification of the running demo, arbitrary UI workflows,
OIDC, durable recovery, historical certificate validation, federation, custom SPIRE
attestors, CloudEvents extensions, WIT/WPT, Sigstore/JAdES or production payments.
