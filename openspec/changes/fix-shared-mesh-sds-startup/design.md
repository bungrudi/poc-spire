# Design

`kubernetes_mesh_identity.configure` sets `holdApplicationUntilProxyStarts: false` in the SPIRE Pod's `proxy.istio.io/config` annotation, alongside its existing `WORKLOAD_IDENTITY_SOCKET_FILE`. The kubelet may then publish the Envoy container ID before Envoy receives SDS credentials. Readiness probes and SPIRE-issued mesh mTLS remain required before application traffic is considered ready. Existing annotation fields are retained.

The shared observer's initial readiness deadline allows one bounded first cycle through the TLS-verified SSM route. It never promotes the run merely because the deadline elapsed: `guard-health.json` must say `observing`, and subsequent checks retain their freshness and identity gates. Deployment must use a fresh run ID and immutable source/image pins.

Live confirmation requires Envoy's exact SVID from SPIRE, a Ready proxy without `PostStartHookError`, functional signed-payload scenarios, and an unchanged unrelated-workload inventory. If the kubelet comparison disproves the proposed cause, revise this change before applying it.
