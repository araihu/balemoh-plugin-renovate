# Environment Variables

## Config

 - `UPGRADES_REPOSITORY` (**required**, non-empty) - GitHub owner/repository scanned with a dedicated read-only credential.
 - `UPGRADES_BASE_BRANCH` (default: `main`) - Single base branch. External presets are rejected by this adapter release.
 - `UPGRADES_GITHUB_TOKEN_FILE` (**required**, non-empty) - File containing a dedicated read-only GitHub installation token or fine-grained token.
 - `UPGRADES_WORK_DIR` (default: `/work`) - Disposable working directory. Raw reports never enter the published directory.
 - `UPGRADES_PUBLISH_DIR` (default: `/snapshots`) - Shared directory retaining the last validated snapshot and repository lock.

