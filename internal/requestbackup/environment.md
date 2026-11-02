# Environment Variables

## Config

 - `UPGRADES_REPOSITORY` (**required**, non-empty) - Repository whose request catalog may be backed up.
 - `UPGRADES_BACKUP_URL` (**required**, non-empty) - Private authenticated HTTPS endpoint ending in /v1/requests.
 - `UPGRADES_BACKUP_TOKEN_FILE` (default: `/credentials/reader/token`) - Reader bearer file; never the action or GitHub writer credential.
 - `UPGRADES_BACKUP_CA_FILE` (default: `/credentials/reader/ca-base64`) - Base64-encoded DER trust anchor used by the private controller endpoint.
 - `UPGRADES_BACKUP_WORK_DIR` (default: `/work`) - Writable disposable work directory, without the production request PVC.
 - `UPGRADES_BACKUP_RESTIC_BIN` (default: `/tools/restic`) - Pinned Restic executable supplied by the deployment.
 - `UPGRADES_BACKUP_RESTIC_REPOSITORY` (**required**, non-empty) - Explicit expected isolated repository; must equal RESTIC_REPOSITORY before any operation.
 - `RESTIC_REPOSITORY` (**required**, non-empty) - Restic backend repository selected by the deployment.
 - `UPGRADES_BACKUP_HOST` (**required**, non-empty) - Snapshot host used by both backup and retention selection.
 - `UPGRADES_BACKUP_TAG` (**required**, non-empty) - Snapshot tag used by both backup and retention selection.
 - `UPGRADES_BACKUP_KEEP_DAILY` (default: `30`) - Number of daily snapshots to retain within this exact host/tag group.
 - `UPGRADES_BACKUP_KEEP_LAST` (default: `7`) - Minimum number of latest snapshots to retain within this exact host/tag group.

