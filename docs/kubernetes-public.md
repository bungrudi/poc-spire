# Public Kubernetes SPIRE demo — 13 October 2026

[spire.adianto.dev](https://spire.adianto.dev/) now serves the complete five-scenario
Kubernetes/Istio/SPIRE/Kafka profile from main. The human remains an **assumed demo
operator**, not authenticated. SPIRE verifies processes, while both TLS layers,
exact Money/control IDs, broker admission and original-producer JWS remain separate.

[Actual owner directive](../openspec/changes/publish-kubernetes-demo-on-existing-domain/review.md)
authorizes merge/push, named public cutover and scoped cleanup. [Evidence](evidence/kubernetes-public-current.json)
records the clean normal application image from `51a9f85`, bounded startup fix
`e53c616`, verifier `cd3fb62`, actual UI/OCI index→platform provenance, public five
outcomes and ledger 0 → 2, browser defenses, renewal and supervised-route restart.
Go/application source is unchanged from the independently reviewed baseline.

The existing Nginx/browser certificate remains the edge. Nginx validates HTTPS
`127.0.0.1:18089` against system roots and exact `spire.adianto.dev`; UI accepts only
that public Host/Origin. The route service forwards only the owned UI service,
using main's ignored descriptor/kubeconfig/context. No Money/control/Kafka listener
is published. The existing certificate hook updates the protected PEM and owned
UI Secret through stdin; projected TLS reload preserves sessions/Money.

## Operation

```sh
systemctl status poc-spire-kubernetes-route.service
curl -fsS https://spire.adianto.dev/healthz
sudo systemctl restart poc-spire-kubernetes-route.service
```

A route-only restart preserves the current run, Pod/config guards and ledger.
Failed/corrupt/stopped whole clusters require explicit operator reset:

```sh
sudo systemctl stop poc-spire-kubernetes-route.service
python3 -B scripts/kubernetes_baseline.py stop
python3 -B scripts/kubernetes_baseline.py start --public
sudo systemctl start poc-spire-kubernetes-route.service
```

This reset creates fresh trust/run/stream and fake balances/history; do not
silently republish old attempts. Startup retains the existing 6 GiB floor, 4 GiB / 3 CPU
node caps and 2 GiB host reserve. Own endpoint downtime during explicit stop/reset
is allowed; foreign-container/default-context/retained-state checks stay enforced.
Route supervision does not repair a failed cluster. Actual host reboot/future
ACME issuance is unobserved; no HA/controller/reliability framework was added.

## Retired resources and preserved work

The old public `poc-spire-local-ui-e30f828e94` and obsolete preview
`poc-spire-local-ui-dd56511c20` each had four containers. Their precisely captured
IDs, owned native issuers/supervisors, unused labelled networks/images and preview
runtime are removed. `poc-spire-demo.service` is masked; the private `8fe59fe9e74c`
baseline was stopped with its helper. Four shared Docker apps, retained identity,
default context and access services remain. No global prune or age-based kill.

Six tracked/six untracked local-preview files are preserved in a local-only stash
and owner-only `.runtime/work-preservation` backup; obsolete UI changes were not
replayed. Runtime keys, kubeconfigs, cookie/CSRF values and private fingerprints
remain ignored. A wrong public Host returned an unmatched edge 502; the Go UI's
independent Host 403 was verified through authenticated loopback TLS. Neither is
misreported as another layer's denial. The first verifier's completed keys were
reused after assertion corrections, without additional ledger effects.
