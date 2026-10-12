# Implementation checkpoint

Owner approval and the frozen approved scope are recorded in [review](review.md).
Implementation uses the isolated managed `codex/spiffe-kafka-signing` worktree
starting from `735b80fad783bc0350740b18229f61aee8f8aa5d`. The original dirty
checkout, public demo and retained identity state are outside this worktree.

## Dependency assessment — 12 October 2026

- Go **1.24.2**, Go SPIFFE **2.6.0**, OpenSpec **1.14.1** and local Node
  **22.23.0** remain pinned. No global installation or host upgrade occurs.
- Direct **go-jose/v4 4.1.5** replaces indirect 4.1.2. Its declared minimum Go
  version is 1.24.0. The [release](https://github.com/go-jose/go-jose/releases/tag/v4.1.5)
  includes fixes for JSON depth, curve/algorithm checking and opaque signer keys.
  This implementation additionally bounds input and permits only compact ES256.
- **franz-go 1.19.5** declares Go 1.23.8. It supplies group-generation-aware manual
  commits, assignment callbacks and bounded polling; its transitive versions are
  locked in go.mod/go.sum. Baseline tests passed with the assessed dependencies.
- Independent interoperability uses development-only **panva/jose 6.1.0**, pinned
  in package.json/package-lock.json. The [security policy](https://github.com/panva/jose/security)
  supports the v6 major. It does not validate SPIFFE chains; policy tests remain
  separate. No runtime application JS dependency is introduced.
- Stock **apache/kafka:4.0.2** JVM image resolved to immutable multi-platform index
  `sha256:836cafdad9f4825880d7cf1d5a21202915ae2527bd0ef1c3600c526ed7814d1f`;
  linux/amd64 manifest is
  `sha256:29a81359d19e2fc90b38bba622244e7f85eb4c06affd975de5d1576393ef8a9d`.
  [Release notes](https://kafka.apache.org/40/getting-started/upgrade/)
  include the producer topic-routing fix. [Apache's CVE list](https://kafka.apache.org/community/cve-list/)
  identifies fixed Java-client issues and a group-DESCRIBE documentation/ACL
  advisory affecting 4.x. The fixture grants Read only to Money's exact group,
  uses the classic group protocol, INFO logs and SSL client certificates;
  producers get no group/admin grant. SASL/OAuth/Connect are not enabled.

Assessment means compatibility and review of relevant published upstream
information; it is not a claim that the entire pinned host/toolchain/image has
no vulnerabilities or that a container dependency scan has passed. This is an
isolated fake-money learning fixture, with no public deployment authorization.

## Verification status

The unchanged baseline `go test ./...` passed before and after dependency pinning.
The first core increment implements the bounded codec/verification/fixed builders,
shared Kafka acceptance/context/receipt lock, observer-only outcomes, embedded
consumer with captured-generation commits, private fixed controls and initialized
caller recovery storage. Twenty-four named new Go test functions exercise these
boundaries with local certificate, fake transport and private-state fixtures.

Formatting, all Go tests, the full race detector, Go vet and scoped whitespace
checks passed on this increment. Existing Go behavior tests passed unchanged.
The RFC 7515 Appendix A.3 ES256 vector passes separately from application policy.
No real-SPIRE/Kafka acceptance case or independent Node JOSE exchange has run yet.
The combined UI, Compose runner and final acceptance/reconciliation remain
incomplete. Tasks 2–4 stay unchecked until their full specified checks pass; only
approval/dependency tasks 1.1/1.2 are complete (2/18).

The current public service has not been rebuilt, restarted or deployed. The
separate local-demo checkout and its dirty files have not been copied or modified.

## Combined-stack checkpoint — 12 October 2026

The opt-in combined UI and isolated Compose/native-SPIRE runner are implemented.
Normal builds contain no acceptance CLI. The separate `kafkatest` build adds
host-controlled fixed cases, without a browser or UDS arbitrary-payload route.
The original checkout and public image were not modified.

The real isolated normal-build smoke passed all five Sync/Kafka scenarios with
complete readiness and owned cleanup; see `docs/evidence/kafka-smoke-current.json`.
The earlier retained failed run is explicitly failed and not acceptance evidence.
Node 22.23.0 / jose 6.1.0 bidirectional signatures, RFC vector, wrong curve and
altered bytes pass. New Go coverage includes inbound P-384/DER and wire/chain
bounds, independent bundle snapshots, expiry under an actually held ledger lock,
real producer/consumer constructor option validation, and four-variant retention.
Observer exact-Money mTLS, no-cert/wrong-server denial, UI recovery/all-variant
settlement and concurrency/rate defenses pass. Deterministic shipped-JavaScript
controls and 14 cleanup/isolation/rendered-PKI checks pass; this is not rendered
browser/visual acceptance.

Review-driven corrections separate browser and broker CAs, issue browser
serverAuth only, assert actual Compose tmpfs options, retain safe cleanup retry
state and propagate incomplete cleanup to failed reports/nonzero exit. Full
JS-01–JS-23 real/deterministic evidence mapping, crash/restart/offset scenarios,
document/spec reconciliation and independent final revision review remain open.
The checklist retains only evidence-complete tasks; smoke is not full acceptance.
