# Read-only advisory implementation

The independent plugin uses the published `plugin-sdk v0.0.1-alpha.3` and Goshtoso `v0.4.4`, matching the alpha.2 UI profile. Balemoh owns its shell and shared runtime. No Balemoh internal package is imported.

The collector inherits Renovate 44.197.0 from its immutable base image. It only runs full dry-run, with automerge and scripts disabled. Each run gets an empty workspace. It compares GitHub base SHA before/after, verifies the retained checkout base branch and remote-tracking base against that SHA, reads policy from that checkout, normalizes the report, then replaces one file atomically. A shared filesystem lock serializes scheduled and manual executions of this binary. A failed or moving run cannot replace the last snapshot.

Only the reviewed image-manager forms enter normalization. Equivalent occurrences aggregate. Conflicting records become one unknown group with no current-reference claim. Unidentified records contribute raw source count and partial-coverage reasons, without invented image identities. Empty updates remain unknown in this release. No universal negative-assessment proof was established for the experimental report.

The private artifact endpoint uses TLS and a reader bearer token. It only serves GET/HEAD for one normalized file and supports ETag. Raw report/log/checkout files stay in a disposable volume. Snapshot storage uses the existing disposable RWX NFS class and is independent from Job lifetime. Lost artifacts can be regenerated; there is no history or backup promise. A missing artifact produces unavailable rather than an empty success.

Backend polling uses a ten-second timeout, a ten MiB response limit, certificate validation, optional additional trust anchor, and rejects redirects. It retains the accepted snapshot during source failures. Producer time controls age; a 304 cannot refresh producer time. SDK queries only read memory. Reconfiguration is prepared before replacing a running collector. Revision-bearing cursors prevent mixing observations.

SDK credential files currently accept bearer-token characters only. `snapshot-ca` therefore carries one base64-encoded DER trust-anchor certificate, bounded by the host's 4096-byte credential contract. The source server uses ordinary PEM key/certificate files. Certificate validation remains enabled. The optional CA is only delivered to the backend.

SDK forms currently support scalar fields and arrays of string enums. The connection uses `serviceMappingsJSON`, a bounded JSON string containing at most 256 exact dependency/service mappings and 64 distinct service IDs. The backend validates it. Host service grants remain separate. Missing catalog presentation never removes image data. Exact mapped IDs resolve through `ui.ServiceCatalog`, then `ui.ValidateServiceIdentities` checks the returned identities and local asset URLs. The list, detail and widget compose current service name with the host icon. A missing icon retains the name; denied/missing/invalid catalog presentation retains image/file data. Image equality does not establish service identity, and multi-service groups stay unmapped. No discovery/catalog-inventory capability is added.

Notifications are optional and disabled by package default. Current complete or partial observations establish/admit positive `updates_found` candidates. Unrelated unknown groups do not invalidate a positive candidate, and partial coverage appears in the message. Startup and recovery after stale/unavailable data establish silent baselines. Major updates remain `info`. An explicit Renovate `isVulnerabilityAlert=true` flag produces `warning`, never inferred CVE severity. A first later security flag has its own stable event ID so it does not rewrite an ordinary notice. Regular and security seen-sets survive candidate disappearance during a session. A 64-item queue retries timeout/unavailable with frozen content; security notices can displace queued unaccepted ordinary notices, and excess admissions log only a connection ID and count. No durable outbox or cross-session replay promise exists. Service references come only from explicit mappings. Retrofitting bindings for previously published events is deferred.

Adapter `0.0.3` retains an explicit dependency-update security flag without changing dependency/candidate IDs. The pinned branch report omits manager, dependency type and security flag. PR/policy association uses the serialized tuple only when unique across extracted identities and branch upgrades; optional contradictory native fields reject the join. Missing flag is unknown. A report's security labels, PR title, aggregate branch flag or major version are not evidence. The Renovate flag may be policy-assigned; no vulnerability scanner or CVE severity is added.

## UI composition receipt

Primary task: inspect image candidates in repository files, then inspect evidence or open an associated PR. Existing Balemoh identity, compact operations list, native detail navigation, and canonical URL filters. No action writes Git, calls a registry, or starts a scan.

| Need | Inspected public API | Decision |
| --- | --- | --- |
| Page identity | `pageheader.PageHeader`, title/actions slots | Reuse; host owns shell |
| Search | `textinput.TextInput`, search type, label, bounded value | Reuse |
| Fixed filters | `select.Select`, options, trigger, hidden submission field | Compose in `toolbar.Toolbar`; canonical GET form submission |
| Image/file groups | `table.Table`, typed columns/rows/cell components; `panel.Panel` header/actions/body/root attrs | Desktop table plus mobile panel comparison. Table Config/Column exposes no mobile/card mode in v0.4.4; CSS exposes one supported composition at a time. |
| Detail evidence | `panel.Panel`, header/body slots | Compose with domain definition lists |
| Coverage/staleness | `alert.Alert`, warning tone | Reuse |
| Security marker | `badge.Badge`, warning tone, soft appearance | Reuse beside the exact candidate in list/detail/widget |
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

The filter region is a single-column grid at every viewport. Search and Assessment are direct children; Update type and Workflow share one direct child with two equal columns. Apply/reset remain in the toolbar action region. Public Toolbar RootClass/FiltersClass hooks own this composition. Removed fixed field widths and the intermediate side-by-side search arrangement. No query, notification or collector behavior changed.

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
