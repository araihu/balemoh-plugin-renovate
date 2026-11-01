# Initial public delivery

Approved UI and manifest version: `0.0.1-alpha.1`. Source and publication status are recorded in the repository Actions runs and image receipts. The existing hosted Renovate bot remains the sole writer.

## Prepared source

Repository: `araihu/balemoh-plugin-renovate`, public as requested. Local runtime directories, credentials, build outputs and Impeccable review snapshots are excluded by `.gitignore`. Validation receipts describe local fixture evidence and are not production claims.

The CI workflow runs generation, race tests, vet, generated-source drift checks, published SDK checks, and the pinned Renovate container profile/report tests. The publication workflow is manual, restricted to this repository's main branch, and depends on that CI workflow. Only the publication job receives package-write permission. Actions use the immutable revisions already used by the Alertmanager plugin.

## Publication

1. Create the public repository and push the reviewed source as its initial commit. Do not modify or push the existing home-lab checkout in this stage.
2. Require the new repository's actual CI run to pass. Local command execution and actionlint do not prove GitHub-hosted workflow execution.
3. Dispatch Publish image for the accepted main commit. The workflow builds Linux amd64/arm64 and macOS arm64 package executables, and publishes an OCI image carrying Linux collector/server binaries. Tags identify source commit, workflow run and attempt; no latest tag is moved.
4. The workflow extracts each Linux image's installed plugin package and compares its SDK digests with the built package. It saves a verified image-pins.json containing source commit, plugin version, image digest and package/manifest digests. Extraction verifies package bytes, not native arm64 execution on the amd64 runner.
5. Use only a successful verified receipt to regenerate the inactive home-lab pins. A failed post-publication verification can leave an unreferenced image in the registry; it must not enter GitOps. CI builds can have different package/image digests from the local fixture build, so do not reuse local pins after a rebuild.

The publication workflow requests SBOM and provenance from BuildKit. It does not independently attest their contents or change package visibility. Registry visibility and pulls must be verified before cluster activation.

## GitOps activation, separate stage

The home-lab runbook at `k8s/renovate-advisory/README.md` owns activation and rollback. Keep the CronJob suspended and snapshot endpoint at zero replicas while publication and dedicated credentials are prepared.

Dedicated read-only GitHub credential, reader token and CA trust must exist before a real collection. Provision secrets as sealed manifests. Do not reuse or change the current bot token. First validate storage ownership/locking, TLS authorization, one full dry-run collection and actual checkout provenance. Then verify retained snapshot, actual query size, list/detail/coverage/widget and notification baseline before scheduling collection. Real inventory can expose the deferred dense-query byte-budget limit; a failed query is not an empty or up-to-date inventory.

Existing Alertmanager installation entries must remain unchanged. No production exposure, Argo path switch, scheduled collection, secret rotation, or deployment is part of source/image publication.
