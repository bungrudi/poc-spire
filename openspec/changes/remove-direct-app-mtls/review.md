# Review

Status: approved for implementation; no live deployment authorized.

Owner: Rudi, 18 October 2026, current conversation. Exact approval:

> Make the openspec, commit push, then implement quickly, dont get dragged too broad

This follows review of the signature-only design and the SPIRE/Envoy SDS proposal
with separate mesh/application SVIDs. The owner explicitly directs preparing the
updated spec, committing/pushing it, then implementing; no second approval is
required for this same scope. No message UUID was exposed in this session.

Approved artifact revision: revision 2, `revision.sha256` manifest SHA-256
`da01409b15c269c7fd003bb3893014bf6f86f6d1e300f99ed964b1014dd54e2a`. The spec commit preserves the pre-implementation artifacts;
later task checkboxes and verification notes record progress, not new approval.

Scope: remove direct app mTLS; mandatory Money payload verification; SPIRE-issued
mesh SVIDs. Retain wire formats, Kafka broker TLS, browser HTTPS and attestation.
No new controller, platform-wide migration or live rollout.
