# Spec 002: Pinned image upgrade requests

Status: implemented on PR #4, unreleased. Pre-v0.1.0 extension to alpha.5. Native binding qualified on 2026-11-03. Production remains on alpha.5 read-only; this change does not enable writes or replace the current hosted bot.

## Context and objective

The homelab snapshot captured on 2026-11-03 contains 30 candidates. All carry digests; alpha.5 `requests.Supported` admits none. Twenty candidates use Kubernetes or Docker Compose, while ten use the advisory regex manager. Source coverage remains partial. The next useful journey is one selected pinned image upgrade created by Renovate, tracked by Balemoh and merged manually.

Related contracts: [upgrade requests](../upgrade-requests.md), [runtime ownership](../../runtime.md), and the authoritative request/snapshot types under `internal/requests` and `internal/snapshot`. Existing HTTP routes, role separation, request IDs, durable states and backup contract remain authoritative.

## Scope and representation

Existing `currentValue`, `currentDigest`, `newValue`, `newDigest` and `updateType` fields represent the approved old/new image references. No user-supplied repository, tag, digest, branch or PR URL is added to the action body. The server resolves those values from the retained selection and snapshot digest.

| Case | Approved old reference | Approved new reference | Required relation |
|---|---|---|---|
| Existing unpinned tag upgrade | `image:old-tag` | `image:new-tag` | Existing tag-only major/minor/patch behavior retained |
| Tagged digest update | `image:tag@sha256:old` | `image:tag@sha256:new` | Same tag, distinct valid digests, update type `digest` |
| Pinned tag upgrade | `image:old-tag@sha256:old` | `image:new-tag@sha256:new` | Approved tag and digest both retained, major/minor/patch |
| Untagged digest update | `image@sha256:old` | `image@sha256:new` | Both tag values absent, update type `digest` |

Initial manager scope remains Kubernetes, Docker Compose and Dockerfile. Accept only canonical lowercase SHA-256 digests with 64 hex characters. Preserve the existing tag/file/package validation. First-time pinning, digest removal, other digest algorithms, Helm/Kustomize/regex writers, grouped multi-file requests, interpolation and ambiguous source representations remain advisory. No capability or credential broadening is needed.

The observed twenty relevant candidates comprise nine tagged Kubernetes digest updates, ten pinned Kubernetes tag upgrades and one untagged Compose digest update. These counts identify a useful slice, not a claim that every source representation is executable.

## Component ownership

| Existing component | Required extension |
|---|---|
| `internal/requests/types.go`, `Supported` | Validate supported tag/digest combinations without weakening tag-only admission |
| `internal/requests/engine.go`, `engineProfile` and `verifyFullDryRunPlan` | Handle nullable tags and bind/compare the selected digest through native execution |
| `internal/requests/github.go`, `verifyDiff` | Verify complete old/new references and occurrence counts against approved base/head |
| Existing confirmation/details UI | Compare tags/digests accessibly without treating an absent tag as a missing digest |
| Existing request tests and isolated qualification sources | Preserve tag-only behavior, durable state and no-replay while proving the digest cases |

## Engine selection prerequisite

Before implementation, qualify how pinned Renovate 44.197.0 binds an explicitly selected target digest through both full dry-run and writer execution. A fixed `allowedVersions` tag does not itself pin registry content. A native lookup may resolve a different digest if the tag moves between runs.

The [native-engine PoC](../../qualification/digest-binding/README.md) passed for all three pinned cases. A final scoped `matchJsonata` package rule disables any post-lookup update whose target digest, tag presence/value or update type differs from approval, before native branch generation. The same rule is passed to preflight and writer; tests compare it with the live-qualified expressions. Fresh-cache writer attempts resolved a moved tag to digest B and created no branch, PR or issue; approved digest A created one exact selected PR per case. If a future engine version cannot provide that binding through the qualified native mechanism, the affected case stays advisory; do not replace Renovate with custom Git/PR writes or treat post-write mismatch detection as exact-target prevention. A new SDK or general job framework is not part of this slice.

## Validation and PR association

1. Admission validates supported old/new reference shapes, canonical digests and update-type relations. A stale snapshot or unsupported shape produces the existing controlled rejection, with no queued execution.
2. A full dry-run contains exactly the approved branch, source occurrences, image, old tag/digest and new tag/digest. Preserve global/repository problem rejection and the bounded `done`/`pr-created`/`no-work` result set. Missing, foreign or changed digest evidence blocks before the writer.
3. Normalization reproduces the selected dependency/candidate IDs. Existing candidate IDs already include `newDigest`; preserve IDs and version-1 records for existing tag-only requests. Approval remains bound to the repository commit and snapshot digest.
4. Writer execution receives the same qualified selection as preflight, with only dry-run disabled. Dashboard, automerge, scripts/plugins, pruning and approval/check bypass remain disabled. The engine binding prerequisite above is mandatory for digest cases.
5. PR association requires the approved image file and exact full-reference replacements against the frozen base. An explicitly configured generated Markdown fingerprint file may also change, limited to one SHA-256 `Source digest` line and regular non-executable file mode. The trusted controller helper runs as a literal, shell-free native post-upgrade task; repository scripts remain disabled. The UI discloses the artifact before submission. The artifact is compared with the current PR base, while image content remains bound to the approved frozen base. Replace complete old references, not tag substrings or prefixes. Matching source occurrence count must agree. When an identical reference is also owned by another manager, Kubernetes tracking reconstructs only the pinned engine's anchored `image:` values, including embedded manifests. Every selected occurrence must change to the approved complete reference, and every other byte must stay identical. Other manager selectors and mixed image-volume references remain fail-closed. Mixed old digests, extra edits and ambiguous encodings are rejected.
6. Retain exact repository/base/branch/author/URL/head association and conservative Checks/status aggregation. Missing PR or orphan branch becomes `needs_attention` without automatic replay. A late verified PR may reconcile read-only. Merges remain manual.
7. A changed base can still race the final pre-write base check, as documented by the current contract. Preserve post-write base/diff verification and operator review for ambiguous external effects. Do not broaden this acknowledged race into permission to choose an unapproved digest.

The UI confirmation and retained receipt must distinguish a digest-only change from a tag upgrade, displaying both old/new tags and digests where present. Do not label absent tags as unknown digest values. Existing keyboard, accessibility and mobile comparison behavior must remain usable.

## Qualification and failures

| Evidence | Required result |
|---|---|
| Native selected digest plan and writer | One selected PR, exact old/new reference, dashboard disabled, manual merge |
| Tag moves or native digest differs from approval | Block before write through the qualified engine binding |
| Missing/malformed digest, inconsistent update type | Controlled rejection, no execution |
| Extra branch, candidate, source occurrence or file edit | Preflight/association rejection, no passing attribution |
| Substring collisions and multiple approved occurrences | Exact full-reference replacement only; mixed identities rejected |
| Duplicate submission, restart, missing PR, branch-only outcome | Retained receipt and no automatic replay |
| Real PR-triggered CI and read-only tracking | Exact-head Checks/status evidence, including after restart |
| Retained old tag-only records and new digest records | Decode/restore compatibility and no replay after recovery |

Run the published replacement artifact against the retained isolated GitHub repository before production promotion. Use fixture-only writer credentials and a separate read credential. Keep production writer disabled and current hosted bot unchanged during qualification. Existing tag-only regression tests and manual-merge behavior must continue to pass. An alpha.5 release/tag is never overwritten to add this capability.

## Boundaries before cutover

Digest support alone does not replace every hosted-bot manager. Homelab must separately disposition Helm, Kustomize, regex and non-image dependency automation, existing cloud PRs, scheduled backup evidence and the repository-specific cloud handover. Current partial inventory is advisory evidence, not an exhaustive cluster inventory or all-up-to-date guarantee.

## Implementation evidence

The [2026-11-03 isolated run](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/41653260099) created fixture PRs #7, #8 and #9, all open and unmerged. The unmodified native binary ran inside the immutable alpha.5 carrier; this is not a replacement published-controller qualification. The plugin's GET-only tracker subsequently verified each exact PR/head/reference diff and reported Checks unknown. No downstream PR-triggered CI claim is made.

Regression tests cover admission and duplicate approvals, missing/changed preflight tags and digests, native-rule equivalence, exact full-reference diffs and substring collisions, mixed source digests, extra edits, and copied-store recovery of old tag-only and new pinned receipts without replay. Confirmation, details and retained status show full digests; absent tags are labeled as untagged images.

Published replacement-artifact writer execution, PR-triggered CI and production promotion remain separate release qualifications. Existing no-PR and branch-without-PR outcomes retain needs-attention/no-replay behavior.
