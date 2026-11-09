# Runtime commands and deployment boundary

`image-upgrades-app-token` and `image-upgrades-request-backup` are versioned Go executables in the plugin carrier image. No homelab source file or ConfigMap is needed to run them. They use the existing standard library, controller record types and caarlos0/env plus envdoc. No Go dependency was added.

## App token sidecar

Run `image-upgrades-app-token run` in a native sidecar. Mount the private App Secret only there. Give the controller a read-only mount of the shared memory directory. The refresher atomically publishes `token` and `expiry.json` as 0600 files, renews every 2,400 seconds by default, holds an exclusive process lock, and mints a fresh token on restart. SIGTERM stops the loop cleanly.

`image-upgrades-app-token ready` validates current metadata and the private token file. A failed renewal retains a still-valid token; within 60 seconds of expiry it removes credentials and fails readiness. Startup/readiness probes gate the controller. Each issuance verifies the configured App slug/owner, App ID, installation ID, exact read-only permissions and selected single-repository scope. The only POST mints an installation token. Repository write endpoints are rejected. HTTPS uses neither proxies nor redirects.

`image-upgrades-app-token qualify --pull-request NUMBER --receipt FILE` performs read-only PR/Checks/status/ref/content/file reads and rejects a changing head. The receipt omits credentials. An empty status list proves API access, not passing CI.

Required and default environment values come from [generated App configuration](../internal/tracking/environment.md). Repository and registration identity are deployment inputs. There is no hardcoded homelab owner, repository or App slug in runtime code. Environment documentation uses envdoc's Markdown format, which omits Go field types.

## Writer credential

The native Renovate engine needs Contents and Pull requests read/write, plus Issues, Metadata and Commit statuses read access for its single repository. Commit-status reads are required when it observes the branch after pushing; lacking this permission can leave an owned branch without a PR. The controller's separate tracking App does not supply credentials to the native engine.

The engine sets `statusCheckWhen.minimumReleaseAge=never` in both preflight and write profiles. This skips publishing the optional `renovate/stability-days` commit status, which requires status write access. Repository release-age eligibility and internal checks remain active; deferred native plans still fail preflight.

A branch without a verified PR becomes `needs_attention` and is never replayed automatically. Inspect its exact diff and repository state before operator recovery. Keep the branch and request receipt; do not delete either to force a retry. Upgrade PR merges remain manual.

## Base recovery

With the writer enabled, queued and `base_changed` requests can recover across unrelated base commits. The entire selected image file and its mode must still match the immutable approval. The native preflight must still produce the approved target. Verified open PRs can also be maintained against a newer base when the generated fingerprint helper is configured. Only the approved image bytes and the helper's digest line are published, using an exact old-head lease. Upgrade merges stay manual.

A pending maintenance push retains its expected head in `rebaseHead`. Read-only tracking can confirm it after restart; the controller never automatically repeats an uncertain push. Inspect `rebase_pending` or `rebase_blocked` requests in GitHub before operator recovery. See the [recovery contract](specs/upgrade-requests.md#base-recovery-and-inventory-maintenance).

Ship the plugin package, controller and request-backup executable together. Older strict readers reject the optional `rebaseHead` field. Do not downgrade while a retained record contains it. No production recovery or rebase is qualified by local Git tests alone.

## Request backup

`image-upgrades-request-backup run` reads [generated backup configuration](../internal/requestbackup/environment.md). The deployment supplies an authenticated HTTPS source, its reader credential and CA, isolated Restic repository, Restic binary, host/tag and retention policy. Restic credentials remain third-party environment configuration. Only RESTIC_/AWS_ and relevant runtime environment values are passed to the child process; unrelated GitHub credentials are excluded.

The runner checks the configured repository against the explicit expected target before side effects. It exports one consistent logical catalog, backs up through stdin, validates the Restic summary, runs `check --read-data`, and only then prunes the exact host/tag group. It never initializes a repository. Failures retain private diagnostics and stop before retention. It never mounts or copies the production request PVC.

Standalone operations retain the original operator-facing flags:

```sh
image-upgrades-request-backup export --repository owner/repo \
  --url https://controller.example/v1/requests \
  --token-file /credentials/reader/token --ca-file /credentials/reader/ca-base64 \
  --archive /work/requests.tar

mkdir -p /work/restored
image-upgrades-request-backup restore --repository owner/repo \
  --archive /work/requests.tar --destination /work/restored
```

Restore accepts version-1 tar archives written by the previous Python implementation. It validates all members, record identities and checksums before creating a private `state` directory. It rejects duplicate JSON keys, unknown record fields, links, traversal, corruption and retained targets. Writes use exclusive creation and fsync. A write failure leaves its partial target for diagnosis; never overwrite it to retry. The new exporter retains validated record JSON bytes rather than duplicating Python canonicalization. Hashes bind those exact bytes.

## Ownership

Plugin owns runtime commands, archive contract, tests, product specifications and controlled qualification sources. Product specifications are in [specs](specs/); historical PoC sources are in [qualification/legacy](qualification/legacy/). The opt-in volume and token-consumer tests live beside the controller in `internal/requests`, with `recoveryproof` and `refreshproof` build tags. They require deliberate operator setup and are not production jobs.

Homelab owns image pins, Argo Applications, Deployments/CronJobs, PVCs, SealedSecrets, network policy, backup backend/schedule/retention, operational receipts and deployment validation. Normal renewal, restart and encrypted restore evidence from the old deployment remains historical. These changes require a new published image and a separately approved rollout. Writer defaults stay false and merges stay manual.

See [local qualification receipt](runtime-validation.json). The legacy exporter ran against an isolated three-record TLS controller fixture; Go restored the actual archive and the controller verified recovery. The retained production volume and encrypted snapshots were not accessed.
