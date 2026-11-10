# Published alpha.6 writer qualification

The immutable artifact under test is `ghcr.io/araihu/balemoh-plugin-renovate@sha256:22d0c77a2d44e2dcc3776f52c210c7f3bcfbacb69902b416f460f44b95a6e467`, from source `f0108bea8b4223a7f0bb2f9f8a52d00af247737f`. The harness never rebuilds or patches the shipped controller or Renovate executable.

Passed on 5 November 2026, [run 42025837560](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/actions/runs/42025837560), frozen fixture base `c981d765e96ef95157eca15984657f151265aabb`. Selected upgrade PRs [#12](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/12), [#13](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/13) and [#14](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/14) were left unmerged. Each passed exact-head Checks and commit status, normal controller restart, logical backup/restore and tracking from the restored directory. The wrong-digest selection created no branch or PR. JSON receipts in this directory record the assertions.

## Scope

The private synthetic repository is `guilycst/balemoh-renovate-engine-poc-20261101`. Install `workflow.yaml` as `.github/workflows/alpha6-writer.yaml`, `qualify.py` as `qualification/alpha6-ci.py`, `cases.json` as `qualification/alpha6-cases.json`, and the verified release receipt as `qualification/alpha6-pins.json`. `verify-pr-ci.py` checks a fixed allowlist of complete image-reference replacements at the PR's exact head. The Actions workflow has read permissions only. Its separate writer credential is a fine-grained PAT restricted to this fixture, expiring 6 November 2026. HMG uses a separate App installation token restricted to this same repository for read-only Checks and status tracking.

The source files include unselected content and sidecars. Each successful case must create exactly one branch and unmerged PR with one exact approved file/reference change. Previous PRs, branches and ordinary issues must remain intact. Dashboard and automerge stay disabled. Duplicate submissions return the same receipt. The controller must retain its request and verified PR across restart, then export and restore through the shipped backup command, reject overwriting a retained target, and resume fresh tracking from the restored directory.

Cases cover a pinned Kubernetes tag+digest upgrade, a Dockerfile same-tag digest update, and an untagged Compose digest update. A deliberately wrong approved digest must stop at preflight with no new branch or PR. The initial snapshot is controlled; the shipped engine verifies actual Git content, frozen base, registry target and native dry-run plan before writes. This is writer qualification, not an independent collector-discovery proof.

## Fixture quota

The first run passed the wrong-digest, pinned Kubernetes and Dockerfile cases, then rejected Compose before writes. A read-only native reproduction reported `branch-limit-reached`. The fixture then set `prHourlyLimit: 10` and `prConcurrentLimit: 10` and froze a new base for the full rerun. No product or homelab policy default changed. Earlier proof PRs were left open and unmerged.

## HMG boundary

HMG is the existing session-only `balemoh-hmg01` namespace. Its real homelab connection stays read-only. A separate fixture connection and controller enable PR requests only for the private synthetic repository. The published image supplies the plugin package, controller and App token command directly. The host database and fixture requests use session-only claims so Pod replacement can be tested without discarding receipts.

No production writer mount, hosted-bot setting, production image pin or upgrade PR merge is part of this qualification. No production cutover is authorized by this receipt.

## HMG result

The published alpha.6 image is running in HMG. The real homelab connection retains its existing connection ID and stays read-only. Its fresh alpha.6 native collection completed at `2026-11-05T12:26:19.582906563Z`, with 182 groups from 210 declarations. Collection warnings remain visible in the coverage tables.

The separate fixture connection created [PR #15](https://github.com/guilycst/balemoh-renovate-engine-poc-20261101/pull/15) through the real review-and-submit browser action. GitHub verified one approved full-reference replacement in `k8s/alpha6-hmg-selected.yaml`, with the unselected Alpine sidecar unchanged. Balemoh tracked the exact head with passing Checks and commit status through a repository-only read App token. Replacing the controller Pod retained the request and PR on the session claim and resumed fresh tracking. The shipped backup command also exported and restored this HMG record into a fresh directory and rejected a retained target.

Notification checks produced Major/Minor/Patch events by default and a security-flagged Digest event despite the type filter. An ordinary Digest event was suppressed until opt-in. Enabling Digest generated no backlog. Test events were marked read, existing events preserved, the real snapshot restored and defaults reset to Major/Minor/Patch for both connections. These were controlled security flags, not real CVE findings.

Desktop and 390px browser smoke checks covered auto-apply search, checkbox type selection, source icons, exact comparisons, PR tracking and coverage tables without horizontal overflow. The list header is static. This was runtime qualification, not a new independent design review or a physical-device/accessibility audit.

The production deployment spec and generation are unchanged, with its alpha.5 image, writer disabled and writer credential unmounted. All six PRs created across the two qualification runs and HMG were left open and unmerged at qualification completion. After qualification, `guilycst` manually merged HMG PR #15 on 5 November 2026 at 13:16:08 UTC. No agent merge is claimed. The temporary Actions writer secret and local plaintext copy were removed, and the qualification workflow disabled. Fixture PR CI remains enabled for HMG. The fixture writer remains only in the HMG Secret and expires 6 November 2026. Its read-only App repository addition remains while HMG is in use. Discarding this namespace must also remove its two session claims.
