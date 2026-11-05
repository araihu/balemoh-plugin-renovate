# Empty-result and lookup-warning qualification

Verified on 4 November 2026 with Renovate 44.197.0, image `ghcr.io/renovatebot/renovate@sha256:d3aa7ee1726eb31bf0b8b006f07bcde28747a820e450cf7f4010c45913f5257f`.

## Decision

An empty update list does not reliably establish that no upgrade is available. Even an empty list with no dependency warning or skip reason is insufficient. Keep the existing `unknown` source assessment and neutral "No candidates reported" presentation. Do not promote the 123 HMG groups to "Up to date" or "No upgrade available under policy" based only on this report.

The pinned lookup initializes an empty result, and several different execution paths retain that shape. The native report writer serializes the dependency result without adding a universal per-dependency success marker. `currentVersion` alone is insufficient for digest checks.

## Native counterexamples

`TestPinnedEngineEmptyResultsAreNotNegativeProof` runs the unmodified native lookup and report writer against a controlled loopback OCI registry. Docker external networking is disabled. There is no GitHub credential, branch, issue or PR operation. The fixture allows internal HTTP only inside this container, so the production `internalHostAccess=block` setting remains unchanged.

| Fixture | Registry evidence | Native dependency result |
| --- | --- | --- |
| Current image | Only version 1.0.0 exists; current version is 1.0.0 | Empty updates, no warning or skip |
| Positive control | Versions 1.0.0 and 2.0.0 exist | Major candidate 2.0.0 |
| Policy filtering | Version 2.0.0 exists, but `allowedVersions` permits only 1.0.0 | Empty updates, no warning or skip |
| Unusable version tags | Registry returns `edge` and `stable`; neither is usable as a numeric Docker version | Empty updates, no warning or skip, no resolved current version |
| Failed first digest pin | Version 1.0.0 exists and `pinDigests=true`, but the manifest cannot be fetched | Empty updates, no dependency warning or skip, resolved current version 1.0.0 |
| Missing package | Registry returns 404 | Empty updates with an explicit lookup warning |

The first, filtered, unusable and failed-pin cases also pass assertions after native report serialization. This proves that the ambiguous empty result reaches the same reporting mechanism consumed by the plugin. This is a lookup/report qualification, not a full GitHub writer qualification.

Run:

```sh
go test -tags container ./internal/runner -run TestPinnedEngineEmptyResultsAreNotNegativeProof -count=1 -v
```

A stronger negative assessment would need explicit evidence of successful, complete version/digest checks and the policy outcome. That evidence must be captured and qualified before changing the snapshot contract. No log-text heuristic or inference from exit code, an empty warning list, or `currentVersion` is introduced here.

## Investigation of the 17 HMG warning groups

HMG snapshot `052f6582-76e7-4ed7-adf6-0ff0b281ebb6` was captured at 2026-11-03 20:20 UTC from repository commit `7d80723c32991d0927ac7001c48f92845f5465da`.

The collector intentionally deletes raw reports/logs after normalization. An older retained diagnostic report was available, but it was not treated as the exact HMG run. Instead, the exact HMG commit was archived into a separate read-only checkout and scanned with the pinned native engine and the same relevant repository policy/image-manager profile. Renovate's local platform forces lookup-only mode, so no GitHub credential or writer was involved. `internalHostAccess=block` remained set for this real-registry replay.

All 17 HMG identities, including their manager, file, declaration key and current digest, matched reproduced warning/skip rows. Equivalent duplicate occurrences were counted as one group. Registry probes below describe current responses on 4 November, not recovered HTTP traces from the original run.

| Groups | Finding | Evidence and next step |
| ---: | --- | --- |
| 9 | Digest-only references fall back to a missing `latest` tag | `ghcr.io/araihu/balemoh` covers 4 groups, `balemoh-plugin-renovate` covers 4, and `dex-operator` covers 1. Anonymous tag lists return 200, contain no `latest`, and `manifests/latest` returns 404. An explicit published tag resolves with 200. Native Docker digest lookup uses `latest` when no tag is provided. Prefer an explicit release tag alongside the digest in a separately reviewed GitOps change; do not create or repoint a release channel during diagnosis. |
| 3 | GHCR anonymous access denied | `ghcr.io/araihu/balemoh-plugin-alertmanager` covers 2 groups and `manja-site` covers 1. Anonymous repository token requests return 401 `UNAUTHORIZED`. Confirm package identity, publication and visibility before deciding whether scoped registry credentials are appropriate. Anonymous denial does not distinguish a private package from a removed or unavailable package. |
| 3 | Internal registry access deliberately blocked | `k8s/splitpro/restore-test.yaml`, `k8s/immich/restore-test.yaml` and `k8s/appflowy/restore-test.yaml` reference the cluster-local registry. The replay logs explicitly report the internal-host block. Decide whether these restore-test images belong in advisory scope. Any future access grant must be limited to the intended registry in the plugin collector, rather than globally allowing internal hosts. |
| 1 | Empty chart-inherited Argo CD tag | `k8s/argocd/values.yaml` gives `quay.io/argoproj/argocd` an empty `currentValue`, with no digest. Native lookup reports `invalid-value`. This override does not identify the image version selected by the Helm chart. Resolve chart inheritance or present it as inherited/unsupported; do not invent a running version. |
| 1 | Plane digest lookup currently rate-limited | `k8s/plane/migration-job.yaml` references `artifacts.plane.so/makeplane/plane-backend:v1.4.3`. The registry challenges with Docker Hub bearer authentication. An anonymous pull token permits listing 130 tags, including `v1.4.3`, but manifest requests return 429 `TOOMANYREQUESTS`. This is not evidence that the tag is missing. Scoped read authentication, an approved mirror or a later bounded retry can be considered separately. |

The native warnings for the nine missing-tag groups, three inaccessible GHCR groups and one Plane group all use the same generic "could not determine new digest" message. The original adapter collapsed these and the other skips into `lookup_unconfirmed`; that explains why the original UI could not identify the cause.

## Limits and boundaries

The local replay used current registry contents with the historical Git commit. It cannot reconstruct the registries' state at the original snapshot time. Four of the 123 formerly empty HMG groups produced a changed result on recheck. The remaining 119 were still empty, split between 69 groups with a resolved `currentVersion` and 50 without one. These are observations, not newly certified no-upgrade statuses.

No current bot configuration, registry visibility, credential grant, collector deployment or HMG package was changed. No request was submitted, and no PR or merge was created. Production controller was verified Ready with `UPGRADES_CONTROLLER_WRITER_ENABLED=false` and no writer credential mount. Only regression evidence/tests and documentation were added; earlier uncommitted UI work was preserved.

References: [pinned lookup implementation](https://github.com/renovatebot/renovate/blob/44.197.0/lib/workers/repository/process/lookup/index.ts), [pinned Docker digest fallback](https://github.com/renovatebot/renovate/blob/44.197.0/lib/modules/datasource/docker/index.ts), [pinned report writer](https://github.com/renovatebot/renovate/blob/44.197.0/lib/instrumentation/reporting.ts), [internal host access](https://docs.renovatebot.com/self-hosted-configuration/#internalhostaccess).

## Cause capture and presentation

The collector now captures private JSON debug output from the pinned engine during collection. Normalization joins recognized diagnostic facts to the exact Docker package and requested tag. Skipped declarations take precedence over HTTP facts, and ordinary Docker authentication challenges do not count as access denial. Only an unsuccessful scoped GHCR token request establishes that cause.

Snapshots retain the existing `lookup_unconfirmed` assessment reason plus one allowlisted cause code. They never retain raw log messages, URLs, headers, response bodies or credentials. The diagnostic reader rejects oversized input, and the collector removes its private working log after normalization. Unrecognized engine output falls back to a generic cause instead of claiming a specific failure.

The list shows a short cause; details explain the failure and next step. Coverage counts each warning group once by cause. Legacy snapshots show that the cause was not captured and need a new collection. This change does not retroactively enrich retained HMG snapshots or certify empty results as up to date.

A second native debug replay against the historical checkout reproduced the same 17 groups. The new classifier identified nine missing implicit `latest` tags, three denied GHCR token requests, three internal-host blocks, one empty Helm tag and one manifest rate limit. Normalization, redaction, ambiguous joins and legacy fallback have regression tests. A controlled local preview demonstrates all five categories without publishing the replay as production inventory.
