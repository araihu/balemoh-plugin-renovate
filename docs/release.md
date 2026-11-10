# Public artifact delivery

Source manifest version: `0.0.1-alpha.13`, using SDK `v0.0.1-alpha.4`, Goshtoso `v0.4.4` and Renovate `44.197.0`.

Alpha.13 verifies mixed-manager Kubernetes image diffs without attributing another manager's declarations to the selected upgrade. See [alpha.13 release notes](releases/v0.0.1-alpha.13.md). The missing-PR tracking correction on this integration branch is unpublished and is not part of the immutable alpha.13 artifact. Publishing it requires a new version and separate deployment approval.

Alpha.11 introduced icon actions with tooltips in upgrade lists and details. Native links keep image/candidate-specific accessible names, keyboard help, 44px targets and distinct responsive IDs. The final PR submission keeps its explicit text. See [release notes](releases/v0.0.1-alpha.11.md).

Alpha.10 skips publishing Renovate's optional release-age commit status, keeping the writer's status permission read-only. Its approved homelab cutover and recovered PR #861 are recorded in [homelab deployment PR #862](https://github.com/guilycst/home-lab/pull/862). Alpha.11 left this engine profile, release-age eligibility, credentials and manual merges unchanged.

The alpha.8 pair was published and qualified in the isolated repository. Its [writer qualification](qualification/alpha8-writer/README.md) remains historical evidence.

Alpha.7 introduced the overview card, list PR states and candidate-review links.
Its [UI qualification](qualification/alpha7-ui/README.md) remains the evidence
for those unchanged interfaces. Alpha.8 requires its own publication receipt.

Alpha.6 introduced pinned-image requests, notification type selection,
collection diagnostics, coverage tables and automatic checkbox filters. Its
[source qualification](qualification/alpha6-ui/README.md) remains historical
release evidence.

The controller defaults to writer disabled. Writes require an owned, diff-verified PR; missing PRs require review and never replay automatically. Snapshot v1 and the pinned native engine remain unchanged. Empty update lists still do not prove that images are up to date.

Alpha.6 [published-artifact writer qualification passed](qualification/alpha6-writer/README.md) for pinned Kubernetes tag+digest, Dockerfile digest and untagged Compose updates. Each case verified an exact selected-file PR, real PR-triggered CI, duplicate admission, restart and backup/restore. The wrong-digest selection created no branch or PR. That alpha.6 HMG qualification enabled writes only through a private-fixture connection; it is not a statement of current homelab activation.

Alpha.5 publication and [isolated writer qualification passed](qualification/alpha5-writer/README.md). The shipped controller opened exactly one selected-file PR and retained verified tracking after restart. The initial GITHUB_TOKEN fixture had `unknown` checks; a [separate fixture-only PAT proof](qualification/alpha5-pr-ci/README.md) now qualifies automatic PR-triggered CI and exact-head passing tracking after restart. Homelab production workflows remain a separate rollout gate. Alpha.4 remains [unqualified after its preflight failure](qualification/alpha4-writer/README.md), with its immutable tag unchanged. At that historical alpha.5 qualification stage, production was read-only and the hosted bot remained unchanged. Current deployment pins and writer ownership belong to homelab manifests and operational evidence; upgrade PR merges remain manual.

## Prepared source

Repository: `araihu/balemoh-plugin-renovate`, public as requested. Local runtime directories, credentials, build outputs and Impeccable review snapshots are excluded by `.gitignore`. Validation receipts describe local fixture evidence and are not production claims.

The CI workflow runs generation, race tests, vet, generated-source drift checks, published SDK checks, and the pinned Renovate container profile/report tests. The publication workflow is manual, restricted to this repository's main branch or version tags, and depends on that CI workflow. Only the publication job receives package-write permission. Actions use the immutable revisions already used by the Alertmanager plugin.

## Publication

1. Commit and push the release source to the existing plugin PR. Keep PR merges manual. Do not modify or push the home-lab checkout in this stage.
2. Require the new repository's actual CI run to pass. Local command execution and actionlint do not prove GitHub-hosted workflow execution.
3. Dispatch Publish image for the accepted main commit or immutable prerelease version tag. A prerelease can reference an unmerged reviewed feature commit; merges stay manual. The workflow builds Linux amd64/arm64 and macOS arm64 package executables, and publishes an OCI image carrying Linux collector/server/request-controller binaries plus the App token and request backup commands. Tags identify source commit, workflow run and attempt; no latest tag is moved.
4. The workflow resolves exactly one child manifest per Linux architecture, extracts its installed plugin package and compares its SDK digests with the built package. Using child digests avoids the Docker classic image store's collision when pulling two platforms under one index digest. It also extracts both runtime tools from both architectures, checks executability and compares their bytes with the corresponding cross-built files. Only then does it record `runtimeTools` in image-pins.json. It saves a verified image-pins.json containing source commit, plugin version, image digest and package/manifest digests. Extraction verifies package bytes, not native arm64 execution on the amd64 runner.
5. Use only a successful verified receipt to regenerate the inactive home-lab pins. A failed post-publication verification can leave an unreferenced image in the registry; it must not enter GitOps. CI builds can have different package/image digests from the local fixture build, so do not reuse local pins after a rebuild.

The publication workflow requests SBOM and provenance from BuildKit. It does not independently attest their contents or change package visibility. Registry visibility and pulls must be verified before cluster activation.

## GitOps activation, separate stage

The home-lab runbooks at `k8s/renovate-requests-readonly/README.md` and `k8s/balemoh-renovate-requests/README.md` own activation and rollback. The existing collector, snapshot endpoint and request-tracking controller remain read-only on their current pins. This publication changes none of their deployments. The production writer stays disabled and its dedicated credential stays unmounted.

Dedicated read-only GitHub credential, reader token and CA trust must exist before a real collection. Provision secrets as sealed manifests. Do not reuse or change the current bot token. First validate storage ownership/locking, TLS authorization, one full dry-run collection and actual checkout provenance. Then verify retained snapshot, actual query size, list/detail/coverage/widget and notification baseline before scheduling collection. Real inventory can expose the deferred dense-query byte-budget limit; a failed query is not an empty or up-to-date inventory.

Existing Alertmanager installation entries must remain unchanged. No production exposure, Argo path switch, scheduled collection, secret rotation, or deployment is part of source/image publication.

## Runtime ownership cleanup

The App token refresher, request archive format and Restic runner belong to this repository and are compiled into the carrier image. Their source is not injected by GitOps ConfigMaps. See [runtime commands](runtime.md).

Homelab owns deployment inputs, credential references, storage, schedule and retention. Alpha.3 introduced the packaged commands and has separate read-only production qualification. Alpha.6 has its own verified publication receipt and replacement-artifact writer qualification. Alpha.8 must obtain its own successful publication receipt and qualify the shipped writer in the isolated repository. Neither an alpha.6 receipt nor a local/HMG package digest supplies alpha.8 deployment pins. The old alpha.2 image cannot run these commands. A new image publication does not authorize deployment or writer cutover. Requalify the normal renewal and production backup after an approved deployment. Earlier Python-runtime receipts do not qualify this Go replacement.
