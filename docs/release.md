# Public artifact delivery

Candidate version: `0.0.1-alpha.14`, using SDK `v0.0.1-alpha.4`, Goshtoso `v0.4.4` and Renovate `44.197.0`.

Alpha.14 adds recovery across unrelated base commits and safe maintenance of the generated inventory on verified upgrade PRs. See the [release notes](releases/v0.0.1-alpha.14.md) and [recovery contract](specs/upgrade-requests.md#base-recovery-and-inventory-maintenance).

The optional persisted `rebaseHead` field requires compatible plugin, controller and request-backup readers. Build them from the same release commit. Publication must verify the package, controller, backup and App-token binaries in both Linux platform manifests before recording deployment pins. Older readers reject pending records containing this field; do not downgrade while those records exist.

The [real-GitHub candidate qualification](qualification/alpha14-base-recovery/README.md) passed on Linux amd64. The receipt identifies its exact source, package and runtime bytes. This is release preparation. Publication and deployment need their own verified receipts. Earlier alpha qualifications remain historical evidence and do not qualify this candidate. Upgrade merges remain manual.

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

The home-lab runbooks at `k8s/renovate-requests-readonly/README.md` and `k8s/balemoh-renovate-requests/README.md` own activation and rollback. Homelab owns its active image pins and writer setting. Source preparation and publication do not change them. Inspect the current manifests and runtime before a separately authorized rollout. Install the compatible plugin and request-backup readers first, then update the controller. Never downgrade a reader while a retained record contains `rebaseHead`.

Dedicated read-only GitHub credential, reader token and CA trust must exist before a real collection. Provision secrets as sealed manifests. Do not reuse or change the current bot token. First validate storage ownership/locking, TLS authorization, one full dry-run collection and actual checkout provenance. Then verify retained snapshot, actual query size, list/detail/coverage/widget and notification baseline before scheduling collection. Real inventory can expose the deferred dense-query byte-budget limit; a failed query is not an empty or up-to-date inventory.

Existing Alertmanager installation entries must remain unchanged. No production exposure, Argo path switch, scheduled collection, secret rotation, or deployment is part of source/image publication.

## Runtime ownership cleanup

The App token refresher, request archive format and Restic runner belong to this repository and are compiled into the carrier image. Their source is not injected by GitOps ConfigMaps. See [runtime commands](runtime.md).

Homelab owns deployment inputs, credential references, storage, schedule and retention. Alpha.3 introduced the packaged commands and has separate read-only production qualification. Alpha.6 has its own verified publication receipt and replacement-artifact writer qualification. Alpha.8 must obtain its own successful publication receipt and qualify the shipped writer in the isolated repository. Neither an alpha.6 receipt nor a local/HMG package digest supplies alpha.8 deployment pins. The old alpha.2 image cannot run these commands. A new image publication does not authorize deployment or writer cutover. Requalify the normal renewal and production backup after an approved deployment. Earlier Python-runtime receipts do not qualify this Go replacement.
