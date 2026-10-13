# Spec Delta: HTTPS public Kubernetes profile

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
