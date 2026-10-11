# Simplified SPIRE demo — 11 October 2026

The owner requested `spire.adianto.dev` and explicitly removed OIDC, asking to
assume an authenticated human identity in order to understand SPIRE. The app
therefore declares an **assumed demo operator**, not a verified human identity.
Anyone who can reach the browser page can obtain a demo session and trigger
fake transfers. This deliberately supersedes the original human-login gate.

Workload security is unchanged: real executable/deployment attestation,
direct mandatory SPIFFE mTLS in Money, exact-ID route authorization and a UI
identity that can read balances but cannot itself transfer. No workload
credential is supplied by the browser or forwarded through Nginx.

## Implemented profile

Current behavior contracts and source/test/evidence limits are indexed in
[the OpenSpec baseline](spec-baseline.md). Future behavior changes need a spec
delta and short plan explicitly reviewed by Rudi before code/apply; see
[the workflow](spec-workflow.md). This runbook owns deployment operations, while
the RFC owns architecture/security rationale. Saved results are recorded evidence,
not a fresh execution triggered by adoption.

- `cmd/ui -mode demo` requires HTTPS and an exact configured origin. OIDC and
  its implementation/dependencies have been removed.
- Browser sessions are in memory with Secure/HttpOnly/SameSite=Lax cookies,
  15-minute idle and one-hour absolute expiry. These are CSRF/session controls,
  not proof of the person's identity. Sessions can be renewed by opening `/`.
- Exact Host/origin and CSRF checks, escaped templates, restrictive CSP,
  bounded attempts, concurrency and a shared assumed-operator rate limit remain.
- Both callers and the isolated UI use UID/GID 1000. This uses the measured
  native UID-1000 agent without new host privileges. Separate container PID
  namespaces and control mounts remain; same-UID peer checks alone are not
  the isolation boundary. Applications receive no Docker/admin socket.
- `infra/remote-demo/compose.yaml` publishes only UI HTTPS at
  `127.0.0.1:18087`; Money and callers publish nothing. UI has a browser bridge
  as well as the internal Money network, since Docker does not publish ports
  for a container connected only to an internal network.
- `scripts/remote_demo.py` creates private temporary SPIRE trust and registers
  the measured image/executable/deployment conjunction, then supervises it.
  Shutdown removes that instance's resources; approved `.runtime/identity`
  state remains unchanged. There is no automatic reset/re-enrollment of the
  retained agent. A full demo restart resets trust, fake balances and retry
  history. A UI-only restart preserves Money's ledger/idempotency history.
- Browser TLS uses a protected combined certificate/key PEM. An atomic file
  replacement is reloaded on the next handshake without restarting the app.

## Testing

Run one real-SPIRE suite at a time:

```sh
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go vet ./...
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/ui_suite.py
```

The UI suite uses temporary self-signed browser TLS trusted explicitly by its
test client, verifies the exact browser hostname and uses real SPIRE for all
workloads. It checks disclosure of the assumption, Secure cookies, CSRF/origin/
Host rejection without mutation, trusted transfer, browser retry, UI restart
with a new session and the same stable ID, no-SVID/TLS rejection with no Money
HTTP status, mount isolation, loopback-only publication and complete cleanup.
It writes sanitized `docs/evidence/ui-demo.json`; failed infrastructure is not
an identity-rejection pass. Public browser/Let's Encrypt checks are separate.

## Installed VPS changes — explicitly approved

After automatic review initially rejected installation, the owner explicitly
approved this exact scope on 11 October 2026. All listed changes are installed:

1. A Let's Encrypt certificate for **only `spire.adianto.dev`**, using the
   existing Certbot account and an HTTP-01 webroot `/var/www/spire-acme`.
   Its private key remains protected in `/etc/letsencrypt`; a protected combined
   copy belongs to UID 1000 at `.runtime/browser/browser.pem` (0600, directory
   0700), mounted into UI only.
2. A new Nginx site `/etc/nginx/sites-available/spire.adianto.dev` and enabled
   symlink. HTTP serves only the certificate challenge and redirects to HTTPS.
   The existing sslh/Nginx HTTPS listener `127.0.0.1:8444` is reused. The site
   proxies to **HTTPS `127.0.0.1:18087`**, validates the upstream certificate
   against system CA roots and the exact hostname `spire.adianto.dev`, and
   preserves the request Host for the app's independent check. No forwarding
   header is trusted as identity. No backend port is publicly bound.
3. A persistent systemd service **`poc-spire-demo.service`**, running the
   project runner as existing user/group `ubuntu`, with Docker group access,
   private umask, restart on failure and clean shutdown. This introduces no
   new OS user, account, SSH access, raised agent privilege or firewall rule.
   Fake-money state is intentionally in memory and resets on full restart.
4. A Certbot deploy hook for **this certificate only**, atomically publishing
   the protected PEM and validating/reloading Nginx. The app reads the renewed
   certificate at handshake time; renewal does not reset balances/sessions.
5. Public remote access **without human authentication**, as requested. The
   page prominently states that anyone can trigger these fake transfers and
   that SPIRE authenticates processes, not the assumed human operator.

Nginx validation and public HTTPS verification pass. The deployed browser API
proves trusted transfer/stable retry and unidentified no-SVID/TLS rejection with
no Money HTTP status or ledger mutation. Live port/mount/capability boundaries,
owner-only browser key permissions, service/timer enablement and manual
certificate-hook continuity also pass. See [public evidence](evidence/public-demo.json).
Actual future ACME renewal and a real host reboot have not been observed; the
initial certificate issuance, renewal hook and startup/renewal timers are verified.
Deployment verification preceded Git publication. On 11 October 2026 the owner
separately requested public source publication to
[bungrudi/poc-spire](https://github.com/bungrudi/poc-spire); protected deployment
credentials and runtime state are excluded.

## Use the live demo

Open [spire.adianto.dev](https://spire.adianto.dev/); no login is required.

1. Click **Attempt trusted transfer**. The caller uses its own real SVID and
   Money verifies `spiffe://poc.internal/caller/trusted` before applying USD 1.00.
2. Repeat that button to retry the same logical attempt without moving money
   twice. Choose **New logical attempt** before making another transfer.
3. Click **Attempt unidentified transfer**. Expected evidence: no matching
   identity, verified Money server and TLS rejection; no Money HTTP status.

Balances are read using the UI's distinct `spiffe://poc.internal/ui` identity.
The fake ledger and retry history are in memory. A full service restart resets
them; a UI-only restart does not. The shared assumed operator is limited to
eight dispatched attempts per minute, two concurrent attempts and 64 historical
attempts per UI lifetime. Never start a new logical attempt to retry an unknown
commit outcome.

## Operations

```sh
systemctl status poc-spire-demo.service
systemctl is-enabled poc-spire-demo.service certbot.timer
curl -fsS https://spire.adianto.dev/healthz
python3 scripts/verify_remote_demo.py
```

The verification script makes one real **fake-money** transfer and saves only
sanitized evidence. It keeps cookies/CSRF values in memory. The public verifier
can run while the demo is running; isolated SPIRE suites acquire the same lock
and must run separately with the demo stopped.

```sh
sudo systemctl stop poc-spire-demo.service
python3 scripts/ui_suite.py
sudo systemctl start poc-spire-demo.service
```

Stopping cleans up this instance's ephemeral SPIRE keys/state, containers and
image. It preserves approved retained `.runtime/identity` state and the protected
browser certificate. No credentials or protected runtime logs belong in Git.
The project location is fixed in the service unit; moving it requires updating
the unit, Nginx/renewal configuration and protected paths.
