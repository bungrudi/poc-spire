# Isolated Kafka/SPIRE demo

This opt-in learning stack preserves the two **Sync (HTTP/mTLS)** cases and adds
three **Async (Kafka)** cases. Money consumes and verifies original-producer
compact ES256 JWS itself, then uses the same in-memory ledger. Broker publication
is not Money success. UI terminal evidence comes only from its own observer SVID
reading digest-correlated Money receipts over exact-Money mTLS.

Start from this repository with the existing UID/GID 1000, pinned Go runtime,
Docker/Compose and OpenSSL:

```sh
python3 scripts/kafka_demo.py --port 18088
```

The foreground runner prints its loopback HTTPS origin and temporary browser CA
path. Test clients must explicitly trust that CA; no OS/browser trust store,
public certificate, Nginx route or system service is changed. The operator is
assumed, not authenticated. Anyone who can reach this isolated loopback UI may
trigger fake transfers. Ctrl-C removes only this run's containers, image, native
SPIRE children, temporary keys/trust, descriptor and data. If cleanup reports an
error, retain the private owned runtime and retry cleanup; do not remove another
project's resources or bypass a busy lock.

The runner uses an initialized owner-only `/tmp/poc-kafka-*` directory because
Unix-domain paths in a managed worktree can exceed Linux's socket-path limit.
Its explicit isolated IdentityRuntime mode validates that private marker, owns a
separate lock/socket/port/descriptor, and preserves the default retained global
guard. SPIRE server and agent remain native UID-1000 processes. Applications use
scratch images, UID 1000, read-only roots, no capabilities or privilege escalation.
They receive no Docker/admin socket. Each caller has its own control and private
state directories. UI sees control sockets, its own Workload API and browser TLS;
it has no Kafka network, broker/admin credential or producer key.

Broker TLS PKI and browser PKI use separate temporary CAs. The browser certificate
has serverAuth only and cannot authenticate as a Kafka client. Stock Kafka 4.0.2
uses one private single-node KRaft broker/controller, TLS 1.3 and default subject-DN
principals. StandardAuthorizer defaults deny. Producers can Write/Describe only
the exact run topic; Money can Read/Describe it and Read only its exact group.
Only the separate ephemeral bootstrap job has admin credentials. There is no
plaintext listener or published broker/controller port. Idempotent-write mode
and consumer auto-commit are disabled. One Money/member/partition is supported.

Caller state is initialized once by runner-only `kafka-init`. At most 64 IDs and
four exact record variants per ID are retained in one atomically replaced,
fsynced 16-MiB-bounded file. A potential-publication marker precedes broker I/O.
Lookup exposes metadata/digests only. Missing/corrupt state disables Kafka
controls; callers never initialize or repair it. Original creation/deadline,
stream and Money nonce survive caller/UI container restart within this run.

Envelopes live at most 30 seconds, within the immutable original 120-second
deadline and current certificate validity. Retry reuses the original ID/content;
re-signing changes only certificate/signature/envelope times. A terminal rejection
for one variant cannot settle an earlier unknown publication. UI polls every
potentially submitted variant within a 30-second worker budget. Unknown/404/
expired evidence preserves the key; explicit retry never creates another ID.

Money stores 4096 receipts for 180 seconds, without evicting fresh results. Slots
are reserved before Kafka effects. The first receipt is immutable; retained
rejected digests cannot later apply. Positive replay still needs current message
verification. Receipt expiry does not remove ledger deduplication. Ledger/history/
context/receipts are process memory only: Money restart resets them, and Kafka
committed offsets cannot reconstruct effects. A detected nonce change stops UI
republishing; this is not durable replay protection or exactly-once payment safety.
A full run reset creates fresh trust/stream/topic and invalidates old browser run
context. No production payment or continuous process-integrity claim is made.

Verification assets include `scripts/kafka_smoke.py`, rendered-boundary and TLS
PKI checks, cleanup/isolation failure injection, deterministic browser controls,
Go/race/vet tests and an independent Node JOSE exchange. Full JS-01–JS-23 evidence
is recorded separately; a smoke pass or structural OpenSpec pass is not the full
acceptance suite. The `kafkatest` build tag adds fixed, host-controlled acceptance
CLI cases only and is absent from normal application builds.
