# Verification — 18 October 2026

Spec/design approval and publication precede implementation at `5a3f878`.
This report belongs to the implementation commit containing it;
`verified-source.sha256` identifies the key checked sources. The revision-2
approval manifest remains the historical proposal snapshot in the spec commit.
MS-01 was shortened for strict validation by moving its URI example into the
scenario; behavior did not change. Task checkboxes record later progress.

| Dimension | Result |
| --- | --- |
| Completeness | 10/11 tasks complete; runtime acceptance task 4.2 remains open. All 19 added/modified requirements have source mappings; MT-01 rename is implemented. |
| Correctness | Deterministic application checks pass. Actual SPIRE-issued workload HTTP and mesh issuer/rotation are not verified in this environment. |
| Coherence | One signed app protocol, separate proxy/app credentials, existing JWS formats and dependencies, no live rollout. |

## Checks run

Use `source /workspace/.onboarding/env.sh` for the pinned Go 1.24.2 tooling.
Local socket tests required the execution environment's networking permission.

| Command/check | Observed result |
| --- | --- |
| `POC_JOSE_INTEROP=1 go test -count=1 ./...` | PASS, including independent Kafka JOSE interoperability. |
| `go test -tags kafkatest ./...` | PASS; migrated development/security fixtures compile and tests pass. |
| `go test -race ./...` | PASS. |
| `go vet ./...` | PASS. |
| `go test -race ./internal/caller ./internal/money ./internal/controladapter ./internal/httpjws ./internal/ledger` | PASS after adding real HTTP response-loss/same-ID recovery and local-expiry tests. |
| `PYTHONPATH=scripts python3 -m unittest discover -s scripts -p 'test_*.py'` | PASS, 83 tests, including proxy grant/CSI/shared-trust-domain boundaries. |
| `npm run spec -- validate remove-direct-app-mtls --strict --no-interactive` | PASS. |
| `npm run spec -- validate --specs --strict --no-interactive` | PASS, all 11 current capabilities. |
| Pinned Istio 1.31.1 `manifest generate` with `kubernetes_mesh.operator()` | PASS. Source inspection confirms custom SDS socket fails closed, proxy metadata injection and the existing workload-socket mount. |
| Offline `istioctl kube-inject` | Not verified: the command attempted Kubernetes webhook discovery; no local API server was available. Stopped the owned check. |
| Source audit | No application `ServerTLS`, `ClientTLS`, `NoCertificateTLS` helper or selectable workload-mTLS mode remains. Browser HTTPS and Kafka TLS remain. |

## Requirement/scenario coverage

- SP/MT/IC/PC/UI requirements map to `internal/httpjws`, signed Money,
  caller, observer and adapter paths. Existing cryptographic tests cover tamper,
  wrong keys/IDs, forged chains, missing signatures, clock limits and an older
  still-current SVID. Money tests cover exact route policy and signed responses.
- SP-06/MT-04 map to `Ledger.TransferCurrent` and
  `TestHTTPSignatureExpiresWhileWaitingWithoutHistoryInsertion`; history and
  mutation share the freshness guard's lock. HTTP/Kafka idempotency is retained.
- PC-04 maps to `TestLostSignedResponseStaysUnknownAndSameIDRecoveryAppliesOnce`:
  a real HTTP connection closes after the signed Money handler commits, no
  verified response is invented, and retry produces one total effect.
- MS-01/MS-05 map to `kubernetes_mesh_identity.py`, the trust-domain policy
  update and proxy registration before readiness in both bootstrap paths.
  Native-sidecar admin registration uses init-container status. Proxy selectors
  cannot match the UID/container of the application signing process.
  The existing Unix digest limit is bounded at 256 MiB to accommodate Envoy;
  pinned SPIRE 1.14.0 rejects the entire Unix attestation on a digest-size error.
  Application registrations still require their exact executable digest.
- Six capability deltas are synced. RFC, README, runbooks, agent invariants,
  TODO and the existing Pages specification are reconciled. Historical reports
  retain their old source/meaning. The old TLS-record proxy runner fails early
  with its retirement notice; signed response-loss coverage replaces it.

## Runtime limit and remaining task

The fresh temporary SPIRE 1.14.0/Compose smoke fixture bootstrapped its issuer
but stopped at observer Workload API identity acquisition (`identity_unavailable`),
before signed HTTP could be exercised. `/tmp` and project-local temporary-root
attempts had the same outcome. This is **not an identity-denial or signature
acceptance pass**. No protocol verdict or root cause is claimed.
`runtime-smoke.json` retains the sanitized outcome and successful Compose cleanup;
owned temporary credentials/processes/image were removed. Retained trust and live
deployments were not used or reset.

**Open acceptance issue:** complete task 4.2 in a compatible disposable runtime,
observe actual SPIRE issuance, proxy SDS issuer/rotation and the S01–S11 flows.
Do not archive or claim deployment acceptance from renderer/unit results.
No source behavior was broadened to make the failed fixture pass.
