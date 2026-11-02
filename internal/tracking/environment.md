# Environment Variables

## Config

 - `UPGRADES_REPOSITORY` (**required**, non-empty) - Single repository allowed by the installation, in owner/name form.
 - `UPGRADES_APP_SLUG` (**required**, non-empty) - Expected GitHub App slug; checked before each token is issued.
 - `UPGRADES_APP_OWNER` (**required**, non-empty) - Expected owner login of the GitHub App registration.
 - `UPGRADES_APP_CREDENTIALS_DIR` (default: `/credentials/app`) - Secret directory containing app-id, installation-id and private-key.pem.
 - `UPGRADES_APP_TOKEN_DIR` (default: `/github-read`) - Memory-backed directory shared with the controller; contains token and expiry.json.
 - `UPGRADES_APP_REFRESH_SECONDS` (default: `2400`) - Renewal interval in seconds; shorter intervals are for isolated qualification only.

