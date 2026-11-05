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
