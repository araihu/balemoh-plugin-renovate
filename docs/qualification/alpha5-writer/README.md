# Published alpha.5 writer qualification

Status: **passed**, 2026-11-03. [Publication](https://github.com/araihu/balemoh-plugin-renovate/actions/runs/41627868177) and [isolated run](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/41628851415) succeeded using source `ab45eb7a4647e96abf0173c859ae12ce87075862`. This reruns the [alpha.4 published-controller proof](../alpha4-writer/README.md) after fixing native full-dry-run `no-work` handling. The initial controlled snapshot is checked against actual Git content by the shipped engine before any write. Production remains on alpha.3 with its writer disabled and hosted bot unchanged.

The client and manual workflow retain alpha.4's checks. Only release identity and the new selected fixture path change. Install `workflow.yaml` as `.github/workflows/alpha5-writer.yaml`, `qualify.py` as `qualification/alpha5.py`, and the verified publication receipt as `qualification/alpha5-pins.json` in the private `guilycst/balemoh-renovate-engine-poc-20261101` repository. Add `k8s/alpha5-selected.yaml` with one nginx `1.25.3` reference and an unselected Alpine sidecar. Keep previous fixtures and proof PRs unchanged.

Run only the digest-pinned published image. Temporarily permit Actions PR creation in the private fixture repository, dispatch manually, then disable the workflow and restore prior permissions. The short-lived repository token must never be replaced with the production writer credential.

Success requires exactly one new PR and branch, the selected-file one-line change to nginx `1.26.3`, duplicate requests sharing one receipt, retained PR tracking after controller restart, unchanged base and previous PRs/branches, no dashboard issue, no automerge and no merge. Raw engine logs and credentials stay ephemeral and are not uploaded. Published JSON receipts record immutable source/image identity and bounded results.

This qualifies the Kubernetes fixed-tag fixture only. `GITHUB_TOKEN` PRs normally suppress downstream PR workflows, so this does not qualify production App-triggered CI. Writer cutover and production deployment require separate approval. Upgrade PR merges remain manual.

## Recorded result

The shipped controller created [fixture PR #3](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/3), left open and unmerged. Exactly one new branch and PR changed only `k8s/alpha5-selected.yaml`, replacing nginx `1.25.3` with `1.26.3`. The Alpine sidecar, base branch, older files, ordinary issues and proof PRs #1/#2 remained unchanged.

Observed request states were `queued`, `running`, then `open`, without tracking errors. Repeated approvals retained one request and PR. After graceful controller restart, the retained receipt identified the same PR/head and recorded a newer tracking timestamp. Checks stayed `unknown`, as expected with no downstream PR workflows triggered by this token. No passing-check or merge-eligibility claim is made.

The fixture workflow is disabled and Actions PR-creation permissions restored. [Post-run preservation](preservation.json) confirms the production alpha.3 deployment and collector remain unchanged, writer disabled, writer credential unmounted, and no upgrade PR merges or dashboard mutations by this task.

Receipts: [image pins](image-pins.json), [anonymous registry verification](registry-verification.json), [writer qualification](qualification.json), [independent review](independent-review.json). Alpha.4 failure evidence remains unchanged.
