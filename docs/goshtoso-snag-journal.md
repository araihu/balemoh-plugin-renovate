# Goshtoso consumer friction

## 2026-10-31: Table header positioning

Pinned version: v0.4.4. Inspected public `table.Config`, `Column`, `Table` example and implementation. `RootClass` supports application layout; no sticky-header option or header class slot exists. The default horizontal-overflow wrapper also establishes the sticky scroll ancestor.

Current workaround keeps the public Table and its semantics. An application RootClass releases overflow on desktop, and one scoped selector positions the semantic thead against the existing host content scroller. No shared typography, spacing, color, DOM, script or dependency is replaced. Mobile remains on public Panel cards. Replace this CSS when Table provides a public sticky-header hook. No upstream issue or publication was sent.

Closing checkpoint: no new control, dependency, credential or host patch. Runtime proof: header stayed at y88 after 908px of host content scroll at1440×1000. No main overflow across390/655/825/1024/1440. Public-hook gap remains recorded; local workaround verified.
