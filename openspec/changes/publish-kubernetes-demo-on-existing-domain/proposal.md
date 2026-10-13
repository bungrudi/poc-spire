# Publish Kubernetes demo on the existing domain

## Why

Rudi explicitly requests the reviewed Kafka/Kubernetes implementation on
`spire.adianto.dev`, merged/pushed to main, with superseded PoC processes removed.
The domain currently serves the old two-scenario Compose UI; the complete five-
scenario Kubernetes baseline is private. [Authorization](review.md) is already
given for this concrete cutover and bounded cleanup.

## What Changes

Preserve local WIP recoverably, merge the reviewed source, use main-local runtime
state and a fresh owned public Kubernetes profile, retain existing Nginx/browser
TLS and validate its HTTPS upstream, supervise only literal-loopback UI forwarding,
and project renewed browser PEM into the owned UI Secret. Validate the public
five-scenario/API/identity/ledger/defense path, then retire only identified old
public/preview stacks, supervisors and temporary resources. Existing SPIRE/JWS/
Istio permissions, caps, assumed-human disclosure and retained/shared state remain.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `https-demo-ui`: exact named public Kubernetes Origin/Host and existing verified edge.
- `kubernetes-demo-runtime`: named public ownership/route supervision and bounded retirement exceptions.

## Impact

Main checkout, Kubernetes deployment/profile helpers, existing certificate deploy
hook, one loopback-route service and named Nginx upstream. No Go behavior/dependency,
controller, HA, comparison or resource expansion. RFC/runtime/TODO/evidence will
identify the new public route and retired two-button profile.
