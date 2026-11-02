# Isolated selected-upgrade PR proof

This harness targets only `guilycst/balemoh-renovate-engine-poc-20261101`, a private repository containing synthetic Kubernetes, Docker Compose and Dockerfile fixtures. It must not be pointed at the homelab repository. See the [execution results](https://github.com/guilycst/home-lab/blob/0b0a6ae801053f4c0458cdf66d50ced0fb9f855c/docs/specs/spec-001-balemoh-image-upgrades/pr-request-poc-results.md).

## Boundaries

- The hosted bot sees `renovate.json` with `enabled: false`. Its installation and homelab policy are untouched.
- The manually dispatched OSS job reads `.balemoh-poc.json` using `configFileNames`. It requires this file and performs no onboarding.
- The engine uses pinned Renovate `44.197.0`, the same image digest as the advisory collector.
- `GITHUB_TOKEN` is short-lived and scoped to this repository. The workflow grants Contents/Pull requests write and Issues read. No personal token or production credential is passed to Renovate.
- Dashboard, dashboard approvals, automerge, platform automerge, scripts, plugins and stale-branch pruning are disabled. There is no schedule, deployment or merge action.
- Repository-wide Actions PR creation is enabled only on this test repository. The workflow does not review or approve PRs. PRs authored using `GITHUB_TOKEN` do not establish normal bot-triggered CI behavior.

## Selection and assertions

`discover` is a full dry-run showing upgrades across all fixture files. Scoped `observe` and `write` use identical selection settings: the selected file, nginx only, and an exact `1.26.3` version constraint. The Kubernetes file also contains an unselected Alpine sidecar. The same nginx reference exists in another Kubernetes file and in Compose.

Every writer invocation first runs a full dry-run and calls `verify_report.py`. The gate rejects foreign files/packages/targets, extra branches and grouped upgrades. It then checks that `main` still matches the run's initial base. Post-run inspection verifies the actual PR diff and unchanged base, rather than trusting the process exit code or PR title.

`verify_live.py` asserts exactly one PR, one changed line in `k8s/selected.yaml`, unchanged negative-control files, no ordinary issues, exactly two Git branches, no automerge and no merge. Its optional expected head detects repeated requests changing the original PR.

Run local checks with no bytecode output:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s . -p 'test_*.py'
```

The executed phases were manually dispatched using:

```sh
gh workflow run proof.yaml --repo guilycst/balemoh-renovate-engine-poc-20261101 -f phase=discover -f scope=kubernetes
gh workflow run proof.yaml --repo guilycst/balemoh-renovate-engine-poc-20261101 -f phase=observe -f scope=kubernetes
gh workflow run proof.yaml --repo guilycst/balemoh-renovate-engine-poc-20261101 -f phase=write -f scope=kubernetes
```

After successful verification, the workflow was disabled and the test repository's Actions PR-creation permission was returned to false. The repository and open PR remain available for inspection. A new authorized execution requires reenabling both test-only settings first.

`workflow.yaml` is installed as `.github/workflows/proof.yaml` alongside `profile.py`, `verify_report.py`, and the contents of `fixtures/`. Do not modify the repository's base while an invocation is running. Artifacts expire after seven days; normalized receipts and these fixtures remain the durable reproduction reference.

## Real Balemoh observation

Writer reports are never passed to the advisory importer. A subsequent successful `observe` full dry-run supplies the actual native PR association. Copy `normalize.go` into the plugin module's ignored `.local` directory, then run it from that module against the downloaded artifact directory. It checks the full-dry-run profile and stable base, invokes the production snapshot adapter, validates the resulting snapshot, assigns a new run ID and publishes it atomically.

```sh
GOWORK=off go run .local/poc-normalize.go ARTIFACT_DIRECTORY SNAPSHOT_FILE
```

`track_host.py PLUGIN_MODULE SNAPSHOT_FILE NEW_PRIVATE_STATE_DIRECTORY --port 14083` starts existing compatible native Balemoh API/BFF binaries from the module's `.local` directory and its package from `dist/package`. It creates a separate SQLite database, loopback TLS snapshot reader and random local reader/service credentials. No GitHub or homelab credential reaches this host. Notifications remain disabled. Stop with SIGINT or SIGTERM; only its own child processes are terminated.

An atomically replaced newer snapshot is collected by the same connection's normal asynchronous poll. Compare the public `upgrades.list` query before/after and inspect the actual detail page's PR link. Freshness and rejected snapshot replacements remain visible; restarting the connection is not used as proof of an observation transition.

## Limits

This is a fixed nginx tag-upgrade case. Compose writer mode, digest updates, arbitrary selections, grouped changes and multiple simultaneous PRs are not qualified. File-specific branch identities and cleanup ownership need a separate production design. The test does not implement the plugin action contract, durable request controller, GitHub PR lifecycle reconciliation, CI integration or writer migration. Nothing is merged to prove activation.
