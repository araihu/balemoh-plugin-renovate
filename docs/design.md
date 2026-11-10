# Renovate plugin architecture

The independent plugin uses the published `plugin-sdk v0.0.1-alpha.4` and Goshtoso `v0.4.4`. Balemoh owns its shell and shared runtime. No Balemoh internal package is imported.

The collector inherits Renovate 44.197.0 from its immutable base image. It only runs full dry-run, with automerge and scripts disabled. Each run gets an empty workspace. It compares GitHub base SHA before/after, verifies the retained checkout base branch and remote-tracking base against that SHA, reads policy from that checkout, normalizes the report, then replaces one file atomically. A shared filesystem lock serializes scheduled and manual executions of this binary. A failed or moving run cannot replace the last snapshot.

Only the reviewed image-manager forms enter normalization. Equivalent occurrences aggregate. Conflicting records become one unknown group with no current-reference claim. Unidentified records contribute raw source count and partial-coverage reasons, without invented image identities. Empty updates remain unknown in this release. No universal negative-assessment proof was established for the experimental report.

The private artifact endpoint uses TLS and a reader bearer token. It only serves GET/HEAD for one normalized file and supports ETag. Raw report/log/checkout files stay in a disposable volume. Snapshot storage uses the existing disposable RWX NFS class and is independent from Job lifetime. Lost artifacts can be regenerated; there is no history or backup promise. A missing artifact produces unavailable rather than an empty success.

Backend polling uses a ten-second timeout, a ten MiB response limit, certificate validation, optional additional trust anchor, and rejects redirects. It retains the accepted snapshot during source failures. Producer time controls age; a 304 cannot refresh producer time. SDK queries only read memory. Reconfiguration is prepared before replacing a running collector. Revision-bearing cursors prevent mixing observations.

SDK credential files currently accept bearer-token characters only. `snapshot-ca` therefore carries one base64-encoded DER trust-anchor certificate, bounded by the host's 4096-byte credential contract. The source server uses ordinary PEM key/certificate files. Certificate validation remains enabled. The optional CA is only delivered to the backend.

SDK forms currently support scalar fields and arrays of string enums. The connection uses `serviceMappingsJSON`, a bounded JSON string containing at most 256 exact dependency/service mappings and 64 distinct service IDs. The backend validates it. Host service grants remain separate. Missing catalog presentation never removes image data. Exact mapped IDs resolve through `ui.ServiceCatalog`, then `ui.ValidateServiceIdentities` checks the returned identities and local asset URLs. The list and detail compose current service name with the host icon. The overview widget uses aggregate counts. A missing icon retains the name; denied/missing/invalid catalog presentation retains image/file data. Image equality does not establish service identity, and multi-service groups stay unmapped. No discovery/catalog-inventory capability is added.

Notifications are optional and disabled by package default. Current complete or partial observations establish/admit positive `updates_found` candidates. Unrelated unknown groups do not invalidate a positive candidate, and partial coverage appears in the message. Startup and recovery after stale/unavailable data establish silent baselines. Major updates remain `info`. An explicit Renovate `isVulnerabilityAlert=true` flag produces `warning`, never inferred CVE severity. A first later security flag has its own stable event ID so it does not rewrite an ordinary notice. Regular and security seen-sets survive candidate disappearance during a session. A 64-item queue retries timeout/unavailable with frozen content; security notices can displace queued unaccepted ordinary notices, and excess admissions log only a connection ID and count. No durable outbox or cross-session replay promise exists. Service references come only from explicit mappings. Retrofitting bindings for previously published events is deferred.

Adapter `0.0.3` retains an explicit dependency-update security flag without changing dependency/candidate IDs. The pinned branch report omits manager, dependency type and security flag. PR/policy association uses the serialized tuple only when unique across extracted identities and branch upgrades; optional contradictory native fields reject the join. Missing flag is unknown. A report's security labels, PR title, aggregate branch flag or major version are not evidence. The Renovate flag may be policy-assigned; no vulnerability scanner or CVE severity is added.

## UI composition receipt

Primary task: inspect image candidates in repository files, then inspect evidence or open an associated PR. Existing Balemoh identity, compact operations list, native detail navigation, and canonical URL filters. Read-only views query retained data. Explicitly granted PR submissions use the separate request controller; merging stays manual.

| Need | Inspected public API | Decision |
| --- | --- | --- |
| Page identity | `pageheader.PageHeader`, title/actions slots | Reuse; host owns shell |
| Search | `textinput.TextInput`, search type, label, bounded value | Reuse |
| Multiple filter choices | `combobox.Combobox`, `ModeMultiple`, static options and built-in checkbox selection | Reuse in `toolbar.Toolbar`; canonical GET form submission, URL-owned state |
| Image/file groups | `table.Table`, typed columns/rows/cell components; `panel.Panel` header/actions/body/root attrs | Desktop table plus mobile panel comparison. Table Config/Column exposes no mobile/card mode in v0.4.4; CSS exposes one supported composition at a time. |
| Detail evidence | `panel.Panel`, header/body slots | Compose with domain definition lists |
| Coverage/staleness | `alert.Alert`, warning tone | Reuse |
| Security marker | `badge.Badge`, warning tone, soft appearance | Reuse beside the exact candidate in list/detail; aggregate flagged-candidate count in the overview widget |
| Service presentation | `icon.Icon`, external sprite URL/symbol, decorative mode; host `LibraryImage` composition example | Compose public sprite icon or semantic image for host image URLs; domain layout beside the name |
| Navigation and recovery | `link.Link`, `emptystate.EmptyState` | Reuse |

Only app-owned layout spacing/wrapping CSS is added. No custom JavaScript or replacement controls. Charts, Markdown rendering, icon acquisition and an additional app shell are not needed. Service icons reuse host assets; the plugin does not download or choose icons.

## Read-only invariant ledger

| State | Request | Result | Context / effect |
| --- | --- | --- | --- |
| Accepted snapshot | List/detail/summary | Retained SDK result | Zero source reads |
| Changed observation | Old cursor | `conflict` | Return to current list; zero writes |
| Missing dependency | Detail | `not_found` recovery | Current connection and filter retained |
| No snapshot | Query | `unavailable` | No zero-count claim |
| Failure after success | Query | Retained rows, `stale` | Source body stays private |
| Partial observation | Query | `partial` with unknown counts | Positive candidates can notify; unrelated unassessed groups do not |
| Unknown/invalid source response | Collection | Reject replacement | Last accepted file/observation retained |
| Source write attempt | PUT/POST | HTTP 405 | Snapshot unchanged |
| Ungranted source | GET | HTTP 401 | No content disclosure |

Development fixture envelopes are simulated and labelled as such. They prove normalization and real-host integration, not a completed production collection or credential permission proof.

The pinned engine checks out `baseBranch` before extraction in `dist/workers/repository/process/extract-update.js`. The runner verifies both local and remote-tracking base refs in the fresh checkout. It does not require final HEAD to stay on the base, since later branch processing may inspect an existing Renovate branch. No report SHA or debug-log parsing is invented.


## Review fixes, 2026-10-30

Adapter `0.0.3` requires the complete native identity/current/target tuple before transferring PR, policy, branch-result or security evidence. A transient SDK `resource_exhausted` callback retains its frozen notification for retry. Security title identifies the Renovate flag; the first body sentence states the CVE/severity limit before any long reference.

Forms and pagination use canonical page GET URLs. Filters, empty assessment selection and cursor/revision survive detail/coverage return. New filter submission resets pagination. Reset to defaults restores the candidate default while retaining repository scope. Conflict/not-found recovery drops obsolete position; invalid-filter recovery resets the query.

Responsive choice: compose public `panel.Panel` on narrow screens and `table.Table` on desktop. Only one layout is visible below/above 64rem. Each candidate in the mobile view and detail labels Git reference beside target, policy and workflow; exact digests stay in a separate evidence panel. App CSS owns layout/wrapping and 44px navigation targets, not shared component internals. Human labels separate collection freshness, coverage and observation timestamp, and explain report reason codes. No host source or bot configuration is changed.

The final correction keeps list cards compact: one Git reference beside grouped candidate proposals, with Details at the footer. Policy/workflow and exact digests remain in Detail. Desktop groups each candidate and its security caveat in a semantic list item. Details has an image/file-specific accessible name. Workflow filter uses PR-specific labels; Assessment has room for its selected value. Invalid cursor recovery drops position while retaining valid filters; invalid filters without position reset to defaults.

Retryable admitted messages remain frozen across source failure, age and recovery. They pause while observation is stale. Recovery establishes a silent baseline for new admissions and resumes delivery of existing queued events. The queue remains bounded and session-local.

Recovery validates the remaining filters through the same exported validator used by retained queries. Invalid position with valid filters preserves context; invalid filter plus position resets in one step. The pinned native policy mappings cover both approval states, schedule, pending checks and the four known PR/branch/commit limit results. The real `runBranchSummary` compatibility test exercises all eight states. Manually edited PRs are labelled as blocking Renovate changes.

## Filter layout follow-up, 2026-10-30

The filter region is a single-column grid at every viewport. Search and Assessment are direct children; Update type and Workflow share one direct child with two equal columns. Reset remains in the toolbar action region. Public Toolbar RootClass/FiltersClass hooks own this composition. Removed fixed field widths and the intermediate side-by-side search arrangement. No query, notification or collector behavior changed.

## Coverage presentation follow-up, 2026-10-30

Observation no longer repeats the Git/runtime or scope paragraphs, and partial coverage reports only the unassessed count. Coverage and image detail share a collection summary that reconciles source records with grouped images and shows assessed/unassessed/excluded totals plus actual collection limitations. Commit, base branch, policy hash, engine version/image and scope remain in a closed-by-default public Goshtoso Accordion titled Technical details. Those fields support diagnosis and reproducibility rather than routine triage. No collection, security-flag or notification behavior changes.

## Image declaration icons, 2026-10-30

Image/file rows use the normalized Renovate manager to choose source presentation: Docker for Dockerfile/Compose; Kubernetes for Kubernetes, Helm values and Kustomize. Custom extraction and unknown managers omit the icon even if the filename resembles a known manifest. These icons describe declaration format, not a discovered runtime service.

The public Goshtoso Icon, Button and Tooltip APIs reuse the host's existing curated Docker/Kubernetes symbols at /ui/icons/sprite.svg, already covered by host attributions. Tooltip triggers have explicit accessible names, decorative SVGs, 44px minimum targets, viewport-clamped portals and separate desktop/card IDs. No icon download, host-internal import, new grant or client script is added. Current compatibility remains the frozen host tuple documented above.

The detail page reuses SourceType in the public PageHeader TitleSuffix slot. Its tooltip control stays outside the h1 accessible name; the source file remains in the description. The same manager inference, accessible naming and fallback apply to list/table/card/detail. Rendering tests cover one source control in detail and two uniquely namespaced controls in responsive list markup.

The detail icon was moved from the title suffix to immediately before the source filename, reusing SourceFile from the list. PageHeader Description accepts only plain text in the pinned version, so an app-owned heading group composes the public title/actions header and shared source row with one separator. No private component markup or positioning hack is copied. Tooltip, accessible naming and 44px targets are unchanged.

## Review corrections, 2026-10-31

Detail keeps stale/partial warnings before candidate evidence, moves extraction fields after the candidates, and moves repeated observation metadata after the primary panel. DOM reading order follows visual order at all widths. List, coverage and widget retain their existing observation composition.

Desktop Table keeps its public component and adds a RootClass for the scoped sticky-header layout. The public v0.4.4 API has no sticky-header option; the narrowly scoped semantic thead selector and overflow release are recorded in goshtoso-snag-journal.md. No nested scrolling widget or copied table markup is introduced. Known workflow/policy summaries use strong text; the explicit unknown fallback remains visible at the normal text size and regular weight with the same foreground color.

## Multiple filter selections

Assessment, Update type and Workflow use Goshtoso v0.4.4 `combobox.Combobox` in client `ModeMultiple`, with its built-in checkbox options and Clear all control. An empty selection means All. Selections use OR within a field and AND across fields; update type and workflow must match the same candidate. Default assessment stays Candidates observed.

Repeated URL parameters preserve selections across list, details, coverage, pagination and request navigation. An empty hidden field preserves explicit All when no option is selected. Browser persistence is disabled because URL state owns these filters. Existing scalar query inputs remain compatible; the backend accepts bounded, validated comma-separated sets in the same string fields. Widget placement remains single-type.

## Report uncertainty and collection problems

An empty `updates` array stays `unknown` in the snapshot. The query metadata derives a repository-wide assessment breakdown from that same retained snapshot, independently of list filters and pagination. It distinguishes no candidates reported, lookup warnings/skips, conflicting declarations, other unknown statuses, candidates and policy exclusions. Only an exact sole `negative_assessment_unproven` reason receives the neutral no-candidates label; mixed or unfamiliar reasons remain unconfirmed.

The amber notice counts unknown groups other than empty reports. It also reports source records not represented by a reliable group identity, without calling those records distinct images. Other repository-level collection problems retain the warning even when no affected group can be counted. Empty reports alone do not produce an amber warning. Coverage explains every category, states that no candidates reported is not proof of being up to date, and keeps technical evidence available. List, mobile cards and details share the derived assessment labels. The existing unknown filter still matches all source-unknown groups, labelled No conclusive result. No snapshot, PR eligibility, notification admission, writer or bot behavior changes.

HMG validation: retained snapshot `052f6582-76e7-4ed7-adf6-0ff0b281ebb6` produced 31 candidate groups, 123 empty reports, 17 unconfirmed lookups, one conflicting group and ten exclusions. Nine source records were outside identified groups. The Minor filter still returned three groups with the same repository-wide breakdown. List/detail/coverage labels and the explanatory link passed live browser checks; no horizontal document overflow at390/883/1440px. Empty-report-only, mixed/future-reason and repository-problem-only cases passed automated tests. Underlying lookup/identity problems were not repaired by this presentation change.

## Explicit-image classification

Policy-disabled declarations take precedence over differing values only when every record in the group is disabled. Mixed active/disabled groups remain ambiguous, and no candidate is attributed to them. Differing disabled values are not selected as a current version.

Named Helm images with an empty tag, no digest and the native `invalid-value` skip are outside explicit-image assessment. They remain identifiable groups with `chart_inherited` and no candidates; chart or parent-value inheritance is not guessed. Empty unnamed Helm overrides and unresolved Dockerfile build arguments are extraction placeholders outside `sourceRecords` and image-group counts. Their counts are explained in the existing human-readable coverage scope. Other malformed, unsafe-path, warned or partially explicit records retain their identity warning.

Snapshot v1 fields, IDs and reason-code extensibility are retained. Empty update lists remain unknown, never proof of being up to date.

## Automatic filters

Selections and search apply after 350 ms without another edit. Goshtoso's public
`combobox:change` event drives the plugin's client module. IME composition waits
until committed. The module uses the existing SDK results fragment route and
HTMX 4 form lifecycle. Only results are replaced, keeping the combobox open,
keyboard focus, search caret and scroll position. The canonical GET form remains
the no-JavaScript fallback. URL history records the applied filters; detail and
coverage links retain them. Failed requests keep the previous results with a
retry message. Only the latest HTMX request context may swap results, update navigation, or
finish loading/error feedback. A URL comparison also rejects responses after
unsent form edits; returning to an earlier URL does not revive its older request.
Stale responses are cancelled at `htmx:after:request`, before HTMX 4 processes
history, with a second guard before swapping results.

Component decision: automatic filtering | public Combobox ClientEvent, Toolbar
slots, TextInput and SDK FragmentURL | compose | parent owns submission timing.
No host, SDK or collector changes required.

## Coverage scanability

Coverage keeps repository and collection time, a freshness hint badge and a compact
review count. Group outcomes and captured lookup causes use Goshtoso Tables with
right-aligned counts. Cause rows show the next step; click/keyboard help explains
the term without a paragraph wall. Zero conflict/unknown/exclusion rows are omitted.
Unidentified declarations remain separate from group totals. Runtime scope and
producer evidence stay under a closed accordion. Static collection semantics moved
to README documentation. List and detail warning behavior stays unchanged.

Component decisions: outcome/cause comparison | Table.Config and Cell.Component |
compose; terminology hints | Tooltip.Help and click Tooltip with Badge trigger |
reuse; secondary evidence | Accordion | reuse. Existing source components, tokens
and host shell remain. List headers scroll normally; the sticky override is removed.

## Quiet list presentation

The list omits the subtitle, repeated stale/review banners, repository-wide empty
report paragraph and idle/success filter copy. Collection time, freshness and
coverage remain in its metadata. Coverage retains the repository breakdown and
warning causes; detail/widget warnings are unchanged. The live filter status is
empty when idle or successful, visible while updating or when a request fails.

## PR controls in the image list

The desktop table adds a PR status column; mobile cards expose the same status.
Controller state takes precedence over static report links. Tracking failures
retain the PR destination with a last-known-status label and suppress passing
checks. Reported-only PRs remain explicitly unverified. Active group requests
remain visible even when a candidate filter hides their original candidate.

Row actions use Goshtoso Table's Actions slot and Link for GET navigation. They
open the verified or reported PR, view a retained request, or navigate to the
existing exact-candidate review route. Multiple candidates have distinct target
labels. Existing CanReviewAgain/CanRequest guards own stale, grant, writer,
unsupported and terminal eligibility. The list never submits the mutation.

| List state | Action | Expected destination | New writes |
|---|---|---|---:|
| fresh, granted, enabled, supported | Review PR request | matching image/candidate review, filters retained | 0 |
| stale, denied, disabled, unavailable or unsupported | review link absent | Details remains available | 0 |
| queued/running/needs review | View request | matching retained request details | 0 |
| open/draft/closed/merged PR | View PR | exact retained GitHub PR | 0 |
| tracking failure | View PR | retained PR; no passing-check claim | 0 |
| reported PR only | View reported PR | report URL, status unverified | 0 |
| blocked, same snapshot | review link absent | retained request details | 0 |
| blocked, changed snapshot | Review PR request | existing revalidation route | 0 |

No backend, controller, credential, merge or submission contract changes.

Terminal PRs use neutral View PR navigation. Merged records do not instruct
another merge; closed records point to GitHub for the closure reason. Terminal
checks are labelled last reported, and unavailable tracking cannot claim passing
checks in either the list or Details. Request review retains repository context
as well as search and update-type selections through GET, POST, Cancel and PRG.


## Upgrade overview widget

Operate mode, inheriting Balemoh's existing theme. The widget is one public
Goshtoso Card with body/footer slots. It replaces the image list with image-group
and candidate totals, a linked type breakdown, explicit Renovate security-flag
count and compact tracked-request states. No chart or nested cards are needed.
Card/Badge/Link were inspected in the pinned v0.4.4 API and composed directly.
The card's public RootClass removes its content-card width limit so placement owns
its width. Container queries adapt the type grid to narrow dashboard slots.

The existing summary query counts every matching candidate, independent of legacy
maxRows. Repository and update-type placement filters apply to every count and
list link. Type links keep repository scope. Counts by type and security flag are
candidate counts; the lead total counts image/file groups. Latest retained
requests count only exact matching dependency/candidate identities, with PR URLs
deduplicated. Tracking errors are counted separately and unavailable tracking
never appears as zero requests. PR counts cover these upgrade candidates rather
than the repository's entire PR inventory.

Zero candidates says No upgrade candidates reported. Missing data stays
unavailable; stale data is labelled as previous results. Partial coverage links
to the coverage page, and the footer retains collection time. No write or merge
action is exposed. Current bot, controller configuration and production writer
are unchanged.


Overview review corrections: type, coverage and footer links include repository
context in their accessible names, while their visible text stays concise.
Shared PR URLs use the freshest retained tracking/update/creation evidence.
Equal-time conflicting states or tracking errors count as unconfirmed. Candidate
and security totals share the 200,000-candidate output bound; image/group counts
retain their separate bound.

## Icon actions, 2026-11-07

Surface brief: existing Balemoh operations list and detail workspace, compact action density, native GET navigation. Status and image comparison keep priority; row and detail actions use familiar decorative Heroicons with Goshtoso Link and portal Tooltip. No new motion, theme or dependency. Final PR approval remains a labelled submit action.

| Need | Inspected public API | Decision |
| --- | --- | --- |
| Compact action navigation | `link.Link`, `WithTarget`, `WithAttrs`, `WithRootClass`; existing row actions slot | Compose native links with explicit accessible names and 44px minimum targets |
| Action glyphs | `icon.Icon`, pinned Heroicons eye, arrow-up, external-link, clipboard and refresh symbols | Reuse decorative bundled icons |
| Pointer and keyboard help | `tooltip.Tooltip`, `WithTrigger`, `WithPortal`; existing source-icon example | Compose unique table/card/candidate-scoped tooltip IDs and explicit descriptions |

List/card actions and detail PR/review/refresh links share the composition. Eligibility, request routes, filter preservation, read grants and manual merge behavior remain unchanged. See the consumer-friction journal for verification and limits.
