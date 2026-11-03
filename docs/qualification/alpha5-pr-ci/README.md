# Alpha.5 PR-triggered CI qualification

Status: **passed**, 2026-11-03. The [published-controller run](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/41633156781) created [fixture PR #4](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/4), which automatically triggered the [successful PR CI run](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/41633315168). Both runs used frozen fixture base `36adf7809988a9f912d499d085ad7de782597796`. The controller image and executable bytes match the original alpha.5 writer proof.

The published alpha.5 controller image remains unchanged. This client uses a new selected file, expected writer `guilycst`, and a token restricted to the private synthetic `guilycst/balemoh-renovate-engine-poc-20261101` repository. The production writer credential is never read or used. The token expires on 2026-11-04. The writer token is stored temporarily as encrypted Actions secret `RENOVATE_CI_FIXTURE_WRITER` in the fixture repository, then removed after the run. A separate short-lived `GITHUB_TOKEN` provides repository read, PR, Issues, Checks and commit-status read access. The proof workflow receives no GitHub write permissions.

Install `pr-ci.yaml` as `.github/workflows/selected-pr-ci.yaml`, `verify-pr-ci.py` as `qualification/verify-pr-ci.py`, and a synthetic Deployment with nginx `1.25.3` plus an unselected Alpine sidecar at `k8s/alpha5-ci-selected.yaml`. The workflow runs on `pull_request`, checks out the exact head without persisted credentials, verifies one approved file and tag replacement, and publishes a commit status for that head. It has only contents read and statuses write permissions.

Install `workflow.yaml` as `.github/workflows/alpha5-pr-ci.yaml` and `qualify.py` as `qualification/alpha5-ci.py`. Manually dispatch that workflow to run `qualify.py` inside the published digest-pinned image with the verified alpha.5 receipt at `/qualification-pins.json`, output directory `/out`, and exact fixture repository/base in `GITHUB_REPOSITORY`/`GITHUB_SHA`. Pass the fixture-only writer token as `RENOVATE_TOKEN` and short-lived read token as `GITHUB_READ_TOKEN`. The carrier stays read-only with no capabilities and a disposable tmpfs. No controller binary is rebuilt or patched.

The client requires the controller to observe `passing` checks for the created PR, verifies the named Actions check and commit status against its exact head, then restarts the controller and requires retained passing status plus a fresh tracking timestamp. It retains alpha.5's selected-diff, duplicate-submission, prior-PR/branch, base, issue and manual-merge assertions.

Independently verify the Actions run event is `pull_request`, its head and PR association match, and its CI receipt names the same approved base/head/file. Disable both fixture workflows and remove the temporary writer secret afterward. Keep previous proof PRs and all credentials/state unrelated to this qualification unchanged. This does not authorize production rollout or writer cutover.

Local validator self-check passed for the exact selected diff and rejected a wrong checked-out head and unrelated-file changes. Workflow lint and Python compilation passed. Actual GitHub execution passed with the receipts below.

## Verified results

Renovate created exactly one new branch and PR, changing only nginx `1.25.3` to `1.26.3` in `k8s/alpha5-ci-selected.yaml`. PR #4 remains open, unmerged and without automerge. Previous proof PRs #1/#2/#3, their heads, base branch and ordinary issues stayed unchanged.

Actions reported event `pull_request`, associated with PR #4 and head `088e63c6d3e0a5c43432e245aeb347d75877a628`. The `selected-image` check and `fixture/selected-image` commit status both succeeded for that exact head. The controller recorded `checks: passing` without tracking errors. Duplicate approvals returned the same receipt and PR. After graceful restart, it retained the same PR/head and passing state, with a newer tracking timestamp.

The writer credential had access to the private fixture; its homelab ref request returned 404. The separate short-lived reader supplied Checks/status access. The temporary Actions secret and local plaintext copy were removed. All fixture workflows are disabled and repository Actions permissions stayed unchanged. The fixture PAT remains registered until its one-day expiry on 2026-11-04. Production writer credential was never used.

Receipts: [controller qualification](qualification.json), [CI-selected diff](pr-ci-receipt.json), [PR-triggered workflow identity](workflow-run.json), [credential scope](credential-scope.json), [cleanup](cleanup.json). Raw logs and credentials are not committed.

This qualifies automatic PAT-authored PR CI and exact-head tracking in the isolated Kubernetes fixture. It does not run the homelab's actual workflows, exercise production App installation permissions, qualify other managers, or authorize production deployment/writer cutover. The previous GITHUB_TOKEN proof remains separate historical evidence with `unknown` checks.
