# Alpha.7 source qualification

This release prepares the upgrades overview widget and list PR controls. It uses
SDK v0.0.1-alpha.4, Goshtoso v0.4.4 and Renovate 44.197.0. Production promotion and
writer cutover remain separate.

## Scope

The overview uses the retained summary query for all matching image/file groups
and candidates, rather than a five-row sample. Repository/type placement filters
scope its totals and destinations. Security totals count explicit Renovate flags,
not established CVEs or severity. The legacy maxRows placement option remains
accepted but does not limit aggregate counts.

PR counts cover exact current dependency/candidate identities. Shared PR URLs
use the freshest retained tracking, update or creation time. Equal-time
contradictory states and tracking errors remain unconfirmed. Unavailable tracking
never produces a zero-request claim. These are retained controller records, not
an inventory of every GitHub PR.

List controls retain existing request admission and manual merges. Repository
filters survive review, Cancel and POST redirects. Merged/closed PRs use neutral
View PR navigation and historical checks, without pending-merge instructions.

## Review corrections and source checks

Independent review found unstable shared-PR deduplication, a security-count schema
bound mismatch and ambiguous accessible link names across repository cards.
Regressions cover repeated queries and reversed input order, equal-time conflicts,
failed tracking, an accepted snapshot with 10,020 flagged candidates, and repeated
cards with repository-specific accessible names.

Release preparation runs generation, the full race suite, vet, filter-module
tests, pinned-engine container tests and package builds. Publication requires
accepted independent code/design reviews against the frozen Git source. Review
acceptance is not evidence that a newly published image has been qualified.

## HMG and limits

HMG is the disposable balemoh-hmg01 namespace at
https://hmg01-balemoh.decastro.me/. Its real homelab connection stays read-only;
the separate private-fixture writer remains scoped to that fixture. Production
writer credentials remain unmounted and the current hosted bot is unchanged.

Desktop and narrow responsive browser checks exercise the real card and type
navigation. Security, empty, unavailable and conflicting-record cases use
controlled tests when absent from HMG. Physical-device, screen-reader and
light-theme coverage are not claimed. These gaps are deferred to the
v0.1.0-candidate accessibility/platform pass, not guarantees of this alpha.

## Remaining release gates

Commit the accepted source and require CI at that exact commit. Publish through
the existing manual workflow and verify both Linux manifests, package digests and
runtime tools. Only that successful receipt supplies release deployment pins.
Local and HMG builds are source previews. Qualify the new immutable writer image
in the isolated repository before considering a writer cutover. Keep production
deployment and bot handover separately approved, with upgrade PR merges manual.
