# Goshtoso consumer friction

## 2026-10-31: Table header positioning

Pinned version: v0.4.4. Inspected public `table.Config`, `Column`, `Table` example and implementation. `RootClass` supports application layout; no sticky-header option or header class slot exists. The default horizontal-overflow wrapper also establishes the sticky scroll ancestor.

Current workaround keeps the public Table and its semantics. An application RootClass releases overflow on desktop, and one scoped selector positions the semantic thead against the existing host content scroller. No shared typography, spacing, color, DOM, script or dependency is replaced. Mobile remains on public Panel cards. Replace this CSS when Table provides a public sticky-header hook. No upstream issue or publication was sent.

Closing checkpoint: no new control, dependency, credential or host patch. Runtime proof: header stayed at y88 after 908px of host content scroll at1440×1000. No main overflow across390/655/825/1024/1440. Public-hook gap remains recorded; local workaround verified.

## 2026-11-03: Multiple filter selection

Discovery correction: fixed filters originally used single-choice Select. User requested checkbox multiselects. Inspected the pinned v0.4.4 Combobox Config, State, static-source example, implementation and keyboard tests. Reused public `Combobox` with `ModeMultiple`, `EnableClearAll` and `DisablePersistence`; no custom interactive markup, script or dependency. One app-owned empty hidden field per control preserves All in GET submissions. The public trigger has a fixed `min-w-40` with no minimum-width option. A scoped semantic `[role="combobox"]` CSS rule removes that minimum inside the existing two-column mobile filter pair. Replace when a public sizing option exists. No upstream report sent.

Closing checkpoint: full race suite, vet and package build passed. Disposable HMG verified three multiple comboboxes, Minor + Patch returning nine groups, multiple Assessment/Workflow selections, clear-to-All, reset, contextual details/coverage links, Home/Space selection, Escape focus return and 390px layout without horizontal document overflow. Existing bot and production writer remain unchanged. No new library dependency, publication or production rollout. Full screen-reader audit remains unverified.

- Auto-apply filters: composed the public `combobox:change` event with the SDK
  fragment route and HTMX 4 lifecycle in a plugin-owned module. Goshtoso owns the
  checkbox interaction; the plugin owns debounce, canonical URL and error copy.
  No missing component API or upstream report needed.

- Coverage tables: reused Table, Badge, click Tooltip and Tooltip.Help. Static
  explanations moved to README; scope/producer evidence uses the existing
  Accordion. HMG desktop/mobile help opens, dismisses with Escape, retains focus
  and stays inside the viewport. The badge info icon uses the public bundled
  Heroicons sprite, rather than the host's curated service-icon sprite. No
  component gap or upstream change needed. Removed the consumer sticky-header
  override at the user's request.

- Automatic request races: HTMX 4 processes history before `before:swap`.
  Composition now rejects obsolete contexts at public `after:request`, keeping
  later swap/navigation guards. Regression executes the pinned response
  coordinator with side-effect spies; HMG normal requests and served asset hash
  verified. No replacement control, dependency upgrade or upstream message.

- List PR controls: inspected pinned v0.4.4 Table.Row.Actions, Column,
  Link options and Badge.Config. Composed row-scoped status with native GET
  links into the existing review route. Mobile cards reuse the same eligibility
  and status components. No custom control, library gap or dependency change.

Closing checkpoint for list PR controls: full race suite, vet, package build and
candidate-target regressions passed. HMG's existing Goshtoso dark theme verified
at 1440px and 390px: direct PR navigation target, merged state/checks, keyboard
Details navigation, Back state, no horizontal overflow, unique DOM IDs and no
console errors. Read-only connection keeps requests disabled. Initial HTML
metadata and the existing 1280x640 PNG preview passed. Full theme/mode matrix,
physical-device and screen-reader/axe audits were not rerun. No merge control,
controller change, publication or production rollout.


## 2026-11-05: overview widget card width

Need: responsive aggregate widget in host-owned dashboard placement. Inspected
public v0.4.4 Card Config Body/Footer/RootClass and its usage example. Composed
Card, Badge and Link. The default vertical Card caps width at max-w-sm and exposes
no fluid-width option; a scoped RootClass max-width override lets placement own
width. No internal markup copied or dependency changed. Closing checkpoint:
aggregate domain content remains semantic; no replacement control or runtime.

## 2026-11-07: Icon actions with tooltips

Need: compact navigation actions in upgrade rows and details, preserving candidate identity and accessibility. Inspected pinned v0.4.4 Link options/target handling, Icon/Heroicons symbols, Tooltip custom triggers/portal and the existing SourceType composition. Composed these public APIs. No new component, dependency, custom interaction script or library gap.

A shared icon-link composition keeps native GET navigation, an image/file-specific accessible name and a tooltip description. Table/card/candidate scopes keep IDs unique. Review tooltips distinguish multiple candidate versions. External PR links retain noopener/noreferrer and announce the new tab. Actions have 44px minimum targets. The final PR submission retains its explicit text and existing confirmation flow.

Closing checkpoint: `make test` passed, including race tests, vet, filter/A-B-A tests and regressions for accessible names, icon-only contents, safe PR navigation and responsive IDs. Actual consumer templates in an isolated component preview passed list/detail checks at 390px and 1440px in Goshtoso/Minimal, light/dark. All 16 combinations had named icon links, matching tooltips, minimum targets and no page overflow. Keyboard focus revealed the correct tooltip; Escape dismissed it without losing focus. Details navigation worked; no browser errors were recorded. No live GitHub action, host integration/screen-reader audit, publication or deployment was performed.
