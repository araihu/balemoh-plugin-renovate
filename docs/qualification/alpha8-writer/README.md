# Published alpha.8 PR-creation qualification

Passed on 6 November 2026 using the published host [v0.0.10-alpha.8](https://github.com/araihu/balemoh/releases/tag/v0.0.10-alpha.8) and plugin [v0.0.1-alpha.8](https://github.com/araihu/balemoh-plugin-renovate/releases/tag/v0.0.1-alpha.8). The [receipt](qualification.json) binds image, platform, binary and source hashes to the results. No product binary was rebuilt or patched for these tests.

Host PR #73 merged as `01b8caa591d06044e2d4a6674a0f20a4a334dbf7`. Its tree matches the independently reviewed `38345867e990fbde05cd3fe1636824c5ffcad306`. [Publication run 42242854821](https://github.com/araihu/balemoh/actions/runs/42242854821) passed its CI, API/UI and read-only demo gates. Anonymous registry reads verified the amd64 and arm64 manifests and the version alias. Runtime qualification used the published arm64 artifacts.

## Results

| Case | Result |
| --- | --- |
| Wrong approved digest | Blocked as `plan_not_actionable`; no branch, PR or issue change. |
| Kubernetes tag+digest | [PR #16](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/16), exact selected reference replacement. |
| Dockerfile same-tag digest | [PR #17](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/17), exact selected reference replacement. |
| Untagged Compose digest | [PR #18](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/18), exact selected reference replacement. |
| Published host review and POST | [PR #19](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/19), created through the shipped UI/BFF/API/backend action path and displayed by its detail content endpoint. |

All four PRs remained open, unmerged and without automerge. Each passed real `pull_request` CI at its exact head, including the `selected-image` Check and `fixture/selected-image` commit status. Repeated submissions retained the same receipt. Controller restart, archive export/restore, fresh tracking after restore and rejection of a retained restore target passed for each positive case. Archive SHA-256 values are in the receipt; archives remain local test artifacts.

The published-host case also verified that review GETs create no request, cross-origin POST returns 403, stale approval returns 409, approved POST returns 303 with the retained request ID, and duplicate host submission returns the same receipt. The host showed the verified PR link after restart and restore. This was HTTP integration, not a new visual or mobile review.

## Fixture and credentials

Only `guilycst/balemoh-renovate-engine-poc-20261101` was writable. Fixture base `e5e4062b2a23132515929d3e5f8fbf16ee5dc88f` stayed unchanged during the suite. Setup added four synthetic files and their exact-diff CI allowlist, preserving earlier cases. Fixture-only hourly/concurrent limits were raised from 10 to 20 to retain existing open proof PRs. Previous PRs, branches, ordinary issues and unselected references remained intact.

The approved fine-grained writer token had Contents/Pull requests read-write and Issues/Commit statuses/Metadata read-only for this fixture alone, expiring 7 November 2026. Its private-file mode was 0600; the local plaintext copy was removed after qualification. A read-only probe of the production private repository returned 404. Controller PR tracking used the existing repository-scoped read-only App installation token. Ordinary-issue inventory used the writer token's approved Issues-read permission because the tracking App has no Issues permission. No token value is included in these files.

## Harness and limits

[qualify.py](qualify.py) runs inside the immutable plugin carrier with [cases.json](cases.json), [plugin pins](plugin-image-pins.json) and [host pins](host-image-pins.json). The host binaries are extracted from the pinned arm64 host image and checked against the recorded hashes. The script accepts only the isolated repository and a frozen commit. Inputs use `QUALIFICATION_CASE`, `GITHUB_REPOSITORY`, `GITHUB_SHA`, `RENOVATE_TOKEN` and the separate `GITHUB_READ_TOKEN`; no production writer credential is used.

Mount the script at `/qualify.py`, the case and pin files at `/qualification-cases.json`, `/qualification-pins.json` and `/qualification-host.json`, the verified host binaries at `/host`, and a writable evidence directory at `/out`. Run cases `wrong-digest`, `pinned-tag`, `dockerfile-digest`, `compose-untagged` and `host-action` in separate read-only containers with a writable temporary directory. The host review GET uses the content endpoint consumed by its lazy-loading shell; POST uses the real page action route.

Initial harness failures stopped before submission: an Issues inventory used the wrong read credential, and a review assertion inspected the lazy-loading shell instead of its content endpoint. The empty request catalog and failure diagnostics were retained before correcting the harness. Shipped binaries and existing requests were not changed or replayed.

The initial snapshot is controlled. Native preflight still verifies live Git content, base SHA, registry digest and the Renovate plan before writes. This qualifies selected PR creation and tracking, not independent collector discovery or production writer activation. Production deployment generations/templates, active images and hosted bot policy were unchanged; writer remained disabled and its credential unmounted. No upgraded workload was deployed, and no upgrade PR was merged.
