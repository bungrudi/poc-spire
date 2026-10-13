# Spec Delta: https-demo-ui

## MODIFIED Requirements

### Requirement: UI-02 Fail-closed HTTPS and browser certificate replacement
Demo startup MUST require explicit mode, exact HTTPS origin and matching current browser certificate/key. Go UI SHALL serve HTTPS. The named public profile SHALL retain existing Nginx with verified HTTPS upstream and exact Host/Origin; private Kubernetes loopback access remains direct HTTPS. Browser TLS MUST remain separate from workload PKI and atomic PEM renewal SHALL preserve sessions/Money.

#### Scenario: Valid configuration and atomic renewal
- **WHEN** a valid hostname-matching PEM is provisioned or atomically replaced with another valid PEM
- **THEN** browser TLS handshakes use the current certificate while existing sessions and fake ledger continue

#### Scenario: Missing or invalid TLS configuration
- **WHEN** demo configuration has missing/mismatched/expired/not-yet-valid certificate material or a plaintext origin
- **THEN** startup or certificate loading fails without enabling anonymous plaintext fallback

#### Scenario: Kubernetes loopback browser access
- **WHEN** the isolated Kubernetes profile forwards its browser port
- **THEN** literal loopback exposes the matching Go UI HTTPS certificate and current session/CSRF/Host/origin controls, without changing the existing Nginx/public endpoint


#### Scenario: Named Kubernetes public cutover
- **WHEN** the owner-authorized Kubernetes profile serves spire.adianto.dev
- **THEN** existing Nginx validates the exact upstream hostname/chain over a supervised literal-loopback route and UI enforces the exact public Origin/Host
- **AND** browser renewal updates only the owned UI Secret without exporting workload credentials or restarting Money

### Requirement: UI-05 Read-only workload identity and bounded resources
While its run is valid, UI MUST use only its observer SVID for balances/enabled Kafka metadata/receipts, verifying exact Money. Compose triggers remain private UDS; Kubernetes triggers MUST use exact-ID adapters in PC-07 without caller mounts/keys. All scenarios SHALL share 64 attempts, two workers, eight dispatches/minute per principal and 128 sessions; bounded bodies/no unsafe eviction remain. UI MUST receive no Kafka/admin credentials or producer keys.

#### Scenario: Verified balance display and existing attempt lookup
- **WHEN** UI reads Money or an existing pending/done attempt is repeated at history capacity
- **THEN** it returns verified balances or the existing attempt without borrowing caller credentials or creating another dispatch
- **AND** an existing retryable attempt may redispatch with its original ID even at full history capacity, subject to rate/concurrency limits

#### Scenario: Forbidden transfer or resource saturation
- **WHEN** UI's SVID directly posts a Money transfer or new browser work exceeds configured limits
- **THEN** Money returns 403; UI returns 429 for concurrency/rate or 503 for capacity without eviction
- **AND** unavailable Money reads return an infrastructure failure, not fabricated balances

#### Scenario: Kubernetes UI uses its own identity at adapters
- **WHEN** the UI dispatches one fixed scenario through a configured private network adapter
- **THEN** its own observer SVID verifies that exact control adapter, with unchanged fixed API/ownership/quota/retry behavior and no borrowed caller state/key


## Traceability and proposed coverage

All original modified-block scenarios are retained. New Kubernetes scenarios are
NOT RUN. [Profile](../../profile.md), [KB acceptance](../../acceptance.md) and
[references](../../references.md) define the proposed gates. Current source and
main contracts remain unchanged at `98f0fa3`; existing evidence is background,
not Kubernetes proof.
