# Alpha.14 candidate base recovery

This opt-in harness accepts only `guilycst/balemoh-renovate-engine-poc-20261101` and a disposable `qualification/alpha14-*` base branch. It builds the compatible package, controller and request-backup binaries from one exact plugin commit. It does not publish an image or deploy to the homelab.

The workflow uses an ephemeral GitHub Actions token scoped to that fixture. It needs Contents and Pull requests write, Issues and Checks read, and Commit statuses write for the harness's own verification status. The product writer profile still disables optional stability-status writes. This harness does not qualify the production credential's permissions.

Cases cover a queued approval captured before an unrelated base commit, a controlled retained `blocked/base_changed` record, and a changed selected file that must stay blocked. Positive cases require native Renovate to create the exact tag+digest PR. The base then advances again with different inventory text. Maintenance must preserve the approved image, retain the new inventory body, regenerate its fingerprint and use the same request and PR.

After observing the real push, the harness exports the pending `rebaseHead` through the controller HTTPS catalog using the candidate backup command, stops the controller, restores the archive and restarts with the writer disabled. Read-only tracking must recover the exact new head. Another cycle must leave the remote head unchanged. A changed image file must create no PR. Previous fixture PRs and branches stay intact; the fixture's main branch stays at the bootstrap commit throughout the run. Test PRs stay open with manual merges and no automerge.

The harness clones each real PR head, recomputes the fingerprint with the candidate helper and verifies the exact image bytes and two-file diff. It then publishes a commit status on that head and requires the controller to observe passing. GitHub Actions token writes do not trigger ordinary `pull_request` workflows. No automatic PR-triggered CI claim is made by this qualification.

Run the recorded workflow on the disposable fixture base with `source_sha` set to the candidate commit. Its artifact contains the candidate image ID, package pins, controller/backup hashes, retained requests and final qualification receipt. It excludes credentials and raw engine logs. A passing candidate receipt does not qualify a later rebuilt or published OCI image; publication must verify both Linux platform binaries and deployment must use its own image pins.
