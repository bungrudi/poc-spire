# Checked standards and implementation grounding

The reviewed planning body below records the scope frozen at `735b80f`.
[Actual owner approval](review.md) followed on 12 October 2026; draft/unrun
statements in that historical body refer to the time before approval and tests.
Current implementation rules and measured coverage are in the
[stable profile](../../../docs/kafka-signing-profile.md) and
[acceptance/evidence](../../../docs/kafka-signing-acceptance.md).


Checked 12 October 2026. These are primary sources; the v1 application choices
in [profile.md](profile.md) are proposed policy, not requirements imposed by JWS.

| Reference | Use in this proposal |
| --- | --- |
| [RFC 7515](https://www.rfc-editor.org/rfc/rfc7515.html), §§3.1,4.1.6,5,7.1 and Appendix A.3 | Standard compact JWS, protected header, x5c DER/base64 representation, exact signing input and ES256 codec vector. |
| [RFC 7518 §3.4](https://www.rfc-editor.org/rfc/rfc7518.html#section-3.4) | ES256 means ECDSA P-256/SHA-256 with a 64-byte R||S signature. It does not define our message schema, ACLs or idempotency. |
| [RFC 8725](https://www.rfc-editor.org/rfc/rfc8725.html), §§3.1,3.4,3.7,3.9–3.12 | Adapt general JOSE lessons: fixed algorithm/curve, input validation, UTF-8, explicit type/destination and separate validation rules. This envelope is not a JWT; JWT-specific claims/requirements are not asserted as normative for it. |
| [SPIFFE X.509-SVID](https://spiffe.io/docs/latest/spiffe-specs/x509-svid/) §§3–5 | URI SAN identity, leaf/usage constraints, independent bundle/path validation. No sender-supplied root becomes trusted. |
| [Working with SVIDs](https://spiffe.io/docs/latest/deploying/svids/) | Workload keys can sign data; SVIDs establish provenance as well as mTLS. Possession is conditional on the issuing attestation policy and uncompromised keys/processes. |
| [Go SPIFFE v2.6.0 verifier](https://github.com/spiffe/go-spiffe/blob/v2.6.0/svid/x509svid/verify.go) and [SVID object](https://github.com/spiffe/go-spiffe/blob/v2.6.0/svid/x509svid/svid.go) | Updating local source, paired chain/crypto.Signer object, ID/path validation at a chosen current time. Supplement digitalSignature/curve checks; no TLS verification bypass is added. |
| [SPIRE 1.14.0 agent defaults](https://github.com/spiffe/spire/blob/v1.14.0/cmd/spire-agent/cli/run/run.go#L530) and [workload key implementation](https://github.com/spiffe/spire/blob/v1.14.0/pkg/agent/workloadkey/workloadkey.go) | Pinned source sets default workload key type ECP256. Current scripts do not override it. ES256 fits that configured default; actual fixture leaf/key must still be asserted in later tests. Public pinned raw source was read; no live SVID/key was inspected. |
| [go-jose v4.1.5 release](https://github.com/go-jose/go-jose/releases/tag/v4.1.5), [go.mod](https://github.com/go-jose/go-jose/blob/v4.1.5/go.mod) | Proposed assessed upgrade from indirect 4.1.2. Release includes algorithm/curve verification and parsing fixes; its Go 1.24.0 minimum fits current Go 1.24.2. Release compatibility is not proof that our integration passes. |
| [franz-go v1.19.5](https://github.com/twmb/franz-go/releases/tag/v1.19.5), [go.mod](https://github.com/twmb/franz-go/blob/v1.19.5/go.mod) | Proposed assessed Kafka client pin; Go 1.23.8 minimum fits 1.24.2. Assess API/security/transitive dependencies before approved implementation. Disable idempotent writes explicitly and manually sequence commits. |
| [Kafka 4.0 quickstart](https://kafka.apache.org/40/getting-started/quickstart/), [TLS](https://kafka.apache.org/40/security/encryption-and-authentication-using-ssl/), [ACLs](https://kafka.apache.org/40/security/authorization-and-acls/) | Proposed stock apache/kafka:4.0.2 JVM fixture, later digest-pinned; ordinary TLS certificates/DN principals and StandardAuthorizer. This is a compatibility candidate, not a latest-version recommendation. No custom SPIFFE principal plugin is assumed. |
| [Kafka delivery design](https://kafka.apache.org/42/design/design/) | Broker delivery/offset progress is distinct from atomic external business effects. Our in-memory ledger/offsets do not constitute durable exactly-once effects. |
| [Independent Node JOSE v6.1.0](https://github.com/panva/jose/releases/tag/v6.1.0) | Proposed separately pinned test-only interoperability implementation. Verify its runtime/API compatibility in implementation; no package is installed now. |

Application contracts were inspected against revision base
**0102ec7b670812c7ff2511d1c9a4ea9b67fe0118** (the prior documentary draft).
[go.mod](../../../go.mod) pins Go 1.24.2, Go SPIFFE 2.6.0 and indirect go-jose
4.1.2; [Money](../../../internal/money/server.go) owns the HTTP policy,
[ledger](../../../internal/ledger/ledger.go) keys history by caller/request ID,
[trusted caller](../../../cmd/trusted-caller/main.go) verifies its own exact ID,
and [identity setup](../../../scripts/identity.py) enforces narrow registrations.
The [baseline index](../../../docs/spec-baseline.md) links named existing tests.
No Kafka dependency/configuration/code, receipt resource or Kafka UI/control API
exists at this base. Current [web app](../../../internal/web/app.go) accepts only
caller/browser_key on POST /api/attempts and derives IDs without transport;
[control](../../../internal/control/control.go) accepts only request_id on
POST /attempt. UI background jobs do not change the HTTP/mTLS transport.
[UI entrypoint](../../../cmd/ui/main.go) already owns an observer source and
[Compose](../../../infra/remote-demo/compose.yaml) has separate control mounts.
[Remote runner](../../../scripts/remote_demo.py) uses shared remote-demo metadata,
and IdentityRuntime locks retained runtime.lock even for ephemeral roots.
The proposed separate runtime/lock mode is implementation work, not an existing
concurrency guarantee. Future infra/kafka-signing/ assets reuse
[the scratch-image convention](../../../infra/local-ui/Dockerfile); this draft
creates no Docker config, credentials or runtime state. Another active chat is
separately editing browser assets/local-demo evidence; that work is preserved and
is not part of this proposal or its checks.

Excluded future scope: historical validation would need trustworthy timestamp
and trust-history evidence (for example Sigstore bundles or JAdES), not a claimed
issued_at. CloudEvents Verifiability and WIT/WPT are not adopted in v1.
