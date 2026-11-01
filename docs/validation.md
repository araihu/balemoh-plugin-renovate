# Validation receipt

Local pre-release implementation, updated 2026-10-31. These checks are not a production rollout or independent release audit.

## Executed

- `make test`: generated templ, SDK schemas and environment docs; `go test -race ./...`; `go vet ./...`.
- `go test -tags container ./internal/runner -run TestPinnedEngineProfile -count=1`: the actual generated self-hosted profile passed Renovate 44.197.0's strict config validator in a container with networking disabled. This found and fixed `internalHostAccess`, whose supported blocking value is `block`.
- SDK manifest/package validation for separate `dist/package/`. Linux amd64/arm64 and macOS arm64 UI/backend builds, Linux amd64/arm64 producer/server builds. No local SDK replacement.
- Multi-platform OCI archive built from the pinned Renovate image without a registry push. The SDK's own package-info program ran inside the Linux arm64 container and returned exactly the same manifest/package digests as the local package. Collector/server binaries are outside the installed package, so unrelated runtime artifacts cannot change its content digest.
- Real API/BFF host built from a Git archive of `4a6b6492183015eed717be0c0104b6951ca10fed` rendered current/detail/coverage pages and the summary widget from a controlled TLS fixture. Repeated page/coverage/widget reads added zero source collection calls. Source and host credentials and the private source URL were absent from public responses.
- Browser checks exercised search, the Unassessed selector, empty filters, detail/back navigation and missing-dependency recovery. A Minor filter returned six fixture groups and omitted Major/pinDigest proposals without altering retained data. Escape returned focus to the composite filter trigger. Search for nginx returned three fixture groups; Unassessed returned three; a nonmatching search returned an explicit filtered-empty state.
- List and detail captured at 390 px and 1440 px, Goshtoso and Minimal, light and dark. The actual global presentation state of the isolated fixture API was changed for these tests; BFF fallback environment values alone do not override stored global appearance. All 16 route/theme/mode/width combinations had page width equal to viewport width. These historical checks measured the document only. The independent review subsequently found internal main/table overflow on mobile; the correction pass below replaces that mobile layout. The final 14080 run had no plugin console errors. Historical host notification-poll errors from the retired 14082 fixture were observed during teardown and were excluded from the final-run result.
- Service presentation checks added exact mapping stability across version changes and no matching by equal image/file names. UI tests cover image and sprite icons, name-only presentation, HTML escaping, denied/unavailable/missing catalogs, foreign/duplicate identities, remote image URLs and incomplete sprites. All preserve image data on presentation failure.
- The isolated real API created the fixture service `Edge proxy`. One exact Compose dependency mapping plus one explicit BFF service-ID grant displayed its current name and bundled Goshtoso icon in list/detail/widget. Of three nginx groups, only the mapped group carried service presentation. The homepage widget was tested with the pinDigest filter to include that group. Mapped list/detail repeated all 16 theme/mode/width combinations without page overflow. The host selfh.st cache was empty in this fixture, so its remote-backed image asset was unavailable; the final visual check uses a bundled sprite. Image-URL support is covered by render/security tests, not a populated real-host selfh.st cache.
- Home-lab collector and full optional host overlay rendered with Kustomize. The focused validator verified suspended collection, endpoint replicas zero, no public route, immutable image pins, no GitHub credential on host/server and unchanged preexisting Alertmanager entries. The active base still excludes this plugin.

Screenshots and the private process receipt live in the task's visualization/development directories. Fixture execution metadata is simulated and the UI explicitly states that it is not production inventory. No GitHub credential or registry query is used by this host fixture.

The frozen host archive replaced its development SDK module substitutions with published `v0.0.1-alpha.3` in both API and BFF modules. Host Goshtoso remains the source pin `v0.4.6-0.20261027195056-cea8387a3c88`; the plugin compiles against the alpha.2 author baseline `v0.4.4`. No host implementation source was changed. This is an explicit development compatibility tuple, not a claim about a published host image.

## Boundaries

Linux arm64 executed the package verifier and pinned engine config validator. Linux amd64 was cross-compiled and included in the OCI archive; native amd64 execution is not established. macOS arm64 ran the real API/BFF/plugin process journey.

Producer safety, snapshot validation and source retention have focused automated checks. The real scheduled GitHub scan with a dedicated read-only credential, actual checkout/report/publication cycle, cluster NFS access/lock behavior, TLS rotation and NetworkPolicy enforcement remain rollout gates. No normal writer run or bot migration was attempted.

Browser checks do not establish full accessibility certification. An axe scan, JavaScript-disabled composite-filter behavior and a real browser transport interruption have not been completed. The independent review and correction pass are recorded below. Backend failure/timeout handling is separate from those browser gates. Retroactive service binding was not exercised on a real host. The package notification default remains disabled; the inactive home-lab overlay prepares host publication enabled. Production inbox delivery remains unverified.


## Candidate and security notification receipt

Adapter `0.0.3` and the optional publication feature were checked against the same frozen API/BFF and public SDK tuple.

- `make test` passed, including race tests/vet. Positive findings notify under partial coverage; startup and recovery remain silent. Tests cover unchanged/reappearing candidates, later first security evidence, separate immutable event identities, retry payload stability, warning priority under the 64-message queue bound, opt-in/grant absence and excluded/unknown groups.
- `go test -tags container ./internal/runner -run TestPinnedEngineSecurityReport -count=1` passed. A synthetic explicit dependency-update flag and a branch cache went through Renovate 44.197.0's actual extraction writer and `runBranchSummary` projection in the pinned image with networking disabled, then through the Go adapter. The flag survived serialization and exact association. This is schema/adapter compatibility evidence, not a real vulnerability lookup.
- `go run ./cmd/dev-host -notifications -listen 127.0.0.1:14081` used an isolated SQLite database and private operator token. The initial partial snapshot omitted one candidate and produced zero notifications. The next normal backend poll received that candidate with synthetic explicit dependency-update `isVulnerabilityAlert=true` evidence and published exactly one warning through the real SDK callback. The host retained its exact service ID and plugin destination. Repeated unchanged polls kept one inbox record.
- The browser showed the warning in the real tray, with the service icon, and followed its link to the flagged candidate's detail. The detail explicitly says that CVE/severity are not established. No notification was created through an external-publisher API or directly inserted into SQLite.

The prepared home-lab overlay now opts in to host inbox publication. It remains inactive with the collector suspended and endpoint at zero replicas. No image was pushed or cluster state changed. Native image package parity was rechecked after the build. Docker CVE detection, real production security-flag availability and production notification delivery remain unverified.

Security list/detail visual checks covered 390/1440 px in Goshtoso/Minimal, light/dark, all 16 combinations without page overflow. The visible badge used Goshtoso's public warning badge. The viewport was reset after testing, and the isolated preview returned to Goshtoso dark. Its notification remained one warning after repeated collection polls.


## Independent-review correction pass, 2026-10-30

Adapter 0.0.3 joins PR/policy/branch result using only fields actually serialized by the pinned branch pipeline, with a unique extracted-identity owner and unique branch upgrade. Security comes only from the explicit dependency-update flag. The SDK notification queue retains frozen messages after retryable `resource_exhausted` and across source failure/stale/recovery; terminal `capacity_exceeded` still drops them. Loss of source continuity resets the silent baseline, not already admitted messages. Security titles identify the Renovate flag and the body starts with the CVE/severity limitation.

`make test`, including race/vet, the offline pinned-engine security-report test and the original independent external-overlay reproductions passed after the corrections. Added regressions cover each native identity mismatch, busy callback retry and canonical URLs, retained search/filter/cursor context and invalid-cursor recovery. Native ARM64 package parity passed for package `09bc95b3fa42c5c9693398f3c510075ea9c0cef93bfcb70f6b01862a0c05712b`.

The current UI uses native GET filters and contextual links. At 390 px the visible public Panel composition shows paired Git/candidate values and human policy/workflow labels; the desktop public Table is hidden. Browser measurements found main client/scroll width 375/375 and comparison container 303/303. Search submission updated the canonical URL; reset cleared search; detail/back and coverage/back retained `compose.yaml`. Full digests remain in exact-reference evidence. The controlled fixture is explicitly labelled and is not production inventory.

The previous 16-combination captures are historical and are not a certification of this new responsive layout. Fresh independent review and final visual receipt are stored with this correction pass. No active bot configuration or cluster resource was changed.

Final visual review used two isolated Impeccable assessments plus Checklist Design. Nielsen score remained 32/40, with no confirmed P1 after the reset label correction and rejection of a keyboard-test false positive. Two P2 presentation items remain: mobile density/first-result distance and compact reconciliation of source/group coverage counts. The root verified actual keyboard selection in all three filters and canonical submission. Reviewers separately inspected the final top-alignment CSS and raw 390px screenshot; no further production code was changed. Final CSS-inclusive identity is `46cef33af7ca7ca327d7ea814d037ff0dafa6ad728bafcb316a4f1754d7958e5`.

Final source-only detector returned zero findings for unchanged `pages.templ`; this is not accessibility certification. Both reviewers inspected live UI earlier; final narrow follow-ups used source and raw screenshots after their IAB runtimes became unavailable. Root confirmed final live navigation, warning and responsive measurements. No mutable detector overlay was injected because CUA evaluate is read-only. Full assistive-technology testing, production credentials/collection/delivery, native AMD64 execution and cluster rollout remain unverified.

## Filter layout follow-up, 2026-10-30

Search and Assessment are direct children of one single-column filter region. Update type and Workflow share one two-column child. Public Goshtoso Toolbar RootClass/FiltersClass hooks compose this layout without changing shared components. Fixed filter widths and the intermediate side-by-side arrangement were removed.

Scoped UI tests and package generation passed. Live browser checks at 390, 825 and 1440 px confirmed the direct-child structure, one filter column, two nested columns and equal main client/scroll widths. Viewport override was reset. This narrow layout follow-up does not repeat the prior independent code/design review.

Current package: `f7abce5ebf6daf120d72c6a101976495dfdba8b8b49a91cce4859cafac2d0417`. Local OCI index: `sha256:8cb6bdfa139ac5ff1906c6f791ea892c9c1172bab2bb6d16fed9bbcefe5d0f86`. The prepared overlay remains inactive, collector suspended, server at zero replicas. No active bot or cluster change.

## Coverage presentation follow-up, 2026-10-30

Removed the two repeated observation paragraphs and the final sentence of the partial-coverage alert. A Collection summary now reconciles source records with grouped images and displays assessed/unassessed/excluded totals and collection limitations. Diagnostic commit, branch, policy fingerprint, engine version/image and scope remain available in a closed-by-default public Goshtoso Accordion. The same evidence composition is used in image detail.

Scoped UI tests and package generation passed. The real API/BFF preview verified Enter opens and closes Technical details with matching aria-expanded/visible content. Checks at 390, 760 and 1440 px found no horizontal main overflow; removed paragraphs and alert suffix are absent. Viewport override was reset. This narrow presentation follow-up does not repeat the prior independent review.

Current package: `4f9ef7a0186a59ceb7dcaf58c63480f035972001f81887ecb0461249d17ac7cc`. Local OCI index: `sha256:34935ee86b23b8b250584f72397ce9a8b195af0f83942ffcc0f1f59d1debf67c`. Inactive overlay pins were refreshed. No active bot configuration or cluster resource changed.

## Declaration source icons, 2026-10-30

Image/file rows now show the host's curated Docker or Kubernetes icon based solely on the normalized Renovate manager. Dockerfile/Compose use Docker; Kubernetes/Helm values/Kustomize use Kubernetes. Custom/unknown managers omit it regardless of filename. Source presentation does not establish runtime or service identity. Goshtoso Icon/Button/Tooltip reuse existing same-origin host assets/attributions; no host-internal import, grant expansion or client script was added.

Tooltip triggers have explicit accessible names, decorative SVGs and 44px targets. Viewport-clamped tooltips open on keyboard focus and dismiss with Escape. Desktop/card IDs differ. Rendering tests cover all supported managers, contradictory filenames, unknown/custom fallback and unique IDs. Live 390/760/1440px checks confirmed both icon families, accessible descriptions, target sizes, no duplicate IDs and no horizontal main overflow. Viewport override was reset. The two removed observation paragraphs are absent from the list; the equivalent desktop table caption suffix was removed too.

Current package: `2bc876d5bf863391d6ac2d3d050d75690eb92f600a0484a447748dea589a5600`. Local OCI index: `sha256:9f8973445a023b12b989034e87f3cd6c1c9ab05ef4a5fd416131bb751c014fe4`. Prepared pins refreshed; collector remains suspended and endpoint at zero replicas. No bot, active host or cluster mutation. This focused UI follow-up does not repeat the earlier independent review.

## Detail source icon follow-up, 2026-10-30

Detail reuses the list's SourceType tooltip/icon through the public PageHeader TitleSuffix slot. The control stays outside h1, preserving the image's accessible title, while the source filename remains in the description. Manager inference, known-source icons and unknown-manager fallback are shared.

Rendering tests cover both responsive list variants and detail across all supported/custom/unknown managers. Live detail checks at 390/760/1440px confirmed a 44px accessible trigger outside h1, tooltip/description, unique IDs and no main overflow. Keyboard focus opens the tooltip; viewport override was reset.

Current package: `b4c89ff586f0fae415f70c53e3afca7c058e1420f4a4eba471e9ac6d558f18a8`. Local OCI index: `sha256:44da34664f18a441c9693f15e9280dd7783c5c82a09d440c39078c24cea82cdc`. Prepared pins updated. No active bot/host configuration or cluster change; no new independent review for this focused UI change.

## Detail icon placement follow-up, 2026-10-30

The icon now sits immediately before the filename, below the image title. It no longer uses TitleSuffix. Detail and list reuse SourceFile, including the manager tooltip and accessible 44px trigger. The pinned PageHeader supports only plain description text, so an app-owned heading group composes its public title/actions header and shared source row under one separator.

Scoped UI tests passed. Live checks at 390/655/1440px verified icon-before-file order, vertical alignment in the same row, no title icon and no horizontal main overflow. Keyboard tooltip still opens and viewport override was reset. Current package: `16000a31b836517077513e3d43a04bfefdff3201027354a2cad245b7cd6ea7c2`. Local OCI index: `sha256:9f3c960304b5809835e427dcf10c254ab4a3af14f4502ccc21b223bd114d12fb`. Prepared pins refreshed; no active bot/host or cluster change. No new independent review for this focused placement change.

## UI review correction pass, 2026-10-31

Both P2 priority issues and the P3 hierarchy observation from the independent UI review were corrected. This is implementation verification, not a new independent review or heuristic rescore. Detail preserves warnings first, renders candidates before repeated observation/extraction metadata, and keeps the DOM order aligned with reading order. At 390×844 the first security candidate comparison moved from approximately y1025 to y637, ending at y715.375.

Desktop header remained at y88 after 908px of host content scrolling. No nested scroll region was added. Specific workflow/policy summaries use weight700; the explicit unknown fallback retains 14px regular-weight text with its existing foreground. The public Table API gap and bounded semantic CSS fallback are in goshtoso-snag-journal.md.

`go test ./internal/view ./internal/pluginui` passed after final templ generation. Package and multi-platform OCI were rebuilt; Linux arm64 package-info returned exactly the current manifest/package digests. Browser checks found no main overflow on list at390/655/825/1024/1440, detail at390/655/1440, and coverage at390/1440. Exactly one table/card layout was visible. Filtered detail/back retained compose.yaml and assessment; reset restored defaults. Tooltip focus opened, Escape was separately observed closing it, and Enter toggled technical evidence. Viewport override was reset and preview kept running.

Evidence: image-upgrades-review-fixes-2026-10-31 contains final screenshots, checks.json and sanitized host receipt. Current package: aae4063227f750e9e581fc63e37e51879803bf7f75c9088e430a2c7901a2ae12. Local OCI index: sha256:428779870d7cd8e229925f959298b7372c71488eaf9a9822d3036fe41283314b. Prepared overlays remain inactive, CronJob suspended and endpoint at zero replicas. No image push, active bot/host change or cluster mutation. Physical touch, screen reader, light-theme revalidation, stale/unavailable transitions and independent rescore were not repeated in this focused pass.


## Cold Kubernetes startup correction, 2026-11-01

The first cluster run failed closed in preparation before repository access. With an empty tmpfs, the pinned image's containerbase wrapper writes tool initialization messages to stdout before the version. Exact comparison of the wrapper output therefore rejected a valid installed engine. Version validation now invokes the installed Renovate Node entry point directly; the scan still uses the image wrapper and the fixed full dry-run profile.

`TestCollectorColdReadOnlyImage` builds the real collector and runs it in the pinned image with network disabled, a read-only root, empty tmpfs work/tmp/snapshots and an inert fake token. It must pass preparation and fail in provenance, proving a cold image reaches the network boundary without loosening version validation. The test, race suite and vet passed locally on ARM64. Native AMD64 is covered by the existing container-test CI job.


## Digest-only reference correction, 2026-11-01

The real repository report exposed empty currentValue strings on digest-only references. The strict snapshot schema correctly rejects empty tags, so normalization now emits null for that absent display tag while retaining the native empty value for exact branch/PR joins. The new synthetic regression failed before the fix, then passed with digest and PR association retained. The previously captured private image-scope report now normalizes to 190 source records and 158 groups. Raw private reports are not committed. Race/vet passed; final cluster publication still requires the corrected image.
