# Proposal

## Why

The current demo authenticates live caller-to-Money connections, but has no
Kafka or message-signing path. A message may outlive its connection and pass
through a broker; the final consumer must authenticate the original producer
itself. Add a bounded learning fixture using existing JWS and SPIFFE standards,
without treating broker admission or a forwarded header as producer provenance.

This is a proposal only. The [owner review](review.md) remains pending; current
source, dependencies, deployed UI and [baseline contracts](../../specs/) stay
unchanged until the spec and short plan are explicitly approved.

## What Changes

- Add an explicit opt-in producer → Kafka → Money ingestion path. Money embeds
  the consumer, validates the original producer itself, and invokes the same
  ledger instance and fixed transfer operation as synchronous POST /transfer.
  There is no HTTP forwarding route or separate consumer ledger.
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
- Specify negative, lifecycle, crash and independent interoperability checks in
  [the proposed acceptance matrix](acceptance.md); none have run yet.

## Capabilities

### New Capabilities

- `spiffe-jws-message-signing`: bounded JWS encoding, X.509-SVID validation,
  original-producer authorization and current-validity/re-signing rules.
- `kafka-signed-command-consumption`: embedded Money ingestion, record mapping,
  shared-ledger idempotency, offset ordering and failure/restart scope.

### Modified Capabilities

- `money-transport-authorization`: scope existing HTTP policy/input rules to
  HTTP; add a separately validated Kafka ingress without changing HTTP routes.
- `fake-ledger-idempotency`: make sync/Kafka sharing and reset limits explicit.
- `identity-certificate-lifecycle`: distinguish existing fresh HTTP handshakes
  from Kafka connections; verify message credentials for every command.
- `private-caller-controls`: preserve default caller/control behavior while the
  same approved trusted executable gains an explicit Kafka signing mode.

WA-01–WA-04 and UI contracts remain compatibility constraints. Their exact
source/test/evidence links are in [the baseline index](../../../docs/spec-baseline.md).

## Impact

Proposed changes: an explicit signing mode in `cmd/trusted-caller`, opt-in Kafka
configuration/consumer startup in `cmd/money`, small signing/consumer packages,
a guarded shared-ledger entry, `infra/kafka-signing/`, and a serial isolated suite.
One approved trusted executable/registration continues to serve its exact ID;
its rebuilt digest needs an explicit narrow policy update, not duplicate grants.
No broker/admin host port or public UI trigger is added. No application receives
another workload's keys or administration sockets.

Later approved implementation will assess a direct go-jose/v4 upgrade from
current indirect 4.1.2 to 4.1.5 and pin a Go-compatible Kafka client/broker image.
The proposed candidates and checked primary references are in [references.md](references.md).
No dependency is changed by this draft. Kafka TLS/admission/ACLs are separate
from end-to-end JWS provenance; JWS supplies integrity, not confidentiality.

Non-goals: deployment or modification of the running demo, public triggers,
OIDC, durable recovery, historical certificate validation, federation, custom SPIRE
attestors, CloudEvents extensions, WIT/WPT, Sigstore/JAdES or production payments.
