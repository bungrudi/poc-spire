# Proposal

## Why

The AWS lab SPIRE agent is healthy, but injected Envoy proxies cannot obtain their SPIRE SVIDs. The shared Istio default waits for the proxy to become ready in a container post-start hook; SPIRE's Kubernetes attestor needs the kubelet to report that container first. This can prevent the proxy's SVID from being issued and keep the application run from becoming ready.

## What changes

- Override Istio's application-hold setting only on SPIRE-owned application Pods, while retaining their explicit CSI Workload API socket and exact proxy registrations.
- Allow the owned observer enough startup time for a first complete observation over the lab's SSM-backed Kubernetes connection. The run stays in bootstrap until observation and promotion succeed.
- Verify the fix in a fresh run and preserve unrelated namespaces and the shared mesh.

## Impact

Only the SPIRE-owned Pod template and its deployment observer change. The platform's global Istio setting, SPIRE selectors, application signature checks and other projects remain unchanged.
