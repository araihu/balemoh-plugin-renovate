# Published alpha.14 qualification

The published image `ghcr.io/araihu/balemoh-plugin-renovate@sha256:7569f6c4b95c907db1bb0285b7dcfca3b686b19a24c1dc3e2a96b79ef21d3e56` passed on native Linux amd64 and arm64. Its source is tag `v0.0.1-alpha.14`, commit `78828978553d59cf0e9035df609c4fcf54f02197`.

[Publication run 42607364086](https://github.com/araihu/balemoh-plugin-renovate/actions/runs/42607364086) verified the package and controller, request-backup and App-token bytes in both platform manifests against the release build. Independent anonymous manifest reads verified the index, child hashes and version alias. Extraction from both published platforms reproduced the package digests and recorded the installed runtime hashes, without rebuilding product binaries. See [platform verification](platform-verification.json) and [publication pins](image-pins.json).

| Native platform | Workflow | Upgrade PRs, left open |
| --- | --- | --- |
| Linux amd64 | [42608684067](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/42608684067) | #22 and #23 |
| Linux arm64 | [42609720803](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/42609720803) | #24 and #25 |

Each run pulled the same immutable index, verified its native runner architecture and source label, and executed the shipped controller and backup command. No product binary was rebuilt or patched. Both platforms passed queued approval recovery, retained `blocked/base_changed` recovery, exact tag+digest preservation, inventory-body preservation and fingerprint regeneration, pending-marker backup/restore, read-only restart with no replay, and rejection of a changed selected file with no PR. Exact-head verification statuses passed; no upgrade was merged or configured for automerge.

The [combined receipt](qualification.json) binds each native run and its runtime hashes to the image. [amd64](qualification-amd64.json) and [arm64](qualification-arm64.json) retain the request identities and before/after heads. Those runtime hashes match the binaries extracted from their published platform manifests.

The harness came from commit `98ce09f80cc3fa536ecc6f28646e66626f6ef88f`; its only extension over the candidate harness records the published image and native platform. The runtime source remains the immutable release commit. [workflow.yaml](workflow.yaml) accepts only an owned image digest and the private fixture repository. It serializes the native runs to preserve previous branches and PRs.

The fixture used ephemeral repository-scoped Actions credentials. Its limits were temporarily raised to admit these proof PRs, then restored along with the original Actions permission setting. The bootstrap adds only the opt-in qualification workflow. See [cleanup receipt](fixture-restored.json). Existing PR heads and branches stayed unchanged during each run.

The harness posts its own exact-head verification status after checking the real GitHub diff. Actions-token writes do not trigger ordinary `pull_request` CI, so no automatic PR-triggered CI claim is made here. The tests cover the Kubernetes tag+digest recovery path, not every image manager or production integration. No homelab credential, deployment, sync policy or production request volume was changed.
