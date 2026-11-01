# Environment Variables

## Config

 - `UPGRADES_LISTEN` (default: `:8443`) - TLS listening address. No plaintext listener exists.
 - `UPGRADES_SNAPSHOT_FILE` (default: `/snapshots/snapshot.json`) - Read-only normalized snapshot file.
 - `UPGRADES_READER_TOKEN_FILE` (**required**, non-empty) - File containing the snapshot reader bearer token, at least 32 characters.
 - `UPGRADES_TLS_CERT` (**required**, non-empty) - Server certificate chain.
 - `UPGRADES_TLS_KEY` (**required**, non-empty) - Server private key.

