# Proposed normative profile: SPIFFE JWS Kafka commands v1

Status: **draft, owner approval pending**. RFC 7515/7518 provide JWS/ES256;
the fields, limits and admission policy below are this application's proposed
profile, not a new industry standard. See [references](references.md).

## Topology and authority

The existing trusted-caller executable gains an explicit one-shot Kafka
producer mode using `spiffe://poc.internal/caller/trusted`. Money embeds one
consumer and authenticates the original signer itself. It calls its **same**
ledger instance used by POST `/transfer`; GET `/balances` continues to read it.
No relay or POST `/transfer/jws` is introduced. The producer cannot choose
arbitrary operations. Default UDS callers and the public assumed-operator UI
continue their existing behavior; Kafka requires complete opt-in configuration.

The first implementation is tested only in an isolated Compose fixture using
the measured UID-1000/native-agent layout. One binary/image/deployment digest
conjunction grants the trusted ID; its Kafka mode does not create a duplicate
identity grant or weaken attestation. Neither signatures nor SPIFFE authenticate
the browser human or prove continuous process integrity.

## Wire form and bounds

Kafka value is exactly the ASCII compact JWS `protected.payload.signature`;
all three segments are nonempty, unpadded canonical base64url. One signature,
no JSON serialization, detached payload, `b64=false`, compression or encryption.
The signing input uses the received encoded segments exactly; never verify a
re-serialized JSON object. ES256's decoded signature is 64-byte R||S, not ASN.1 DER.
These encodings follow [RFC 7515](https://www.rfc-editor.org/rfc/rfc7515.html#section-3.1)
and [RFC 7518](https://www.rfc-editor.org/rfc/rfc7518.html#section-3.4).

Protected header has exactly `alg`, `typ`, `x5c`:

```json
{"alg":"ES256","typ":"application/vnd.poc-spire.kafka-command.v1+jws","x5c":["BASE64_DER_LEAF","BASE64_DER_INTERMEDIATE_IF_NEEDED"]}
```

`typ` is a private profile identifier, not a registered media type or a JWT.
`x5c` uses standard padded base64 DER, leaf first, followed by its ordered issuer
chain; a supplied root is never installed as trust. Unknown headers, including
`crit`, `jku`, `jwk`, `kid`, `x5u`, and algorithm substitutions, are rejected.
There is no sender-directed key/bundle/OCSP/AIA fetch. [RFC 7515 §4.1.6](https://www.rfc-editor.org/rfc/rfc7515.html#section-4.1.6)
defines the chain representation; v1 fixes the allowed header set.

Bounds before decoding/parsing: value 32768 bytes; decoded protected header
16384 bytes; payload 4096 bytes; x5c 1–4 certificates, each DER at most 2048
bytes, aggregate at most 8192 bytes. Kafka key at most 128 bytes, headers empty.
Decoded JSON must be UTF-8, depth at most four, with no duplicate member names
(including escape-equivalent names), unknown fields, trailing values, invalid
Unicode, nulls or wrong types. Numbers below are integers in decimal notation,
without fractions/exponents; timestamps are 0–4102444800. Signature/parser errors
are terminal rejection, never unsigned fallback. Kafka fetch/batch buffers are
also bounded; broker record limits complement application bounds.

## Signed context and Kafka mapping

The payload is one JSON object with exactly `context`, `transfer`, `issued_at`,
`expires_at`. This illustrative payload is unsigned and is not test evidence:

```json
{
  "context": {
    "version": 1,
    "environment": "poc-spire-isolated",
    "stream_id": "0123456789abcdef0123456789abcdef",
    "topic": "poc.spire.commands.0123456789abcdef0123456789abcdef",
    "audience": "spiffe://poc.internal/money",
    "type": "fake.transfer.v1",
    "created_at": 1791760000,
    "deadline": 1791760120
  },
  "transfer": {
    "request_id": "logical-transfer-1",
    "from": "demo-a", "to": "demo-b",
    "amount_minor": 100, "currency": "USD"
  },
  "issued_at": 1791760000,
  "expires_at": 1791760030
}
```

Money's immutable startup policy fixes environment, stream_id, exact topic,
audience, version, type and allowed producer ID. Fixture stream IDs are random
128-bit lowercase hex and topic suffixes match them; another environment/run
requires different configuration, never a value from the record. Actual Kafka
topic must equal signed topic; record key is exactly UTF-8 `transfer.request_id`
(`[A-Za-z0-9_-]{1,128}`). No Kafka header conveys identity or security policy;
v1 rejects all headers. Partition, offset, broker timestamp and client ID are
unsigned delivery metadata and never authorize effects. Producers cannot sign
broker-assigned offsets in advance. The consumer subscribes to one configured
topic/partition; copying to another destination fails context verification.

## Certificate, signature and authorization

Signer captures one immutable SDK SVID object, including chain and crypto.Signer;
it checks current leaf validity, leaf/private public-key equality, exact trusted ID and P-256 key before
signing. Rotation switches the whole object, never key and certificate separately.
No keys are exported. ES256 is the proposed fixed algorithm: a different observed
SVID key type fails closed and requires a reviewed profile change, not negotiation.

Money obtains `poc.internal` trust independently through its protected Workload
API. It validates X.509-SVID path/leaf rules at **current consumer time**, derives
exactly one SPIFFE URI SAN, checks `digitalSignature`, non-CA constraints and
P-256 key, then verifies JWS with that leaf's public key. Its allowed ID is exactly
`spiffe://poc.internal/caller/trusted` for this environment/topic/type. A valid
other ID is unauthorized even if Kafka admitted it. Untrusted roots or claimed
payload identity cannot confer permission. SPIFFE's [X.509-SVID standard](https://spiffe.io/docs/latest/spiffe-specs/x509-svid/)
and the [pinned SDK verifier](https://github.com/spiffe/go-spiffe/blob/v2.6.0/svid/x509svid/verify.go)
are the validation basis; required usage/curve checks supplement the SDK.

## Current-validity window and retries

All times are whole UTC seconds. Original creation and deadline are immutable:
`0 < deadline-created_at <= 120`. Each signing attempt sets fresh issued_at and
`expires_at = min(issued_at+30, deadline, floor(leaf.NotAfter)-5)`; it refuses to
publish unless more than five seconds remain. It waits at most ten seconds for
a suitable rotated SVID, then reports unavailable; it never invents a new ID.

Money requires `created_at <= issued_at < expires_at <= deadline`,
`0 < expires_at-issued_at <= 30`, issued_at within the signing leaf's validity,
`issued_at <= now+5`, `now < expires_at`, `now < deadline` and current valid
certificate paths. Five seconds permits a slightly future issued_at only:
there is **no** expiry or X.509 validity grace. Clocks are assumed synchronized
within five seconds; known clock-health failure is infrastructure failure.
Tests inject clock readings rather than changing host time. A claimed signing time cannot rescue an
expired certificate or a removed trust root. Queuing past these limits rejects.

Retries retain original context/creation/deadline and the exact semantic transfer
fields. The producer may resend identical JWS while valid or re-sign using its
current SVID and new issued_at/expires_at within the original deadline. Signature,
certificate and envelope times are not ledger keys. Unknown broker acknowledgement
or Money outcome does not authorize a new request_id. A caller may intentionally
start a new logical operation with a new ID; no exactly-once guarantee follows.

The one-shot producer consumes an explicit immutable command descriptor containing
the original context and transfer fields. The local fixture harness creates it
once, before the first publish, and supplies the same descriptor on every invocation
through a private read-only temporary mount. It contains no credential. Producer
mode does not silently generate a new ID, created_at or deadline; it rejects a
missing/expired descriptor. Changing logical content is an explicitly new command,
not retry. Broker retry may reuse valid envelope bytes; re-signing recaptures only
the SVID and envelope times. No new retained credentials/state are required.

## Validation-to-commit boundary

Policy is immutable during one Money lifetime; changing it requires explicit
restart/reset review. Each validation attempt captures one immutable local bundle
snapshot from Money's existing updating Go SPIFFE source. No per-command online
fetch, custom publication/generation subsystem or record-supplied trust is needed.
Available cached trust can validate a current command during SPIRE outage; missing
trust is transient, but a certificate rejected by available accepted trust is
terminal. Later bundle updates govern subsequent attempts, not retroactive
revocation of a completed decision or an already captured validation snapshot.

Validate the bounded envelope, complete path/signature, context and exact ID
outside the ledger lock, with a two-second overall local verification budget.
Immediately before Kafka history lookup/mutation, under the shared ledger lock,
recheck current time, all certificate validity intervals in the validated path,
message/deadline validity, maximum two-second elapsed snapshot-to-acceptance
budget and consumer assignment ownership. An expired result
cannot commit even after lock contention. A captured policy/bundle snapshot
governs this bounded attempt; remote or later local trust changes are not claimed
instantaneous. No cryptography or network fetch occurs while holding the ledger
lock. The sync HTTP guard remains its existing pre-authorization/post-body checks.

## Shared effects and acknowledgement

Money invokes its existing fixed integer ledger with authenticated **original
producer ID** and the parsed transfer fields. Key is `(producer ID, request_id)`;
canonical business equality is the existing typed ledger.Request fields, not JWS
bytes, certificate, envelope timestamps, Kafka offset or an invented relay ID.
Context is revalidated on every delivery but is not a new sync idempotency key.
The first Kafka terminal ledger decision also binds its immutable context to that
key in bounded Money memory, atomically with the ledger decision. Later Kafka
deliveries with changed context/creation/deadline conflict; only envelope times
and the signing certificate/signature may change. A prior sync result has no Kafka
context until its first valid Kafka copy binds one and replays that result.
Bindings have at most the ledger's 10000 retained keys, use no unsafe eviction,
and are not inserted for pre-validation or transient/capacity failures.
Thus sync→Kafka, Kafka→sync and valid re-sign retries replay the same stored result;
changed business content conflicts before another mutation. Fresh invalid/expired
messages cannot retrieve a stored result by bypassing verification.

One Money instance, one consumer group member, one partition and one in-flight
record; no shared-ledger safety across replicas is claimed. Auto-commit is
disabled. Pausing processing preserves heartbeats/polls within the configured
session/max-poll interval. Assignment callbacks fence commit work on revocation;
an inactive/lost assignment stops processing and offset commits. Reassignment
resumes from the broker-committed offset and revalidates redelivery. An additional
Money replica or group member is outside v1 and must fail fixture validation.

A successful mutation,
authenticated stable replay, business rejection/conflict or deterministic invalid
message is a terminal disposition; then commit exactly that partition's next
offset using current group generation/ownership. Capacity exhaustion, unavailable
trust material, Kafka/network outage or an
unclassifiable outcome is transient: pause, retain record and request_id, do not
advance offsets. Retry without new command content. Commit-ack uncertainty retries
the acknowledgement of the already decided record and blocks later records;
redelivery while Money stays alive revalidates and uses ledger deduplication.
Transport polls/fetches/commits have bounded deadlines and shutdown cancellation.
An acknowledged produce means broker publication, not Money commit. Kafka
decisions are reported as sanitized Money audit outcomes with `ingress: kafka`,
authenticated original producer ID, request_id, stage and ledger result/replay.
They have no Money HTTP status; existing numeric ledger result codes may be
labeled business codes only. No result topic or new browser endpoint is added.
Producer idempotent-write mode is explicitly disabled and acknowledgements use
all in-sync replicas (one in this fixture). No cluster IdempotentWrite or
transactional-ID ACL is assumed or required; uncertain produce outcomes may
duplicate records, resolved by Money's business idempotency during its lifetime.

All effects/history/context bindings stay in memory. Money crash/restart clears them; committed
Kafka offsets do not reconstruct balances, and unacknowledged or resent still-valid
commands can apply again in the new Money lifetime. A fresh fixture uses fresh
trust/topic/stream ID, but replay protection across a reused configured stream is
**not durable**. No database, transactional outbox or atomic offset+ledger storage
is claimed. This known limit must be demonstrated in crash tests and owner review.

## Broker admission and evidence

Use stock Kafka TLS 1.3 client-certificate admission with a temporary fixture CA,
separate broker/producer/Money client certificates and normal hostname verification
for `kafka`. These are conventional test-PKI fixtures, not a custom Kafka plugin
or persistent CA service; each application mounts only its own broker credential.
Broker PKI is independent of SPIRE. Preserve default DN principals: exact
`User:CN=kafka-producer` has Write/Describe on the exact topic; exact
`User:CN=kafka-money` has Read/Describe and Read on the one configured group.
StandardAuthorizer defaults deny; only isolated admin tooling creates topic/ACLs
and has admin privileges. No app receives admin keys, no plaintext listener is
reachable by apps, and broker/controller ports remain unpublished. Negative
fixtures get narrowly scoped temporary admission to prove JWS rejection beyond ACLs.

Kafka's [default TLS principal is a subject DN](https://kafka.apache.org/40/security/authorization-and-acls/),
not a SPIFFE URI SAN. Broker admission identity never substitutes for the signer.
JWS does not encrypt its payload or stop a broker withholding/reordering records.
Logs/evidence contain case IDs, verified IDs after validation, reason codes,
digests, versions and ledger/offset observations; no complete JWS, SVID chain,
private key, cookie, token or broker credential is exported. Temporary broker
records contain the signed public chain by design and are destroyed on cleanup.
