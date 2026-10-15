# Owner approval

Status: approved for implementation of the plan reproduced in the owner message beginning `PLEASE IMPLEMENT THIS PLAN:` on 2026-10-15, in the current KKI conversation. This is the actual human implementation request, not an agent approval.

Approved scope: shared foundation repository, explicit deployment orchestration, existing-cluster PoC modes, ownership guards, evidence and teardown. Detailed implementation artifacts instantiate that approved plan.

No additional authorization is inferred for GitHub publication, persistent access credentials, existing public-demo cutover or unrelated host changes.

## Owner-approved region adjustment — 15 October 2026

In this KKI conversation the owner answered the deployment-region question: "Use Sydney for this lab (recommended); update the OpenSpec region contract and keep the current account setup." This approves the specified Sydney lab adjustment and its implementation under the preceding request to apply the platform to AWS. It does not approve account conversion, a change to the separate production target, or GitHub publication.

## Owner-approved Spot adjustment — 15 October 2026

The owner directly instructed: "use cheap computes, spots". This authorizes replacing the lab's initial On-Demand worker with economical Spot capacity. The implementation uses compatible 4-vCPU/32-GiB x86_64 alternatives; existing identity, target, ownership and teardown controls remain. No account upgrade or GitHub publication is inferred.

## Free Plan compatibility correction — 15 October 2026

The owner chose Sydney while preserving the current Free Plan and requested cheap Spot compute. Actual AWS apply rejected the original 32-GiB candidates and IAM OIDC provider creation. The earlier candidate validation established regional availability, not Free Plan launch permission. Within the approved existing-cluster implementation, replace those candidates with two eligible 8-GiB Spot workers and use EKS Pod Identity for workload AWS access. SPIRE still binds its run to exactly one explicitly labelled node. No account upgrade, On-Demand fallback, public demo change or GitHub publication is authorized by this correction.
