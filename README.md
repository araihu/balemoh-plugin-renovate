# Balemoh Renovate


[Runtime commands and repository ownership](docs/runtime.md) cover App token renewal, request backup/restore and the deployment boundary. Plugin version `0.0.1-alpha.7` requires SDK `v0.0.1-alpha.4`; production activation stays separate.
Renovate advisory plugin with optional selected-upgrade PR requests. Lists image/file groups, Git references, candidates, policy/workflow evidence and coverage. Includes an overview widget with upgrade-type, security-flag and tracked-PR counts, a read-only hourly collector, private snapshot endpoint and durable request controller. Merging stays manual.

Alpha pre-release plugin at https://github.com/araihu/balemoh-plugin-renovate. CI validates the source; image publication is a separate manual workflow. Production activation remains separate. The existing hosted Renovate bot remains the production writer until a separate handover. Request support requires the companion Balemoh/SDK action changes; it defaults disabled.

## Build and test

Requires Go 1.27, Node.js 20 or newer, Python 3 and Docker for container builds.

```sh
make test
go test -tags container ./internal/runner -run TestPinnedEngineProfile -count=1
make package
make image
```

`make package` builds a separate `dist/package/` directory with UI/backend executables for Linux amd64/arm64 and macOS arm64, plus Linux collector, server and request-controller binaries outside the installed package. It validates the manifest and prints manifest/package digests. The request implementation pins published SDK `v0.0.1-alpha.4` without a module replacement. Release CI checks checksum resolution and builds the compatible host and plugin independently. Generated SDK schemas and templ output are committed source. Environment docs use the pinned `envdoc` tool, and runtime parsing uses `caarlos0/env/v11`. Stock environment docs omit Go types.

For SDK co-development only, point `SDK_SOURCE` at a companion SDK checkout and create an ignored workspace:

```sh
SDK_SOURCE=/absolute/path/to/balemoh/plugin-sdk
mkdir -p .local/actions
cat >.local/actions/go.work <<EOF
 go 1.27.3
 use (
  "$PWD"
  "$SDK_SOURCE"
 )
 replace github.com/araihu/balemoh/plugin-sdk v0.0.1-alpha.4 => "$SDK_SOURCE"
EOF
GOWORK="$PWD/.local/actions/go.work" make test
GOWORK="$PWD/.local/actions/go.work" make package
```

For a multi-architecture OCI archive, without publishing:

```sh
docker buildx build --platform linux/amd64,linux/arm64 \
  --metadata-file .local/image-metadata.json \
  --output type=oci,dest=.local/image-upgrades.oci.tar .
```

## Installation

Install `dist/package/` on both the API and BFF using exact manifest/package digests. Read-only releases use an alpha.3-compatible host. This request-capable branch requires the additive alpha.4 action host contract. Source credentials belong only to the backend connection. The UI gets retained query grants and optional explicitly granted service IDs.

```json
{
  "snapshotURL": "https://renovate-advisory.balemoh.svc:8443/snapshot.json",
  "browserRepositoryURL": "https://github.com/guilycst/home-lab",
  "intervalSeconds": 60,
  "maxSnapshotAgeSeconds": 43200,
  "notificationsEnabled": true,
  "serviceMappingsJSON": "[]"
}
```

Backend secrets are `snapshot-token`, the reader bearer token, and optional `snapshot-ca`, one base64 DER trust-anchor certificate. The host reads each from a credential file. It accepts a single line of bearer-safe characters, so PEM cannot be passed directly. The endpoint cannot disable TLS verification or follow redirects. The plugin never gets the GitHub bot token.

Connection grants are `upgrades.summary`, `upgrades.list`, `upgrades.get`. Notifications require the separate optional `notifications.publish/v1` capability grant and configuration toggle. The package default is disabled; enable the toggle and grant together to publish to the host inbox. This does not grant Git, Renovate writer or cluster operations. The inactive home-lab overlay now prepares both settings enabled.

Optional mapping string example:

```json
[{"dependencyId":"64-character lowercase hex ID","serviceId":"64-character lowercase hex ID"}]
```

Use exact derived IDs. Do not infer runtime identity from image or file names. Up to 256 mappings and 64 distinct services are accepted. The host service grant is separate. Add the same `serviceId` to the BFF installation's `serviceIDs` array. Obtain service IDs from the host catalog and dependency IDs from `upgrades.list` or `upgrades.get`; compare the intended repository, manager, file and declaration before saving a mapping. If a group covers several services, leave it unmapped in this version.

Mapped groups show the host's current service name and icon in the list and detail views. The overview widget shows repository-level counts and does not resolve service mappings. Sprite icons use Goshtoso's public `icon.Icon`; image icons use the validated same-origin host URL. Both are decorative beside the name. Hidden, missing, ungranted or unavailable catalog entries fall back to the image/file presentation. Name and icon changes never change dependency or candidate IDs. No image-name matching or runtime-version claim is made.

The detail route parameter is `dependency-id` because SDK route parameters require lowercase local identifiers; query input remains `dependencyId`.

## Understanding collection results

No candidates reported means Renovate returned an empty update list. It does not prove the image is up to date. Lookup unconfirmed means Renovate skipped a lookup or reported a warning. Conflicting declarations prevent reliable attribution to an image/file group. The No conclusive result filter includes all of these source-unknown states.

Collection coverage shows repository-wide counts, independent of list filters and pagination. The warning counts groups needing review separately from empty reports. Source records without a reliable identity are counted separately, because they cannot be called distinct images. Empty reports alone do not trigger an amber warning. This presentation does not change the source assessment, notifications or PR request eligibility.

The coverage page uses tables for group outcomes and warning causes. Help buttons
explain terms on click or keyboard activation. A stale collection stays marked as
stale; its findings remain visible. Scope and producer evidence are collapsed.

| Outcome | Meaning |
| --- | --- |
| Candidates reported | At least one upgrade candidate was returned. |
| No candidates reported | Empty update list. This is not evidence that the image is up to date. |
| Lookup unconfirmed | A lookup was skipped or returned a warning. The causes table shows the captured reason and next step. |
| Conflicting declarations | The same image/file group has disagreeing source records. |
| Outside upgrade scope | Disabled declarations or image tags inherited from Helm charts. These are not lookup failures. |

Counts cover the whole repository. Each image/file group contributes once to its
outcome and once to its captured lookup cause, if any. Unidentified declarations
are reported separately and do not count as distinct image groups. The collector
publishes fixed cause labels, never raw registry responses.

Empty unnamed Helm overrides and unresolved Dockerfile build arguments are outside
explicit-image counts. Named inherited chart images remain excluded groups; the
plugin does not guess the chart's resolved image version. The collection's scope
text retains the actual placeholder counts under technical details.

[Native qualification and warning investigation](docs/qualification/empty-results-2026-11-04.md) records the empty-result counterexamples and the reproduced causes behind the 17 HMG warning groups.

## Candidate notifications

When notifications are enabled, ordinary notices default to `major`, `minor` and `patch`. Configure `notificationUpdateTypes` on the plugin connection to replace that selection. `digest`, `pinDigest` and `pin` are opt-in. An empty array selects security-flag notices only. Security flags bypass the type filter, but `notificationsEnabled: false` disables all publishing. This setting does not filter the image list or change PR request eligibility.

```json
{
  "notificationsEnabled": true,
  "notificationUpdateTypes": ["major", "minor", "patch", "digest"]
}
```

Changing the connection configuration establishes a new silent baseline. Enabling another type does not send a backlog of existing candidates.

The first validated observation and the first observation after a collection failure/age gap establish silent baselines. New candidates in later current observations produce `info` notifications. Partial coverage does not suppress a proven candidate. Unassessed/excluded groups, stale data and failed collection never notify. Repeated polls and reappearing candidates do not notify again during the backend session.

Adapter `0.0.3` preserves `isVulnerabilityAlert=true` only on an explicit dependency update record. The pinned branch-report projection omits the security flag, manager and dependency type. PR/policy evidence joins by the available branch/file/package/name/current/target/update-type tuple only when it identifies exactly one extracted native identity and exactly one branch upgrade. Ambiguous projections remain unknown. Titles, labels, branch names and major versions cannot establish security. The optional flag's absence means unknown. A flagged candidate produces `warning`, a security badge and a separate `image-upgrade-security:<candidateId>` event. A later first security flag for an ordinary known candidate can notify without editing its prior event. The queue holds 64 frozen messages and prioritizes security notices over unaccepted ordinary notices. Delivery is best effort, with a silent baseline after restart and no durable plugin outbox.

The flag can be set by Renovate policy. It does not establish a CVE or vulnerability severity. Renovate's [OSV datasource list](https://docs.renovatebot.com/configuration-options/#osvvulnerabilityalerts) does not include Docker; this plugin adds no image scanner, automatic CVE coverage or extra GitHub permissions. The normalized snapshot remains the source of the displayed flag.

## Collector and artifact server

See [collector environment](internal/runner/environment.md), [server environment](cmd/snapshot-server/environment.md) and [design/invariants](docs/design.md).

The collector has no writer-mode flag. It enforces Renovate full dry-run and requires a dedicated GitHub credential limited to this repository with contents, pull requests and issues read access. Private registries are not enabled. The collector receives no Kubernetes token. It keeps a repository filesystem lock, limits execution time and publishes only after report and checkout validation. Failed runs leave the last file intact.

The snapshot server is an authenticated TLS reader for one normalized file. It has no upload/write API. It mounts snapshot storage read-only and receives only its reader token and TLS key. The collector does not receive either reader secret. TLS rotation requires a process reload; GitOps manifests declare the existing Reloader annotation. Producer snapshots and raw transient files are separate.

The first adapter deliberately emits unknown for empty update lists. Positive candidates remain useful under partial coverage. It does not yet produce proven `no_candidate` results; new positive candidates can notify even when other groups are unassessed, with partial coverage stated in the notice. External presets and credential-bearing repository policy are rejected for publication. Registry architecture, CVEs, runtime versions and deployment status are outside this version.

## Real-host development check

Build compatible Balemoh API and BFF binaries into `.local/host/balemoh` and `.local/host/balemoh-ui`, then:

```sh
make package
go run ./cmd/dev-host -notifications
```

This starts an isolated database and a TLS fixture source. No GitHub credential or registry is used. It checks real API/BFF page, coverage and widget responses, secret isolation and that repeated browser reads cause no additional source collection. It leaves a private local receipt and a loopback preview until stopped. Fixture execution metadata is simulated and explicitly labelled.

## GitOps handoff

See [initial public delivery](docs/release.md) for the prepared CI/publication sequence and the separate activation gates.

Home-lab owns `k8s/renovate-advisory`, the optional `k8s/balemoh-image-upgrades` overlay and `scripts/prepare-image-upgrades.py`. The overlay preserves the existing Alertmanager installation. It is not connected to Argo, the CronJob is suspended and the endpoint has zero replicas. Publish the exact OCI archive, provision sealed dedicated credentials and CA trust, then authorize activation separately. No settings for the existing Renovate app are changed.

The optional `-notifications` fixture mode starts with one candidate withheld, then publishes a synthetic native-shaped security flag on the next normal collector poll. It verifies one warning in the real host inbox, exact service reference and zero baseline notifications. This uses an isolated SQLite database and separate local operator credential. It is not a real CVE finding or a production scan.

## Selected upgrade requests

The optional `upgrades.request` action needs both an explicit named grant and
`data.action/v1`. Configure `controllerURL` with a private HTTPS origin and the
API-only secrets `controller-read-token`, `controller-action-token` and optional
`controller-ca`, base64 DER. Read/action bearers must differ. Neither GitHub nor
controller credentials reach the BFF.

The UI shows a confirmation page for supported tag upgrades, submits immutable
selection IDs and the retained snapshot digest, then redirects to the durable
request status. Repeat submissions return the existing receipt. It displays
queued/running/open/draft/closed/merged and uncertain states with last checked
time. Checks remain unknown when evidence is missing, incomplete or stale.
No merge/approval action exists.

The separate `image-upgrades-controller` mounts normalized snapshots read-only,
a private durable request directory and disposable engine workspace. See
[controller environment](internal/requests/environment.md). Writer defaults
false. Enabling it requires a dedicated repository-scoped engine credential,
read-only PR/checks/status credential and exact expected PR author. The reader
and writer credentials are reread from files to support refreshed tokens.

One worker performs full-dry-run preflight, verifies the exact file/package/tag/digest
and approved base, then invokes pinned Renovate with the same scope. Dashboard,
automerge, platform automerge and pruning are disabled. Existing repository
policy remains required. No hosted bot dashboard edit is involved.

Requests support unpinned tag major/minor/patch upgrades, tagged SHA-256 digest
updates, pinned tag upgrades and digest-only updates in Kubernetes, Docker Compose
and Dockerfiles. Native post-lookup rules reject an unapproved target digest
before branch generation. First-time pinning, digest removal, custom/Helm managers
and ambiguous declarations remain advisory. See [pinned-image scope and proof](docs/specs/spec-002-pinned-image-requests/spec-002-pinned-image-requests.md).
A base change during the external write can create an ambiguous branch; the
controller reports needs_attention and never automatically repeats that write.

Before rollout, qualify flock/fsync/rename on the chosen persistent volume and
prove retained-state restore. Keep the current hosted bot unchanged until the
separate single-writer cutover. Release builds must resolve the published SDK without a workspace replacement.
