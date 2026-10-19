# Tasks

- [x] Implement the guarded public origin, certificate SAN, UI NodePort, scoped ingress and loopback verifier.
- [x] Run focused and full Python tests, Go tests and strict OpenSpec validation.
- [x] Confirm the Ready UI rejects TLS 1.2 while the ALB target fails health checks; allow TLS 1.2 only on browser HTTPS.
- [x] Initialize the browser session before the loopback verifier calls the session API.
- [x] Publish an immutable source revision and image, deploy a fresh owned run and verify the AWS URL through ALB.
