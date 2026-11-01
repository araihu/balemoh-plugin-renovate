# Balemoh Renovate

Read-only Renovate advisory plugin. Lists image/file groups, Git references, proposed tags/digests, policy/workflow evidence, optional PR links and coverage. Includes a private snapshot endpoint and an hourly collector implementation. No Balemoh core changes are required.

Alpha pre-release plugin at https://github.com/araihu/balemoh-plugin-renovate. CI validates the source; image publication is a separate manual workflow. Production activation remains separate. The existing hosted Renovate bot remains the writer.

## Build and test

Requires Go 1.27, Python 3 and Docker for container builds.

```sh
make test
go test -tags container ./internal/runner -run TestPinnedEngineProfile -count=1
make package
make image
```

`make package` builds a separate `dist/package/` directory with UI/backend executables for Linux amd64/arm64 and macOS arm64, plus Linux collector/server binaries outside the installed package. It validates the manifest and prints manifest/package digests. Published SDK `v0.0.1-alpha.3` is pinned without a local replacement. Generated SDK schemas and templ output are committed source. Environment docs use the pinned `envdoc` tool, and runtime parsing uses `caarlos0/env/v11`. Stock environment docs omit Go types.

For a multi-architecture OCI archive, without publishing:

```sh
docker buildx build --platform linux/amd64,linux/arm64 \
  --metadata-file .local/image-metadata.json \
  --output type=oci,dest=.local/image-upgrades.oci.tar .
```

## Installation

Install `dist/package/` on both the API and BFF using exact manifest/package digests. Use a compatible alpha.3 host, tested here at Balemoh `4a6b6492183015eed717be0c0104b6951ca10fed`. Source credentials belong only to the backend connection. The UI gets retained query grants and optional explicitly granted service IDs.

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

Mapped groups show the host's current service name and icon in the list, detail and summary widget. Sprite icons use Goshtoso's public `icon.Icon`; image icons use the validated same-origin host URL. Both are decorative beside the name. Hidden, missing, ungranted or unavailable catalog entries fall back to the image/file presentation. Name and icon changes never change dependency or candidate IDs. No image-name matching or runtime-version claim is made.

The detail route parameter is `dependency-id` because SDK route parameters require lowercase local identifiers; query input remains `dependencyId`.

## Candidate notifications

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
