# Planning and owner review record

Status: **pending**.
Approval date: none.
Approved revision: none.

This file records review only. It is not approval. An agent-written summary,
a passing `openspec validate`, or permission to commit and push does not
approve the behavior change.

## What needs review

Review [proposal](proposal.md), [design](design.md), the deltas under
[specs](specs/) and [tasks](tasks.md) together.

The proposed Kubernetes profile removes application mTLS and signs each
identity-bearing HTTP payload with the sender's X.509-SVID. Istio/Envoy keeps
outer mTLS. Compose direct mTLS, Kafka broker TLS, browser HTTPS and UDS
controls stay as they are.

No runtime code is authorized until Rudi explicitly approves this delta and
the short plan. A later scope change needs a new review before implementation.
