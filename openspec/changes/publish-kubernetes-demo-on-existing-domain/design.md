# Public cutover design

Preserve primary checkout WIP in a local stash/private backup and fast-forward the
reviewed implementation before deployment. Do not replay the old UI assets. Stop
the owned private baseline with its helper, then build a fresh normal public run
from committed main source under the unchanged 4 GiB/3 CPU and startup/reserve gates.

A small explicit public profile supplies `https://spire.adianto.dev` and the
existing owner-only browser PEM to Kubernetes UI creation. Only UI receives that
browser key; all workload/broker trust and exact process/route permissions remain.
A systemd service supervises the owned `kubectl port-forward` at 127.0.0.1:18089.
The existing Nginx site's HTTPS upstream port changes to 18089, retaining public
certificate/root/hostname verification. The existing renewal hook updates the
same protected PEM and the existing owned UI Secret; whole-directory projection
and Go TLS reload avoid an application restart.

Capture the exact old public/preview IDs before startup. After public validation,
stop/disable their supervisors and remove only their owned resources. The new
run's original protected snapshot excludes only those captured retirement IDs,
never its own kind node; retain all shared-container/default-context/identity
fingerprints. Public mode owns its named URL, so stopping/resetting that cluster
may interrupt that endpoint without pretending foreign-service damage or recovery.

Verify five public outcomes and exactly two new effects on a fresh ledger,
Host/Origin/CSRF denials, real UI Pod/image/context provenance, unchanged caps,
renewal projection and a route-service restart preserving run/ledger. Save
sanitized evidence and reconcile current contracts/runbook before final push.
No controller, HA, benchmark, passing-matrix repeat or host access change.
