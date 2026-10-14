# Planning and owner review record

Status: **approved**.
Approval date: 14 October 2026.
Approved revision: `9f048254a4a89d1be80c6dcf7df88c822ad2cb01`.

Owner message: “Commit push, then apply new spec”.

This file records that approval. A later commit that implements the approved
revision is not a new behavioral approval. Compose direct mTLS stays in the
approved scope.

## What needs review

Review [proposal](proposal.md), [design](design.md), the deltas under
[specs](specs/) and [tasks](tasks.md) together.

The proposed Kubernetes profile removes application mTLS and signs each
identity-bearing REST payload with the sender's X.509-SVID. Istio/Envoy keeps
outer mTLS. Verification must reject a tampered payload, a signature that does
not verify, and an obsolete signature as separate results. A previous SVID
still inside its certificate lifetime and 30-second envelope still verifies.
Compose direct mTLS, Kafka broker TLS, browser HTTPS and UDS controls stay as
they are.

That delta and short plan are the approved scope. A later scope change needs
a new review before implementation.
