# Current feature baseline and verification index

## Baseline identity and review status

- Baseline source revision: **cc4ea6beef8138e16fb320449b7096b03ca09711**
  (`Publish HTTPS SPIRE demo and illustrated service guide`); main was clean
  before this adoption.
- Adoption date: **11 October 2026** (owner timezone Asia/Jakarta).
- Authorization: Rudi directly requested applying OpenSpec through the
  orchestrator and granted standing commit/push permission for authorized
  poc-spire project work to the existing origin. See the original owner message
  reference in [AGENTS.md](../AGENTS.md). This adoption is finalized after
  independent review; runtime/deployment/access changes remain outside its scope.
- Review status: **source-derived documentary baseline; independent verification
  passed with all findings resolved; owner acceptance of baseline correctness
  pending**. Existing recorded
  implementation checks do not imply a new full acceptance run or owner approval
  of a future behavior change.
- Actual development tools: OpenSpec **1.14.1**, Node **22.23.0**, npm **10.9.8**.
  Go is pinned to **1.24.2**, SPIRE to **1.14.0**, Go SPIFFE SDK to **2.6.0**.
  Docker/Compose/Linux versions belong to each historical fixture's evidence;
  this adoption does not re-measure their runtime behavior.
- Setup: [workflow and reproducible tooling](spec-workflow.md). No bulk RFC
  conversion or synthetic feature apply/archive was performed.

The [six capability contracts](../openspec/specs/) describe the implemented
profile at useful behavioral granularity. [RFC 0001](rfcs/0001-process-identity.md)
retains architectural rationale and T01–T23; [WORKSPACE.md](../WORKSPACE.md)
retains R1–R8. These indexes are references, not separate competing contracts.
Each stable requirement ID below appears in its capability's heading.

## Coverage legend

**Recorded** means existing sanitized evidence at the baseline revision reports
a pass. It has been inspected, not rerun during adoption. **Deterministic** means
a named unit check is available and covered by the adoption's local test invocation;
it does not establish real SPIRE handshake/deployment behavior. **Source** means
the implementation/configuration was inspected without a dedicated saved
integration assertion for every scenario. Go test invocations reused cached
passing results because application sources were unchanged; the seven Python
unit tests executed again. No missing check counts as a pass.

Several original JSON reports have no timestamp or source commit. Their stored
`status: passed` is historical evidence, not proof that the full current checkout
or future environment passed. Entries below identify exact cases instead of
using one report's top-level pass as blanket acceptance.

## Workload attestation

Contract: [workload-attestation](../openspec/specs/workload-attestation/spec.md).

| Requirement / original IDs | Source | Named checks and recorded evidence |
| --- | --- | --- |
| WA-01 / R1,R5,R7; T04,T05 | [attestation.py](../scripts/attestation.py), [identity.py](../scripts/identity.py) | Recorded [attestation-latest](evidence/attestation-latest.json): `approved`, `wrong-executable-same-deployment`, `approved-wrong-deployment`. [Feasibility](attestation-feasibility.md) verifies pinned selector syntax and conjunction. |
| WA-02 / R5,R7; T21 | [identity.py](../scripts/identity.py), [backend_extended_suite.py](../scripts/backend_extended_suite.py) | Deterministic [test_identity.py](../scripts/test_identity.py): `test_broader_parent_duplicate_and_ttl_policy_cannot_be_silently_reused`, `test_exact_existing_entry_is_reused_and_not_owned_by_test_cleanup`; recorded [attestation-latest](evidence/attestation-latest.json): `rebuilt-executable-old-registration`; [backend-extended](evidence/backend-extended.json): `T21-explicit-digest-update-and-no-silent-lifetime-reuse`. |
| WA-03 / R1,R4,R5; T03,T20 | [remote Compose](../infra/remote-demo/compose.yaml), [caller_suite.py](../scripts/caller_suite.py) | Recorded [callers-control](evidence/callers-control.json) runtime users/ports and `unidentified-caller`; [ui-demo](evidence/ui-demo.json) runtime mounts/UID; [public-demo](evidence/public-demo.json) `runtime_boundaries`. |
| WA-04 / R5,R7,R8; T04,T05,T18,T20 | [attestation.py](../scripts/attestation.py), [attestation fixture](../cmd/attestation-fixture/main.go) | Deterministic [test_attestation.py](../scripts/test_attestation.py): `test_timeout_does_not_disclose_bootstrap_arguments`, `test_missing_or_extra_selector_cannot_count_as_executable_proof`; [main_test.go](../cmd/attestation-fixture/main_test.go): `TestDenialDoesNotAcceptInfrastructureFailures`. Recorded selectors/cleanup in [attestation-latest](evidence/attestation-latest.json). |

## Money transport and authorization

Contract: [money-transport-authorization](../openspec/specs/money-transport-authorization/spec.md).

| Requirement / original IDs | Source | Named checks and recorded evidence |
| --- | --- | --- |
| MT-01 / R3,R7; T01,T02,T06,T07 | [identity.go](../internal/identity/identity.go), [server.go](../internal/money/server.go), [Money entrypoint](../cmd/money/main.go) | Recorded [money-security](evidence/money-security.json): `no-cert-forged-identity-headers`, `self-signed-allowed-uri`; [ui-demo](evidence/ui-demo.json): `real-trusted-transfer-browser-repeat`, `real-unidentified-no-SVID-TLS-rejection`. |
| MT-02 / R3,R7; T08 | [server.go](../internal/money/server.go) | Deterministic [server_test.go](../internal/money/server_test.go): `TestRoutePolicyBeforeParsingOrLedger`, `TestBoundedInputAndObserverPolicy`; recorded [money-security](evidence/money-security.json): `valid-wrong-id`, `observer-cannot-transfer`, verified-peer audit. |
| MT-03 / R3,R7; T09 | [identity.go](../internal/identity/identity.go), [caller.go](../internal/caller/caller.go), [UI entrypoint](../cmd/ui/main.go) | Recorded [money-security](evidence/money-security.json): `wrong-server-identity`; [ui-demo](evidence/ui-demo.json) actual caller/server verification and observer balance reads. |
| MT-04 / R2,R3,R7; T15,T17,T23 | [server.go](../internal/money/server.go) | Deterministic [server_test.go](../internal/money/server_test.go): `TestBoundedInputAndObserverPolicy`, `TestCredentialExpiryCannotReachLedger`; recorded [money-security](evidence/money-security.json): `rotation-expiry-and-connection-reuse`. |

## Fake ledger and idempotency

Contract: [fake-ledger-idempotency](../openspec/specs/fake-ledger-idempotency/spec.md).

| Requirement / original IDs | Source | Named checks and recorded evidence |
| --- | --- | --- |
| FL-01 / R2; T01,T15 | [ledger.go](../internal/ledger/ledger.go) | Deterministic [ledger_test.go](../internal/ledger/ledger_test.go): `TestIndependentTransfersConserveMoney`, `TestBusinessRejectionsAreAtomicAndStable`; recorded [ui-demo](evidence/ui-demo.json): `real-trusted-transfer-browser-repeat`. Overflow/funds boundaries are unit evidence. |
| FL-02 / R2,R7; T13,T14,T16 | [ledger.go](../internal/ledger/ledger.go), [caller.go](../internal/caller/caller.go) | Deterministic [ledger_test.go](../internal/ledger/ledger_test.go): `TestConcurrentRetryAndLostResponse`, `TestConflictIdentityBindingAndCapacity`; recorded [money-security](evidence/money-security.json): `concurrent-duplicates`, `lost-response-conflict-invalid-input`; [backend-lifecycle](evidence/backend-lifecycle.json): `T16-real-post-commit-connection-drop`; [backend-proxy](evidence/backend-proxy.json): `T16-opaque-encrypted-response-loss-after-commit`. |
| FL-03 / R2,R7; T23 | [ledger.go](../internal/ledger/ledger.go), [server.go](../internal/money/server.go) | Deterministic [ledger_test.go](../internal/ledger/ledger_test.go): `TestConflictIdentityBindingAndCapacity`; [server_test.go](../internal/money/server_test.go): `TestHTTPHistoryCapacityCannotForgetOrAuthorizeKeys`. |
| FL-04 / R2; T22 | [Money entrypoint](../cmd/money/main.go), [remote_demo.py](../scripts/remote_demo.py) | Recorded [backend-lifecycle](evidence/backend-lifecycle.json): `T22-money-restart-process-lifetime-only-history`; [ui-demo](evidence/ui-demo.json): `UI-restart-new-session-same-ID-no-duplicate`. Full-demo fresh-state behavior is also source/runbook coverage. |

## Identity and certificate lifecycle

Contract: [identity-certificate-lifecycle](../openspec/specs/identity-certificate-lifecycle/spec.md).

| Requirement / original IDs | Source | Named checks and recorded evidence |
| --- | --- | --- |
| IC-01 / R3,R7; T17 | [identity.go](../internal/identity/identity.go), [server.go](../internal/money/server.go), [security client](../cmd/security-client/main.go) | Recorded [money-security](evidence/money-security.json): `rotation-expiry-and-connection-reuse`; deterministic [server_test.go](../internal/money/server_test.go): `TestCredentialExpiryCannotReachLedger`. |
| IC-02 / R4,R7; T02,T17,T18 | [identity.py](../scripts/identity.py), [caller.go](../internal/caller/caller.go) | Deterministic [test_identity.py](../scripts/test_identity.py): `test_failed_or_malformed_refresh_preserves_last_bundle`, `test_public_permissions_are_set_before_atomic_replace`; recorded [backend-lifecycle](evidence/backend-lifecycle.json): `real-ca-rollover-and-public-bundle-refresh`, `T18-wrong-bundle-missing-bundle-and-socket`. |
| IC-03 / R7; T17,T18 | [identity.go](../internal/identity/identity.go), [caller.go](../internal/caller/caller.go), [backend_suite.py](../scripts/backend_suite.py) | Recorded [backend-lifecycle](evidence/backend-lifecycle.json): `registration-removal-and-cached-expiry`, `T18-server-outage-valid-cache-then-expiry`, `T18-agent-outage-is-infrastructure-failure`. |
| IC-04 / R1,R7; T18,T22 | [identity.py](../scripts/identity.py), [remote_demo.py](../scripts/remote_demo.py) | Recorded [identity-restart](evidence/identity-restart.json) issuance/CA/agent/token/permission checks; [backend-extended](evidence/backend-extended.json): `extended-outage-expires-node-and-workload-no-automatic-reenrollment`, `explicit-ephemeral-node-reenrollment-and-live-source-recovery`. Deterministic [test_identity.py](../scripts/test_identity.py): `test_test_option_cannot_change_retained_ca_policy`. |

Administrative root taint/removal is additionally recorded as
`old-root-taint-rotation-and-revocation-propagation` in
[backend-extended](evidence/backend-extended.json). It does not establish
automatic expired-root sweeping or production-duration rollover.

## Private caller controls

Contract: [private-caller-controls](../openspec/specs/private-caller-controls/spec.md).

| Requirement / original IDs | Source | Named checks and recorded evidence |
| --- | --- | --- |
| PC-01 / R4,R7; T12,T20 | [remote Compose](../infra/remote-demo/compose.yaml), [control.go](../internal/control/control.go) | Recorded [callers-control](evidence/callers-control.json): `cross-caller-control-isolation`, runtime ports; [ui-demo](evidence/ui-demo.json): `cross-caller-control-isolation`, runtime mounts. |
| PC-02 / R4,R7; T12,T23 | [control.go](../internal/control/control.go) | Deterministic [control_test.go](../internal/control/control_test.go): `TestControlPeerAndProtocolCannotBecomeArbitraryRequest`; recorded [callers-control](evidence/callers-control.json): `kernel-peer-uid-enforcement`, `bounded-fixed-control-protocol`. |
| PC-03 / R3,R4,R7; T01,T02 | [trusted entrypoint](../cmd/trusted-caller/main.go), [unidentified entrypoint](../cmd/untrusted-caller/main.go), [caller.go](../internal/caller/caller.go) | Recorded [callers-control](evidence/callers-control.json): `trusted-caller`, `unidentified-caller`; [ui-demo](evidence/ui-demo.json): `real-trusted-transfer-browser-repeat`, `real-unidentified-no-SVID-TLS-rejection`. Unexpected-issuance handling is source coverage, not a separately rerun injection. |
| PC-04 / R4,R7; T02,T09,T16,T18 | [client.go](../internal/control/client.go), [caller.go](../internal/caller/caller.go), [app.go](../internal/web/app.go) | Recorded [callers-control](evidence/callers-control.json): `stable-caller-retry`; [backend-proxy](evidence/backend-proxy.json) bounded handshake/post-commit loss. Deterministic [app_test.go](../internal/web/app_test.go): `TestLimitsNoEvictionAndRetryableResult`. Malformed control responses/request-ID mismatch are source coverage; backend fault clients do not cover every production-caller transport branch. |

## HTTPS demo UI

Contract: [https-demo-ui](../openspec/specs/https-demo-ui/spec.md).

| Requirement / original IDs | Source | Named checks and recorded evidence |
| --- | --- | --- |
| UI-01 / R4,R6; T01,T02,T10 retired | [auth.go](../internal/web/auth.go), [assets.go](../internal/web/assets.go), [app.go](../internal/web/app.go) | Deterministic [app_test.go](../internal/web/app_test.go): `TestAssumedDemoSessionRequiresExplicitHTTPSAndCSRF`; recorded [ui-demo](evidence/ui-demo.json): `assumption-disclosure-session-CSRF-origin-Host`; [public-demo](evidence/public-demo.json): `public-HTTPS-disclosure-session-CSRF`. |
| UI-02 / R6,R7; T20 | [UI entrypoint](../cmd/ui/main.go), [Nginx route](../infra/remote-demo/spire.adianto.dev.nginx), [renewal hook](../infra/remote-demo/renew-browser-cert) | Deterministic [main_test.go](../cmd/ui/main_test.go): `TestBrowserCertificateValidityAndAtomicRenewal`; recorded [public-demo](evidence/public-demo.json) `certificate_hook`, certificate/permission/runtime checks; [runbook](remote-demo.md) installed edge. Scheduled future ACME renewal remains unobserved. |
| UI-03 / R6,R7; T11,T19 session subset | [app.go](../internal/web/app.go), [auth.go](../internal/web/auth.go) | Deterministic [app_test.go](../internal/web/app_test.go): `TestWebAuthorizationBeforeDispatch`, `TestSessionExpiryRotationLogoutAndOneUseCode`, `TestAssumedDemoSessionRequiresExplicitHTTPSAndCSRF`; recorded [ui-demo](evidence/ui-demo.json): `assumption-disclosure-session-CSRF-origin-Host`. Most expiry/role tests use the local unit profile; OIDC assertions are retired. |
| UI-04 / R2,R4,R7; T13,T14,T16,T22 | [app.go](../internal/web/app.go), [assets.go](../internal/web/assets.go) | Deterministic [app_test.go](../internal/web/app_test.go): `TestInterruptedConcurrentRepeatedAttemptsAndOwnership`, `TestLimitsNoEvictionAndRetryableResult`; recorded [ui-demo](evidence/ui-demo.json): `UI-restart-new-session-same-ID-no-duplicate`; [public-demo](evidence/public-demo.json): `public-real-trusted-transfer-stable-retry`. |
| UI-05 / R3,R4,R7; T08,T23 | [UI entrypoint](../cmd/ui/main.go), [app.go](../internal/web/app.go) | Deterministic [app_test.go](../internal/web/app_test.go): `TestLimitsNoEvictionAndRetryableResult`, `TestWebAuthorizationBeforeDispatch`; recorded [money-security](evidence/money-security.json): `observer-cannot-transfer`; [ui-demo](evidence/ui-demo.json) observer reads and mount boundaries. No unbounded load/DDoS acceptance is claimed. |
| UI-06 / R4; optional local profile, historical T10/T19 subset | [admin.go](../internal/web/admin.go), [auth.go](../internal/web/auth.go), [local_demo.py](../scripts/local_demo.py), [local Compose](../infra/local-ui/compose.yaml) | Deterministic [app_test.go](../internal/web/app_test.go): `TestSessionExpiryRotationLogoutAndOneUseCode`, `TestWebAuthorizationBeforeDispatch`; local admin kernel UID/socket ownership is source coverage, not a newly exercised end-to-end login test. |

## Original requirement and matrix coverage

R1–R7 map to the rows above. R8 is fulfilled by the original planning documents
and this document map; it is a workflow requirement, not a new runtime capability.
T01–T09 and T11–T18, T20–T23 have the bounded current/recorded/unit/source mappings
above. **T10 verified-human denial and T19 OIDC validation are retired for the
public profile**; retained session checks and optional local-login checks must
not be presented as passing those original human-authentication requirements.
The original full Gate F/G was not passed under its original definition.

## Limits and discrepancies

1. The public profile deliberately has no verified human and uses one assumed
   principal. Rate limits, idempotency scope and attempt ownership are shared;
   they are not individual-user protection. The separate local preview has
   one-use OS-owner login, without OIDC.
2. Original planning text and October 5 checkpoint bodies contained outdated
   OIDC, different-UID UI and unresolved-deployment statements. Active RFC,
   README/workspace/plan references are reconciled for this adoption. Dated
   checkpoints and original JSON evidence remain historical with explicit
   labels; they must not be read as current blocker/acceptance lists.
3. The retained substrate and supervised remote demo have different lifecycle
   contracts. The former preserves approved CA/agent state; a full restart of
   the latter recreates temporary trust and fake state. UI-only restart preserves
   the live Money process. These behaviors are not a contradiction to resolve
   by resetting retained state.
4. Reusing a browser key with another caller is rejected while its UI attempt
   record exists. The derived ID excludes caller kind; that UI conflict record
   is lost on UI restart. The baseline states the actual bounded guarantee,
   not persistent cross-caller conflict prevention.
5. Contract scenarios exceed the saved assertions in several places (notably
   every production-caller fault branch, local admin end-to-end, public absolute
   session-expiry boundaries, and every saturation variant). Named unit/source
   coverage is identified separately. Do not turn a structural spec pass into
   security acceptance. No application defect was silently fixed in this adoption.
6. Automatic expired-root sweeping, production-duration rollover, arbitrary
   TCP/IP packet-loss/retransmission, actual scheduled ACME renewal, other
   UID/agent layouts and production payments remain unverified/out of scope.
7. Executable attestation does not measure all memory/configuration or protect
   compromised host/Docker/SPIRE/UI/authorized processes. Public fake-money
   behavior is intentional, not a claim of payment readiness.

## Adoption verification

The implementation runs only local tooling/reference checks and deterministic
Go/Python tests; it does not run real-SPIRE suites, start/restart services,
change registrations, invoke the public verifier or trigger live transfers.
The real suites can use retained identity state or the shared live descriptor;
fixture isolation must be established before a future run.

The exact commands/results and distinction from historical integration evidence
are recorded in [openspec-adoption.json](evidence/openspec-adoption.json).
Independent review passed after the UI retry, eligible SVID-source and restart
wording corrections, including the health-check liveness clarification. The
review receipt is in the adoption evidence. Owner baseline acceptance remains
pending; independent verification and publication do not imply that acceptance.
