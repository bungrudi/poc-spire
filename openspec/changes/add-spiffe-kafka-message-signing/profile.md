# Proposed normative profile: SPIFFE JWS Kafka commands v1

The reviewed planning body below records the scope frozen at `735b80f`.
[Actual owner approval](review.md) followed on 12 October 2026; draft/unrun
statements in that historical body refer to the time before approval and tests.
Current implementation rules and measured coverage are in the
[stable profile](../../../docs/kafka-signing-profile.md) and
[acceptance/evidence](../../../docs/kafka-signing-acceptance.md).


Status: **draft, owner approval pending**. RFC 7515/7518 provide JWS/ES256;
the fields, limits and admission policy below are this application's proposed
profile, not a new industry standard. See [references](references.md).

## Topology and authority

The existing caller executables gain separate opt-in private Kafka controls;
trusted signs using `spiffe://poc.internal/caller/trusted`. Money embeds one
consumer and authenticates the original signer itself. It calls its **same**
ledger instance used by POST `/transfer`; GET `/balances` continues to read it.
No relay or POST `/transfer/jws` is introduced. Producers cannot choose arbitrary
operations. Existing scenarios remain **Sync (HTTP/mTLS)** even though browser
jobs run in the background; added scenarios are **Async (Kafka)**. Default UDS
commands remain unchanged; the combined opt-in UI adds the fixed menu below.

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

The private caller creates an immutable descriptor once for the server-derived
request_id/scenario, before publishing. It retains the descriptor and exact
bounded record variants in a private per-run state mount, never storing an SVID
private key. Lookup/retry recovers the original creation/deadline and digests;
UI/caller-container restart within that run must not silently reset them. Corrupt
or missing state after a known attempt fails closed as unknown. Broker retry may
reuse identical valid bytes; re-signing recaptures only SVID/envelope times before
the original deadline. Caller state is bounded and removed with the combined run,
not new persistent credentials or durable Money state. See recovery rules below.

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
They have no Money transfer HTTP status; existing numeric ledger result codes are
business codes. The observer receipt query has its own HTTP status, kept distinct
from the Kafka disposition. No result topic or HTTP transfer forwarding is added.
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
`User:CN=kafka-producer` and `User:CN=kafka-unidentified` have Write/Describe on the exact topic; exact
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

## Fixed browser menu and private controls

The two existing buttons are grouped as **Sync (HTTP/mTLS)**: trusted transfer
and unidentified no-SVID/TLS rejection. Their existing POST /api/attempts body
(`caller`, `browser_key`), POST /attempt body (`request_id`), 202/background-job/
GET /api/attempts/{id} polling, Money HTTP semantics and private mounts are unchanged.
Background browser execution does not make these scenarios Async transport.

Only the enabled combined profile adds **Async (Kafka)**:

| Browser scenario | Fixed caller action | Expected Money evidence |
| --- | --- | --- |
| `kafka-trusted` | Trusted caller seals the fixed transfer and publishes it. | Valid original trusted SVID/JWS; applied/replayed shared ledger result. |
| `kafka-unidentified` | Unidentified caller observes reachable Workload API no-identity denial, then publishes one fixed unsigned probe using only its own broker-admission certificate. | Exact-record rejection for missing valid JWS; no authenticated original producer asserted. |
| `kafka-tampered` | Trusted caller signs the fixed transfer, then deterministically changes amount_minor to 200 without re-signing. | Exact-record signature rejection; certificate contents do not authenticate the author of invalid bytes. |

These are the whole menu. Wrong-ID/root/curve/deployment and other security cases
remain tests. Intentional negative probes violate the signed-command profile and
must be rejected; they are not an unsigned transfer fallback.

New browser POST /api/kafka-attempts accepts exactly `scenario`, `browser_key`
and `stream_id`, with the same bounded body/key/CSRF/Origin/Host/session/operator
checks as existing creation. stream_id must equal UI's configured current stream;
other fields/scenarios fail. It uses the unchanged issuer/subject/browser-key
RequestID derivation, not a transport namespace. A retained ID bound to another
scenario/stream conflicts. Ownership and GET /api/attempts/{id} polling remain
principal-bound, shared for the assumed public operator. All scenarios share the
existing 64-attempt, two-active-worker and eight-dispatched-attempts/minute limits.
Completed/pending repeats do not dispatch; retries retain the same key/ID.

Each selected private caller socket adds POST /attempt/kafka with exactly
`request_id`, `scenario`, `money_instance_id`, and GET /attempt/kafka/{request_id} for read-only metadata
lookup. Trusted accepts only its two Kafka scenarios; unidentified accepts only
its one. These routes retain kernel peer-UID enforcement, strict names/types,
unknown/trailing-field rejection and a 1024-byte POST bound. No caller can choose
destination/topic/amount/identity/headers through the control request. POST has a
ten-second overall deadline; lookup has a two-second deadline; existing POST
/attempt is unchanged. There is no TCP control endpoint or shared caller mount.
The instance field is a 32-hex process nonce from UI's authenticated Money metadata
read, not a selectable SPIFFE identity or an authorization grant. It is bound to
the descriptor on first creation; a different nonce on retry conflicts rather
than refreshing the descriptor. The current observer alone reads outcomes.

Each caller keeps at most 64 per-run descriptors, keyed by request_id/scenario,
and at most four exact sealed record variants per descriptor. Private caller-only
state directories are 0700/files 0600, separate from UI-mounted control volumes.
The run owner initializes a stream-bound manifest and empty bounded state once,
before first caller startup. Callers require both and never initialize/repair
missing state themselves. One atomically replaced, fsynced state/index (maximum
16 MiB per caller) contains the entries/variants; missing/corrupt state or a
manifest/stream mismatch disables Kafka controls. Only an intact initialized
store can allocate an unseen ID; a missing known entry cannot be recreated.
Persist original descriptor and exact bytes/digest, marking the variant as
potentially submitted, atomically **before** broker submission; fail without
publishing if that fails. Container restart reloads the
same state; missing/corrupt expected state is unknown/fail-closed, never automatic
new creation. New IDs at capacity receive 503 without evicting retry state. Same
ID/different scenario receives 409. Expired descriptors cannot be renewed.

Metadata exposes only original creation/deadline, scenario, configured stream/Money instance,
variant digests/expiry and broker publication state (`not_submitted`, `queued`,
`publish_unknown`), not JWS/private keys. Ambiguous submission reserves its variant
first. An ambiguous or crashed submission remains potentially published; it is
never downgraded to definitely not submitted without evidence. Retry resends
identical current bytes; re-signing creates at most one next
variant with the same immutable context before the original deadline. Unsigned
and tampered probes use their exact fixed variant rules. Publication state is
caller diagnostics; even an acknowledged `queued` record is not transfer success.

## Authenticated Money receipts

The new Money read-only resource has GET /kafka/outcomes for process/stream/
consumer-readiness metadata and GET /kafka/outcomes/{digest} for receipts. Both are enabled only with
Kafka, authorized only for the exact observer `spiffe://poc.internal/ui`, over the
same mandatory direct SPIFFE mTLS. Path digest is 64 lowercase hex; no query/body
is accepted. Existing POST /transfer and GET /balances behavior stays intact.
UI uses its own source and verifies Money's exact ID on each fresh read. It has
no broker connection/credential, admin privilege or producer signing key.

Correlation is to the complete producer-supplied Kafka record, not an untrusted
JSON identity/request_id. Define F(bytes) as signed 64-bit big-endian length followed
by exact bytes, with length -1 for null (no bytes). The record digest is lowercase
SHA-256 hex over:

```text
ASCII("poc-spire-kafka-record-v1\0") || F(UTF8(topic)) || F(key)
|| uint32_be(header_count)
|| each header in original order: F(UTF8(header.name)) || F(header.value)
|| F(value)
```

Length framing distinguishes null/empty values and header sequences. Broker
partition/offset/timestamp are not part of this digest. Both publisher and Money
compute it from exact bytes before transformations; unsupported headers/invalid
JWS still get their own digest when the record fits bounds. Oversized/unavailable
bytes cannot yield a fabricated receipt. Never index invalid-JWS rejection by a
claimed producer/request_id. Digest is a correlation identifier, not a credential
or signature; Money's authenticated response provides the evidence.

Money reserves a bounded receipt slot before any Kafka ledger/history effect.
Applied/replayed/business-rejected/conflicted results and deterministic structural/
crypto/identity/context/freshness rejections create terminal receipts; transient
unknowns do not. Ledger decision, Kafka context binding and success/business
receipt become visible atomically under the shared acceptance lock; guard failure
creates only a rejection receipt for expired certificate/message/deadline, not
ledger history. Snapshot-budget exhaustion or assignment loss is transient and
creates no terminal rejection receipt. Receipt-capacity failure
pauses with no business effect/offset advance. Existing Sync operations do not
require Kafka receipts. A record's first terminal receipt is immutable per Money
instance; later duplicates cannot overwrite a past commit with an expiry rejection.
While a terminal structural/crypto/identity/context/freshness rejection receipt
is retained, the same digest is terminally discarded without business effects,
even if time/bundle changes could otherwise make it pass. No cached receipt can
authorize a signature, signer or ledger replay: a potential positive replay still
requires current validation before history access. This rejection memory is bounded
by receipt retention and the Money lifetime, not a durable tombstone. Expiry/restart
removes that receipt guarantee. New re-signed bytes have a new digest and require
full validation plus shared business idempotency/context checks.

Receipt limit is 4096 terminal/reserved entries; completed entries are retained
180 seconds from observation and can expire afterward. Fresh entries are not
evicted to make space. Expiring receipts does not evict ledger dedup/context keys.
Money has a random per-process instance_id. Lookup returns at most 8192 bytes:
200 contains that instance, exact digest, observation/retention times, terminal
disposition/reason and authorized business result where present; 404 means no
retained receipt and includes instance_id (and consumer-ready status), not proof
of rejection/non-commit. Full/missing trust or transient processing does not
invent a terminal record. Invalid JWS has no verified producer/request_id; a
verified producer ID is supplied only after complete SVID and signature validation.
The query's HTTP status is distinct from its Kafka/business disposition.

## UI polling, unknown outcomes and restart

Each Kafka worker reads authenticated Money process/stream metadata and looks up
the initialized caller store. Known descriptors recover original instance/deadline/
variants; any instance mismatch stops automatic publication as reset/unknown.
Then it queries every potentially submitted digest before republishing. Only an
intact initialized store reporting an unseen ID allows first creation; absent/
corrupt store or missing known entry never does. The first publish passes and
retains the authenticated instance nonce; subsequent attempts recover it. All calls remain fixed,
bounded and under the shared worker/rate limits. Kafka workers poll at most once
per second per variant, with two-second query deadlines and a 30-second worker
budget; browser polling reports preparing/queued/awaiting-Money separately.
Sync worker timing and response fields remain unchanged. No background periodic
polling continues after the worker budget; explicit same-key retry can resume.

Only an exact digest-matching, authenticated Money receipt establishes terminal
evidence. Trusted success additionally requires expected original trusted ID,
request_id and fixed business result; negative cases display the correlated
Money rejection as **unattributed invalid record** plus separate caller diagnostics.
UI never concludes success from broker ack, balances, unsigned logs or echoed
claims. A committed/replayed variant resolves the logical operation; otherwise
every submitted/ambiguous variant must have a terminal no-effect receipt before
the UI declares rejection. Any outstanding variant keeps the logical outcome
unknown. Conflicting/mismatched receipt fields are an evidence failure.

Timeout, 404, receipt expiry, unavailable Money or lost control response retains
the ID and means queued/unknown, not rejected. Lookup recovers ambiguous-publish
digests even if POST's response was lost. UI-only restart renews the session and
uses the same browser key/stream and surviving caller state; it does not refresh
creation/deadline. After deadline, UI can still read retained receipts but cannot
republish/re-sign or allocate a fresh ID as retry. Missing old evidence stays
unknown. A detected Money instance change is a reset warning, not a terminal
result; automatic republishing stops. Undetected crash/restart remains the existing
memory-only limitation, never durable safety. A new combined stream rejects old
browser stream IDs; starting a new logical attempt is explicit, not an automatic
retry with a new deadline. Public sessions share the assumed operator, not privacy.

## Proposed Compose organization and lifecycle

The following directory/assets are planned, not created in this draft:

```text
infra/kafka-signing/
  compose.yaml          combined Money/UI/two callers/stock KRaft broker
  kafka.properties.tmpl TLS/controller/admission configuration
  acl-bootstrap.*       exact topic/group/DN grants using stock Kafka tooling
  broker-pki.*          standard temporary test certificate helper/config
  README.md             isolated start, readiness, retry, inspection, cleanup
```

Reuse the static Go/scratch app image build, UID/GID 1000, capability drop,
no-new-privileges and read-only application roots. Add only per-caller private
state mounts and own broker-client credentials. Money/callers join a private
Kafka network; UI does not. UI retains the Money network and only its two
read-only control mounts plus browser TLS. Kafka/admin/controller ports publish
nothing; only a separate loopback HTTPS UI port (proposed 18088) is exposed.
Its configured origin/certificate are independent of the live demo's 18087/Nginx
route. Development browser PKI is generated temporarily and explicitly trusted
by test clients; no existing public certificate/service is changed.

Use one stock Apache Kafka JVM container in combined single-node KRaft mode,
one topic/partition/group, temporary TLS/DN ACL bootstrap and a private data
volume. Kafka readiness proves TLS/hostname/admission, topic/ACL availability and
consumer assignment; UI /healthz alone is liveness. Configure broker/fetch/value
bounds, and gate the menu on the complete configured/readiness state.

Current IdentityRuntime takes the retained identity global lock even for temporary
instances, and RemoteStack uses shared remote-demo.json. The planned combined
runner must use **validated fully ephemeral** state with its own project, lock,
server port, Workload API socket and descriptor (for example
.runtime/backend-kafka-<stream>/). An explicit isolated-runtime option can be
factored from the helper; retained/default modes keep their existing lock. Never
ignore lock failures, delete someone else's descriptor or silently reuse live
CA/registrations. Native SPIRE server/agent stay host processes with the measured
permissions; Compose does not require moving the agent into Docker.

Start only this separately named combined project; wait for native SPIRE issuance,
private controls, broker/ACL readiness, Money assignment and browser HTTPS. Failure
cleanup removes only owned containers, volumes, temporary PKI/trust/descriptors,
and child processes. Caller state survives UI/caller container restart within a
run but is removed on full run cleanup. Full restart uses a fresh stream/topic,
resets shared fake Money state and invalidates old attempt context. The deployed
demo and approved retained identity/browser state must remain unchanged. All
generated examples are placeholders; .runtime is already ignored.
