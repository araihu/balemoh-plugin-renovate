# Balemoh Renovate plugin: feasibility

Status: researched proposal with report PoC executed, 2026-10-29. User selected a Renovate-only solution aligned with GitOps. Disposable local containers were used; no plugin, writer migration or cluster deployment has been performed.

## Conclusion

Recommend the Renovate OSS container, pinned by digest and scheduled as a Kubernetes CronJob. One job handles repository policy, image dependency lookup, PR creation and report production after an approved migration. A version-specific adapter validates its report and publishes the normalized snapshot consumed by the advisory plugin.

The important limitation is that Renovate's machine report is experimental. Before implementing the plugin, prove that the selected release exports candidate, assessment and workflow evidence needed by the page. Empty PR or update lists cannot establish complete coverage. A failed report gate means narrowing the product or revisiting the installation, not scraping generated Markdown.

The [executed PoC](https://github.com/guilycst/home-lab/blob/0b0a6ae801053f4c0458cdf66d50ced0fb9f855c/docs/specs/spec-001-balemoh-image-upgrades/poc-results.md) confirms candidate/branch/PR/error fields in `44.197.0`. The image-only remote dry-run exited zero with 190 raw image records and 82 candidate entries. It still contained fourteen recorded problems, 21 warning/skip records and 77 empty-update records requiring conservative assessment. The data supports the advisory list; complete coverage and full notification/host behavior are not proven.

[spec-001-balemoh-image-upgrades.md](spec-001-balemoh-image-upgrades.md) defines the proposed snapshot and plugin behavior. [advisory-design.md](advisory-design.md) records installation choice, integration boundaries and writer migration. Runtime inventory backends are outside the selected scope.

## Installation comparison

| Installation | Fit for this plugin | Tradeoff |
| --- | --- | --- |
| Current Mend-hosted bot | Least infrastructure; retain it for the read-only proof. | No account-accessible report/trigger API was established. GitHub PR reads alone miss candidates that have not become PRs. |
| Renovate OSS container in Kubernetes CronJob | Recommended for one GitOps repository. Own the exact version, credentials, execution and report artifact. | Requires scheduling, report normalization and a small private artifact transport. No native long-running REST server. |
| Renovate OSS in GitHub Actions | Same engine/report contract, with native workflow-dispatch API. | Good alternative if CI should own execution. Report delivery/retention and writer migration are still needed. No Renovate workflow currently exists in this checkout. |
| Mend Renovate self-hosted Community Edition | Built-in scheduler, webhooks, stateful queue and job API. | Useful execution control, but not a complete dependency dashboard feed. Adds service state and Mend licensing without removing the report adapter. |
| Mend Renovate self-hosted Enterprise Edition | Native structured repository dashboard, dependency and update API. | Consider only if a supported dashboard API and enterprise operation justify the commercial installation. More than the current one-repository advisory journey requires. |

This is an installation recommendation, not an assertion that CLI reports have stronger compatibility guarantees than the Enterprise dashboard. Their experimental contract is the main current integration risk.

## Decisive source evidence

The [official self-hosting guide](https://docs.renovatebot.com/getting-started/running/) distributes OSS Renovate as a container/CLI and leaves scheduling to its operator. [Report configuration](https://docs.renovatebot.com/self-hosted-configuration/#reporttype) documents file/S3 output and marks reporting experimental. The proposed adapter consumes data, not log text or issue Markdown.

Mend's [self-hosted overview](https://github.com/mend/renovate-ce-ee/blob/main/docs/overview.md) documents the scheduler, queue, webhooks and administration API. The [Community OpenAPI](https://github.com/mend/renovate-ce-ee/blob/main/docs/openapi-community.yaml) includes job enqueue/status APIs and states that the `RenovateDashboard` object contains no data in Community Server. Its routes do not include the repository dashboard endpoint. The [Enterprise OpenAPI](https://github.com/mend/renovate-ce-ee/blob/main/docs/openapi-enterprise.yaml) declares `GET /api/v1/repos/{orgRepo}/-/dashboard` as Enterprise-only and returns dependencies and categorized updates. Inspecting shared schema names alone would therefore overstate CE capabilities.

The [API documentation](https://github.com/mend/renovate-ce-ee/blob/main/docs/api.md) distinguishes editions and warns that selected fields have no backward-compatibility guarantee. These API sources were inspected on `main`; a selected released image must be checked before implementation. CE/EE are distributed under [Mend terms](https://github.com/mend/renovate-ce-ee/blob/main/README.md), separately from the open-source CLI.

## Evidence baseline

| Source | Revision or scope | Finding |
| --- | --- | --- |
| Balemoh main inspected | `2b82350fbfc0e3c3c00abb03bde3f9f65a3cd499` | Backend lifecycle, retained queries, pages, widgets, scoped notification publication and service presentation. |
| SDK | Alpha.2 tag available locally after fetch | Optional service name/icon resolution; not a complete image inventory callback. |
| Balemoh PR #56 at inspection | Head `dc3b2181f7f7628762674844331e78adb2fecc7a` | Proposed alpha.3 notification `ServiceID` and optional `ServiceBinder`; release status must be rechecked before implementation. |
| Alertmanager plugin main inspected | `3fd52afd491202ea0c059d43000a7ac902a3304c` | Existing best-effort notification pattern with silent startup baseline. |
| Home-lab | HEAD `8686d3519374c355ccae6bd6d7c19b0bd717a6d1`, unrelated working changes retained | Renovate policy and upgrade runbook already exist. No live cluster or hosted account inspection. |
| Renovate report PoC | `44.197.0`, immutable index `sha256:d3aa7ee1726eb31bf0b8b006f07bcde28747a820e450cf7f4010c45913f5257f`, Linux ARM64 | Controlled extraction/lookup and remote full dry-run executed. Positive candidates and workflow fields verified; limits are recorded in the result document. |
| Mend CE/EE | Current Community and Enterprise OpenAPI, retrieved 2026-10-29 | Execution APIs exist in CE; full structured dependency dashboard is Enterprise-only. |

## Existing SDK fit

Use `backend.Handler.Configure`, `Status`, `Query` and `CloseConnection` for asynchronous report collection. Pages/widgets use retained queries and the existing shared host assets. Optional `notifications.publish/v1` supplies a connection-scoped publisher and durable host receipt. Do not add another alarm API or write the host database directly.

Optional `catalog.services.read/v1` returns granted service names/icons, not dependency inventory. Associate a Git dependency with a Balemoh service through an explicit mapping. Service names do not establish workload/image identity. Adopt PR #56's optional notification reference/binding only with a compatible SDK/host tuple; detection does not depend on it.

References: [SDK](https://github.com/araihu/balemoh/blob/2b82350fbfc0e3c3c00abb03bde3f9f65a3cd499/plugin-sdk/README.md), [backend contract](https://github.com/araihu/balemoh/blob/2b82350fbfc0e3c3c00abb03bde3f9f65a3cd499/plugin-sdk/backend/backend.go), [service presentation](https://github.com/araihu/balemoh/blob/2b82350fbfc0e3c3c00abb03bde3f9f65a3cd499/plugin-sdk/ui/services.go), [PR #56](https://github.com/araihu/balemoh/pull/56).

## Repository policy and coverage

[renovate.json](https://github.com/guilycst/home-lab/blob/0b0a6ae801053f4c0458cdf66d50ced0fb9f855c/renovate.json) enables Kubernetes, Argo CD, Helm values, Kustomize, Compose and custom regex extraction. It disables automerge, pins digests, applies release-age/PR limits and requires dashboard approval for major/digest changes. CNPG `imageName` uses the custom manager. Private registry and other selected dependencies have intentional exclusions.

Preserve these rules. Excluded means excluded, not current. Show Git image references, not asserted runtime versions. Helm chart dependencies are separate from image dependencies; chart-default images that are not extracted are outside coverage. Existing [upgrade operations](https://github.com/guilycst/home-lab/blob/0b0a6ae801053f4c0458cdf66d50ced0fb9f855c/docs/runbooks/dependency-upgrade-operations.md) remain the execution procedure.

## Proof required before implementation

The first report-contract slice is now exercised. The list below remains the broader acceptance contract; completed and unresolved parts are distinguished in [poc-results.md](https://github.com/guilycst/home-lab/blob/0b0a6ae801053f4c0458cdf66d50ced0fb9f855c/docs/specs/spec-001-balemoh-image-upgrades/poc-results.md).

1. Select one released OSS image by digest. Run it read-only against one explicit repository/base and validate actual file report output. Successful process exit is not proof that report export succeeded.
2. Exercise Kubernetes images, Helm values, Kustomize, Compose and CNPG extraction, including no-PR candidates, grouped PRs, approval/release-age deferrals, digest changes, disabled/private packages and failed lookups.
3. Establish which fields prove an assessment completed. Preserve `unknown` when the selected release cannot distinguish failed/missing lookup from no candidate.
4. Verify base SHA and non-secret policy provenance. Reject reports produced against moving/unverified base revisions. Retain operator settings/preset differences from the current writer.
5. Produce the same normalized artifact from a normal writer run in an isolated test repository. This is necessary because the intended installation uses one writer/reporting job, not a permanent second scanner.
6. Validate artifact publication failure, source staleness, scoped query denial and optional notifications on a matching real API/BFF host.
7. Recheck SDK/PR #56 release status and choose a compatible build tuple. Recheck all operational state before any authorized migration.

Passing these gates establishes an implementable advisory contract. Switching the GitOps repository's bot ownership remains a separate operational action.
