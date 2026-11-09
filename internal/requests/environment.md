# Environment Variables

## Config

 - `UPGRADES_REPOSITORY` (**required**, non-empty) - Single allowed GitHub owner/repository. No client-supplied repository is accepted.
 - `UPGRADES_BASE_BRANCH` (default: `main`) - The repository base branch. A newer base requires an unchanged approved image file.
 - `UPGRADES_CONTROLLER_SNAPSHOT_FILE` (default: `/snapshots/snapshot.json`) - Normalized snapshot published by the existing read-only collector.
 - `UPGRADES_CONTROLLER_DATA_DIR` (default: `/requests`) - Durable private request directory; requires working advisory flock and atomic rename.
 - `UPGRADES_CONTROLLER_WORK_DIR` (default: `/work`) - Disposable per-execution workspace, separate from durable state.
 - `UPGRADES_CONTROLLER_ADDRESS` (default: `:8443`) - Private HTTPS listen address.
 - `UPGRADES_CONTROLLER_READ_BEARER_FILE` (**required**, non-empty) - Reader bearer credential file for authenticated status queries.
 - `UPGRADES_CONTROLLER_ACTION_BEARER_FILE` (**required**, non-empty) - Separate bearer credential file for explicit request submission.
 - `UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE` (**required**, non-empty) - Read-only GitHub token for PR reconciliation; never passed to the UI.
 - `UPGRADES_CONTROLLER_PR_AUTHOR` (**required**, non-empty) - Expected GitHub login creating PRs, including the [bot] suffix for a GitHub App.
 - `UPGRADES_CONTROLLER_GITHUB_WRITE_TOKEN_FILE` - Dedicated GitHub writer token file, required only after an authorized writer migration.
 - `UPGRADES_CONTROLLER_WRITER_ENABLED` (default: `false`) - Disabled by default. Enabling requires a separate writer handover and credential.
 - `UPGRADES_CONTROLLER_GENERATED_FINGERPRINT_FILE` - Optional Markdown fingerprint artifact. Only its Source digest line may change.
 - `UPGRADES_CONTROLLER_MAX_SNAPSHOT_AGE_SECONDS` (default: `3600`) - Maximum accepted collection age in seconds.
 - `UPGRADES_CONTROLLER_TLS_CERT` (**required**, non-empty) - HTTPS certificate path.
 - `UPGRADES_CONTROLLER_TLS_KEY` (**required**, non-empty) - HTTPS private key path.

