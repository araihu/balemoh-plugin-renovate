# Native digest binding proof

This isolated test runs the unmodified Renovate 44.197.0 binary carried by the immutable alpha.5 image. It tests a native `matchJsonata` package rule, applied after lookup, that disables an update unless its target digest, tag presence/value and update type match the approval. `allowedVersions` alone cannot bind a mutable tag's digest.

The private fixture is `guilycst/balemoh-renovate-engine-poc-20261101`. A loopback-only OCI registry serves an approved digest A, then a changed digest B. Each native invocation uses a fresh checkout/cache. For Dockerfile tagged digest, Kubernetes pinned tag upgrade and Compose digest-only references, success requires:

1. A full dry-run selects exactly one branch/update at A.
2. A real writer resolves B and creates no branch, PR or issue, with previous GitHub inventory unchanged.
3. A real writer resolves A and creates exactly one unmerged PR/branch, changing one complete approved image reference in one file.

Existing PRs/branches and fixture main remain unchanged during each run. Dashboard, automerge, scripts, plugins and stale-branch pruning stay disabled. The workflow uses only its short-lived fixture `GITHUB_TOKEN`. The native binary is not patched. The loopback registry requires `internalHostAccess: allow` and an insecure host rule only inside this proof container. Production keeps `internalHostAccess: block`.

Install `workflow.yaml` as `.github/workflows/digest-binding.yaml` and `qualify.py` as `qualification/digest-binding.py` in the fixture. The three files under `fixtures/` belong under `digest-binding/`. Keep the fixture's existing dashboard-disabled Renovate policy. Temporarily permit Actions PR creation, dispatch manually, then disable this workflow and restore the previous Actions permissions. Fixture-only hourly/concurrent limits of ten leave room for retained proof PRs. Product limits are unchanged.

Raw engine logs and tokens stay ephemeral. Uploaded JSON contains bounded projections and assertion results. A passing receipt proves native target binding, not a published replacement controller or production cutover. PR-triggered CI remains a separate qualification because `GITHUB_TOKEN` normally suppresses downstream PR workflows.

## Recorded result

Passed on 2026-11-03, [run 41653260099](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/41653260099), fixture base `2925d0a0ff1fcb1c1057bb15788b2073f66b9fc8`. [Receipt](qualification.json) records the three native rules and exact references. PRs [#7](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/7), [#8](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/8) and [#9](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/9) remain open and unmerged. Each changed-digest attempt resolved B and left the complete prior branch/PR/ordinary-issue inventory unchanged.

Earlier harness attempts stopped on a config migration warning and fixture rate limits. The partially completed attempt created PRs #5/#6 before the third positive case hit the hourly branch budget. Those PRs remain open and were preserved by the final complete run. No proof PR was merged or removed. The final workflow is disabled and Actions PR-creation permissions restored.

Run the plugin's opt-in GET-only tracker against these retained native branches:

```sh
go test -tags digestproof ./internal/requests -run '^TestNativePinnedPRTracking$' -v -count=1
```

This uses the authenticated local GitHub CLI credential only for reads. It calls the shipped tracking code directly, checks PR author/base/owned branch/exact full-reference diff/head and expects unknown Checks. Native proof branches use a separate prefix; this does not claim controller admission, durable execution or restart against those branches. Local regression tests cover durable canonical request receipts separately.

Receipts: [native run](workflow-run.json), [live tracking and restored fixture permissions](tracking.json), [production preservation](preservation.json). Hosted Renovate updated its own dashboard during this task; its policy stayed unchanged and this task made no dashboard edits.
