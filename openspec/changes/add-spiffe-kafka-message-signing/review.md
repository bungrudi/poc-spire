# Owner review

Status: **pending**. Implementation is **not approved**.

Draft base: `4b0077437cc17381fb14c918fff6a779e518c994`.
Draft date: 12 October 2026, Asia/Jakarta.

The parent supplied the owner's drafting request for JWS + SPIFFE Kafka signing
and subsequent explicit scope choice: **embed the Kafka consumer in Money and
validate there**, preserving synchronous routes and using the same ledger.
Those instructions authorize this draft, not approval of its final artifacts.
No approval message/reference or approved artifact revision is invented here.

Review together: [proposal](proposal.md), [design](design.md),
[normative profile](profile.md), [delta specs](specs/),
[acceptance](acceptance.md), [references](references.md), [tasks](tasks.md).

Proposed policy choices requiring review: ES256/x5c compact envelope; immutable
descriptor and bounded 30-second/120-second timing; one Money instance/partition;
stock broker TLS/DN ACLs; shared cross-ingress keys plus bounded Kafka context
bindings; cached local trust snapshot boundary; memory-only crash/restart scope.

After an actual approval, record owner message/date and a SHA-256 manifest of
the approved proposal, deltas, design and normative appendix. Material changes
require renewed review before dependent implementation. Independent agent review,
CLI planning completion and commit/push permission do not satisfy this gate.

Draft-only verification is recorded in [validation.md](validation.md).

Independent agent draft review: **passed**, 12 October 2026. The parent
orchestrator in chat `01a12b31-9ead-7e92-a024-23860cfe5d8f` independently checked
strict validation, modified-scenario preservation, links, scope and key semantics.
This is not owner approval, acceptance of implementation or execution of security
tests. Publishing this pending proposal does not authorize code/apply.
