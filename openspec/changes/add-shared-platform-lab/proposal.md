## Why

Both PoCs currently create isolated clusters and install Istio. The approved integrated AWS lab needs one foundation owner and reusable PoC deployments.

## What Changes

Add explicit shared-cluster render/deploy/verify/remove interfaces, guarded by an environment identity; preserve standalone workflows. A new poc-platform repository owns Terraform, shared Istio, immutable composition, locking, evidence export and teardown.

## Capabilities

### New Capabilities
- `shared-cluster-deployment`: explicit target, single ownership, immutable sources, reproducible configuration and bounded cleanup.

### Modified Capabilities
None: existing standalone profiles keep their behavior.

## Impact

Deployment tooling, a new EKS overlay and runbooks; no public endpoint cutover or application authorization relaxation. AWS apply and runtime acceptance remain separately recorded.

## Approved lab region

On 15 October 2026 the owner selected Sydney (ap-southeast-2) for account 581695527661 because the current AWS project is region-restricted. Update this shared lab's location contract and preserve the current Free Plan/account setup. The observability production target remains a separate concern.

## Approved Spot compute

The owner requested "use cheap computes, spots" on 15 October 2026. The shared lab uses two Free Plan eligible m7i-flex.large Spot workers (2 vCPU / 8 GiB each), one labelled for SPIRE and one for observability. Preserve the current AWS account setup and avoid automatic On-Demand fallback.
