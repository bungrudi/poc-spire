# Verification — shared platform implementation

Date: 15 October 2026 (owner timezone Asia/Jakarta).

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
