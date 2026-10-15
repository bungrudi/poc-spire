# Verification — shared platform implementation

Date: 15 October 2026 (owner timezone Asia/Jakarta).

## Local deployment corrections — later 15 October 2026 checkpoint

The initial source-check/publication sections below predate AWS provisioning.
The shared EKS SPIRE deployment based on
`c7cb63c549628acfd0e36897c36859090a673df3` subsequently reached issuer/CSI
readiness and recorded its five initial application Pods and five workload
registrations. The platform adapter then exceeded its 1,800-second limit. No
guard baseline/process/health or complete-bootstrap result existed when inspected.
These observations do not establish deployment or functional/security acceptance.

Two source defects were investigated and corrected locally:

- The outer deployment's stale target receipt overwrote nested PSAT/CSI resource
  records when applying telemetry. An isolated full-deployment regression
  reproduced the loss of exactly three records before the fix. Reloading the
  persisted receipt before the outer apply preserves all five cluster-resource
  UIDs. The regression also proves guarded cleanup retains the complete inventory
  and waits for both namespace removals before cluster-resource removal.
- Each descriptor/read/probe repeated full remote target validation. A normal
  observation cycle required 77 kubectl invocations (64 remote requests and 13
  local configuration reads). Eight verified API reads with an already-issued
  token in process memory averaged 0.7931 seconds, about 50.8 seconds for 64
  equivalent sequential requests before CLI/exec overhead. Token caching alone
  cannot establish the existing startup/freshness bounds at that measured latency.
  Explicit lifecycle/stage/cycle scopes now reuse checked targets, validate local
  configuration/kubeconfig/CA inputs on reuse, and retain fresh namespace checks
  before mutations. Independent guard reads and state probes run concurrently;
  Kubernetes writes remain serial. A healthy shared observation now needs one
  full target validation, eight object reads and four state probes.

Validation performed against the local correction:

- `python3 -B -m unittest discover -s scripts -p 'test_*.py'`: 80 passed.
- `go test ./...`, `go test -race ./...`, `go vet ./...`: passed; Go tests used
  existing passing cache where reported.
- Strict project-local OpenSpec validation and `git diff --check`: passed.

New tests cover complete exact-UID inventory, scope cleanup/expiry, context
propagation into workers, CLI/runtime cache sharing, local file drift, fresh
mutation checks, concurrent observations, replacement rejection before probes,
API unavailability, preserving earlier proved loss despite a later API/probe
failure, and the unchanged five-second probe budget. Existing monotonic
quarantine and destructive-versus-unavailable tests pass. The 10-second startup
and 15-second health-freshness bounds remain unchanged; stale health is rejected.
Initialized runs still cannot be silently retried.

At 11:24 UTC, a read-only comparison found the three unrecorded current resource
candidates had creation times within the deployment interval and matched all
expected pinned-manifest fields. The PSAT binding names only the SPIRE server
service account in the recorded owned namespace. This supports identification,
not original-UID provenance. It does not authorize receipt reconstruction or
label-based adoption/deletion. No reliable original creation receipt/audit
evidence has been recovered. A one-off exact-current-UID cleanup exception needs
its own concrete owner-reviewed recovery scope; normal removal still requires
the original receipts.

A bounded search inspected 128 local runtime files, selecting 14 ownership,
deployment-log and evidence/export candidates including the prior workload
archive. None contained the three missing UIDs. Terraform declares AWS resources
and foundation bootstrap, not these component Kubernetes objects; its versioned
state is not a recorded source of their API-assigned UIDs. Remote S3 state
versions were not downloaded or inspected.

This work did not publish source, edit cached sources or change live resources,
authentication settings, credentials, security thresholds or the public demo.
EKS timing, deployment/verify/export and integrated acceptance of the correction
remain unrun and belong to the serialized coordinator recovery.

## Executed source checks

- Python: poc-platform 19 tests; poc-observability-01 23 tests; poc-spire 64 tests — all passed (106 total).
- SPIRE: go test ./..., go test -race ./..., go vet ./... — passed; Go tests reused existing passing build/test cache where reported.
- Terraform 1.13.5: main lab and retained-state bootstrap validate; formatting passes. AWS 6.38.0 and TLS 4.1.0 providers are locked.
- Terraform mocked-provider tests: disposable lab, wrong account, wrong region, unrestricted API, unresolved inputs — 5 passed. No AWS resources created.
- Observability: seven exact chart versions rendered 53 objects. Rendered Loki/Tempo use HTTPS S3 and distinct workload roles, with no local MinIO credentials.
- Shared Istio: 40 objects rendered from checksum-verified official 1.31.1 release charts; pilot/CNI/proxy images use pinned digests. The rolling chart index lacked 1.31.1, so it is not used.
- SPIRE: complete 44-object identity/CSI/application topology rendered offline with disposable PKI; requesting images are pullable immutable references. Temporary render keys were removed.
- OpenSpec: the add-shared-platform-lab change validates strictly in each repository.
- Existing tracked-file whitespace checks pass; the two pre-existing untracked observability evidence files were preserved.

## Not established

No real AWS plan/apply, EKS deployment, telemetry acceptance, cold S3 historical read, runtime removal-isolation, teardown billing reconciliation or second clean recreation has run. Local rendering and mocked tests do not establish those results. Extended SPIRE executable/fault-injection acceptance remains open for the shared EKS profile.

The VPS has no configured AWS profile/credential files or AWS CLI. Its available RAM was below the local full-stack host contract. No existing public demo was cut over.

## Publication and desktop registration

No commits, GitHub repository creation or pushes were performed. Composition pins deliberately identify the base revisions with adapter_published=false; they refuse deployment until the reviewed adapters are published and exact new commits are recorded.

The remote Git checkout exists at /home/ubuntu/Workspace/poc-platform. Desktop registration remains manual because Computer Use blocks the Codex app and project-management tools cannot add a project root. Add this remote folder through the existing personal-vps connection.

## Sydney adjustment — 15 October 2026

The owner approved moving only this shared lab to account 581695527661 in ap-southeast-2. OpenSpec artifacts were updated before code. The platform (19), observability (23) and SPIRE (64) Python tests pass, and the five Terraform mock cases pass with Sydney accepted and Jakarta rejected. Strict OpenSpec validation passes in each repository. The observability production target remains Jakarta. AWS runtime acceptance is still pending. AWS CLI 2.37.9 is now installed in poc-platform's ignored tooling directory.
