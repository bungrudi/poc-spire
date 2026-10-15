# Shared platform profile

The approved `add-shared-platform-lab` change adds an existing-cluster adapter:

```bash
python3 scripts/shared_cluster.py render --config /absolute/path/target.json --output /private/path/policies.yaml
python3 scripts/shared_cluster.py deploy --config /absolute/path/target.json
python3 scripts/shared_cluster.py verify --config /absolute/path/target.json
python3 scripts/shared_cluster.py export --config /absolute/path/target.json --output /private/path/evidence.json
python3 scripts/shared_cluster.py remove --config /absolute/path/target.json
```

The platform supplies schema-v1 configuration: exact context/kubeconfig, API URL,
CA fingerprint, kube-system UID, lab/account/region marker, private runtime path,
a new twelve-hex-digit run ID, published normal application image and its exact
executable SHA256 inventory. The normal image is built by `build_shared_image.py`
from a clean pinned source revision. No test fixture binary is included.

SPIRE owns `spire-<run>` and `poc-<run>`, exact per-lab PSAT/kubelet RBAC and the
singleton SPIFFE CSI driver. Existing resources are collisions; they are never adopted.
The platform owns EKS, network policy support and shared Istio/CNI. This adapter never
creates a cluster, changes kubelet settings, approves arbitrary CSRs or installs Istio.

The shared adapter reuses the current server/agent, application, identity registration,
policy, broker bootstrap and monotonic guard implementations. It replaces the kind-only
transport with an explicit target, makes digest-pinned images pullable from ECR and
keeps the same requesting-container executable/Pod/owner/image conjunction. Verified
kubelet TLS remains mandatory. EKS kubelet trust and all integrations require actual
runtime acceptance; a readiness failure never permits a TLS bypass.

The lab uses one observed x86 node and no automatic scaling. New application identities
are bound to actual Pods; no existing public demo or issuer state is reused. Replacing
guarded Pods or losing initialized state quarantines the run. Export and remove it,
then deploy a fresh run. Removal deletes only recorded namespace/resource UIDs, stops
the exact owned guard process and removes that run's temporary issuer/browser keys.

`verify` checks guard health, the five browser scenarios, ledger conservation/replay and
CSRF/origin/Host denials through a new loopback-only port (18443 by default) with verified
browser TLS. It does not cut over `spire.adianto.dev`. The original standalone and
public entry points retain their defaults.

Integrated telemetry initially covers mesh TCP metrics. Collector scrape ingress is
restricted to the observability namespace on port 15020; application/control ports are
not opened. Application logs/spans are not claimed from opaque TCP mesh telemetry.
Extended executable-negative, outage and lifecycle acceptance remains explicitly
unrun until tested in the disposable shared environment.

The shared AWS lab is restricted to account `581695527661` and Sydney
(`ap-southeast-2`). The former Jakarta lab target is rejected.
