# Alpha.14 candidate base recovery

Passed on 9 November 2026 in [workflow run 42604715283](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/42604715283), from plugin commit `78828978553d59cf0e9035df609c4fcf54f02197` on Linux amd64. The [receipt](qualification.json), [package pins](package-pins.json), [runtime hashes](runtime-sha256.txt) and [release preparation](release-preparation.json) bind the results to that locally built candidate. No published-image or production qualification is claimed.

| Case | Result |
| --- | --- |
| Queued frozen approval | [PR #20](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/20), created on the newer base, then maintained without changing the approved target. |
| Retained `blocked/base_changed` receipt | [PR #21](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/21), same immutable approval and request through creation and maintenance. |
| Changed selected file | Stayed `blocked/base_changed`; no new PR. |

Both positive cases preserved exact tag+digest bytes, verified the two-file diff and recomputed the fingerprint from the actual GitHub head. Both exported and restored a pending `rebaseHead` with the candidate backup binary, resumed tracking with the writer disabled, observed passing exact-head status and left the remote head unchanged through another cycle. The extracted runtime hashes match the cross-built controller and backup bytes. Both upgrade PRs remain open and unmerged.

The [fixture permission receipt](fixture-permissions-restored.json) confirms restoration of the previous Actions setting. A bootstrap commit added only the opt-in workflow to fixture main before the run; all test base advances used the disposable branch. Existing fixture PRs and branches were unchanged during qualification. No homelab credential or deployment was used.

Two earlier attempts, [42602590302](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/42602590302) and [42603831032](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/42603831032), stopped before writes. The synthetic image files lacked `apiVersion` and `kind`, so the pinned Kubernetes manager extracted no dependency. The controller correctly returned `plan_not_actionable`. Valid Kubernetes fixtures fixed the harness; no product code changed for those attempts.

This opt-in harness accepts only `guilycst/balemoh-renovate-engine-poc-20261101` and a disposable `qualification/alpha14-*` base branch. It builds the compatible package, controller and request-backup binaries from one exact plugin commit. It does not publish an image or deploy to the homelab.

The workflow uses an ephemeral GitHub Actions token scoped to that fixture. It needs Contents and Pull requests write, Issues and Checks read, and Commit statuses write for the harness's own verification status. The product writer profile still disables optional stability-status writes. This harness does not qualify the production credential's permissions.

Cases cover a queued approval captured before an unrelated base commit, a controlled retained `blocked/base_changed` record, and a changed selected file that must stay blocked. Positive cases require native Renovate to create the exact tag+digest PR. The base then advances again with different inventory text. Maintenance must preserve the approved image, retain the new inventory body, regenerate its fingerprint and use the same request and PR.

After observing the real push, the harness exports the pending `rebaseHead` through the controller HTTPS catalog using the candidate backup command, stops the controller, restores the archive and restarts with the writer disabled. Read-only tracking must recover the exact new head. Another cycle must leave the remote head unchanged. A changed image file must create no PR. Previous fixture PRs and branches stay intact; the fixture's main branch stays at the bootstrap commit throughout the run. Test PRs stay open with manual merges and no automerge.

The harness clones each real PR head, recomputes the fingerprint with the candidate helper and verifies the exact image bytes and two-file diff. It then publishes a commit status on that head and requires the controller to observe passing. GitHub Actions token writes do not trigger ordinary `pull_request` workflows. No automatic PR-triggered CI claim is made by this qualification.

Run the recorded workflow on the disposable fixture base with `source_sha` set to the candidate commit. Its artifact contains the candidate image ID, package pins, controller/backup hashes, retained requests and final qualification receipt. It excludes credentials and raw engine logs. A passing candidate receipt does not qualify a later rebuilt or published OCI image; publication must verify both Linux platform binaries and deployment must use its own image pins.
