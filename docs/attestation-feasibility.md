# P1 executable attestation evidence — 5 October 2026

The real-SPIRE feasibility suite passed on the existing `personal-vps` host at
`/home/ubuntu/Workspace/poc-spire`. The versioned result is
[attestation-latest.json](evidence/attestation-latest.json). This is the P1
issuance proof, not acceptance of the money service, callers, or remote UI.

## Verified release and selectors

SPIRE 1.14.0, Go 1.24.2 linux/amd64, Go SPIFFE SDK 2.6.0, Docker 29.1.3,
Compose 5.0.0, Linux 6.14.0-37-generic. The released SPIRE archive is verified
against SHA-256 `dd5238274a6f4c52610b008efe67464f22bf30ac9841ab57d07a7a1087904fac`.
The SDK and transitive Go dependencies are locked in `go.mod`/`go.sum`.
Fixture images use `scratch` and locally built static binaries; their actual
content-addressed image config digests are captured per run rather than using
an unverified registry tag as image identity.

Pinned-source review (SPIRE tag tree `d18ee04aa47be45ae5f58fab6431cf1ab8d3af77`):

- [Unix documentation](https://github.com/spiffe/spire/blob/v1.14.0/doc/plugin_agent_workloadattestor_unix.md)
  and [implementation](https://github.com/spiffe/spire/blob/v1.14.0/pkg/agent/plugin/workloadattestor/unix/unix_posix.go):
  `discover_workload_path = true` enables `unix:sha256`. Linux hashing opens
  `/proc/<peer PID>/exe`, not a client-supplied binary path. The proof uses a
  128 MiB binary-size bound. The plugin recalculates on attestation; no hash
  cache is implemented in that plugin.
- [Docker documentation](https://github.com/spiffe/spire/blob/v1.14.0/doc/plugin_agent_workloadattestor_docker.md)
  and [implementation](https://github.com/spiffe/spire/blob/v1.14.0/pkg/agent/plugin/workloadattestor/docker/docker.go):
  observed selectors include `docker:label:poc-spire.attestation.role:<role>`
  and `docker:image_config_digest:<digest>`. The latter is an image config
  digest, not a manifest/repository digest. Failed image inspection omits it;
  requiring it in the entry therefore fails closed.
- [Workload attestation](https://github.com/spiffe/spire/blob/v1.14.0/pkg/agent/attestor/workload/workload.go)
  combines plugin outputs; a failed plugin contributes no selectors.
  [Cache matching](https://github.com/spiffe/spire/blob/v1.14.0/pkg/agent/manager/cache/lru_cache.go)
  requires every entry selector to be present, including across plugins.
  The test server is fresh and creates only one workload registration.
- [Peer tracking](https://github.com/spiffe/spire/blob/v1.14.0/pkg/agent/endpoints/peertracker.go)
  checks the original peer is still alive after attestation, reducing PID-reuse
  substitution. It does not claim to measure every instruction, configuration,
  future `exec`, or mutable process memory.
- [Join-token service](https://github.com/spiffe/spire/blob/v1.14.0/pkg/server/api/agent/v1/service.go)
  constructs the canonical agent ID under `/spire/agent/join_token/<token>`.
  The optional token CLI SPIFFE ID adds an alias; it is not the canonical
  parent. The runner verifies the actual server agent matches its parent,
  then redacts that parent because it includes the one-use token.

## Measured least-privilege feasibility layout

The temporary SPIRE server and agent run as the existing host user (UID/GID
1000). The agent uses the host's existing procfs view and Docker-group access;
no root, added capabilities, host-PID container, new group membership, sysctl,
firewall, or host security policy change is required for this layout. Docker
API authority remains part of the trusted computing base; it is not a
read-only API.

Three fixture containers run as `1000:1000`, with identical host UID mapping,
read-only root filesystems, all capabilities dropped, `no-new-privileges`,
and a private internal Docker network. Their sole host bind mount is the
public Workload API socket directory, mounted read-only; private admin/state
directories and the Docker socket are not mounted into workloads. The public
API socket permits connecting but grants credentials only after attestation.
No ports are published. A test-only TCP sentinel proves network reachability;
it is not a money endpoint, plaintext money fallback, or remote trigger.

The approved registration requires the exact attested agent parent **and**
UID **and** executable SHA-256 **and** deployment role **and** image config
digest. An application cannot update labels or registrations. Native Linux
Docker was tested; rootless Docker, Docker Desktop, a containerized agent,
and the RFC's proposed application UID 10001 remain unverified.

## Results

| Case | Observation |
| --- | --- |
| Approved executable, approved deployment | Exactly `spiffe://poc.internal/caller/trusted` issued |
| Unidentified executable, unidentified deployment | Socket/network reachable; `PermissionDenied: no identity issued` |
| Probe executable inside the trusted container | Same UID, image and deployment; only executable digest differs; no identity |
| Approved executable in wrong deployment | Same UID, image and executable; only deployment label differs; no identity |
| Rebuilt executable with old registration | Same approved deployment/image; changed digest receives no identity |

Each case requires a fresh, correlated agent selector observation. The proof
first waits for approved issuance; a missing socket, infrastructure failure,
timeout, wrong identity, incomplete selectors, or unexpected credential makes
the suite fail. The three binaries are distinct test builds of one instrumented
client, with different compiled markers; actual caller applications are later
milestones. Credentials remain in memory/private temporary files, never in
evidence. CA, join token, private keys, registrations, runtime directories,
containers, image and network are torn down after the run.

Early failures were fixture execute permissions, an incorrect node-parent
assumption, and registration readiness. An unsupported synchronization setting
was removed. The saved failed runs are diagnostic history, not passing tests.

## Remaining boundary

Gate B is demonstrated for this measured UID-1000/native-agent feasibility
layout. Re-run the attestation matrix after changing UID, agent namespace,
privileges, binaries, or image policy. This M1 report alone does not establish money mTLS, authorization, ledger,
OIDC, remote HTTPS, credential rotation or RFC T01–T23 full-suite acceptance.
Later verified M2–M4 results and their limits are in the
[implementation checkpoint](implementation-status.md).

Rudi subsequently approved protected project-local SPIRE CA/database and
agent state under `.runtime/identity/` (directories 0700, files 0600, excluded
from version control; no app mounts). M2 restart/permissions checks pass.
Administration remains local; no persistent host access or remote deployment
was added. The P1 runner remains ephemeral.
