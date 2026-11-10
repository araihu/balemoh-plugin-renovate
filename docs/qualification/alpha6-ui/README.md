# Alpha.6 source qualification

The alpha.6 source includes approved pinned-image requests, configurable upgrade notifications, collection diagnostics/classification, coverage tables and automatic checkbox filters. Writer activation and production deployment remain separate.

## Review scope

The pinned-request review previously accepted source `49adcba44912ea977e09006e11803ea7042dcd58`. The later notification configuration was tested in disposable HMG; no separate independent notification review is claimed.

The coverage and filter review inspected the Git worktree, including untracked source, at base/HEAD `a0da22af6aab0e42a1f685c89fd106923b346348`. Its final identity was `673313f7ba70f68e353ffe9c76fd6847c8e45f35d7a286cee4d558fae8aa43f5`. The UI pass applied Checklist Design and Impeccable on desktop, 883 px and 390 px, without confirmed UI blockers. The code pass found an overlapping A/B/A request race.

The race fix was accepted at scoped identity `0b5a95b5924922687aa2e8456cf110cf373bb5354b63e0e08d194f0c0247af03`, covering `assets/filters.js`, its regression test and `docs/design.md`. Only the latest request context may affect results, navigation or completion feedback. Stale responses are cancelled at `htmx:after:request`, before pinned HTMX 4.0.0 processes history, with later swap/navigation guards retained.

The regression executes the extracted pinned response coordinator with side-effect spies. It covers A/B/A contexts, push/replace history, unsent edits, stale success/failure/abort, latest success and latest visible failure. The old implementation and the first late-guard repair both failed the relevant regression. Two Node tests pass after the correction. The independent reviewer verified the pinned lifecycle contract and request handlers without reading the producer chat, memory or private receipts.

## HMG evidence

Disposable namespace `balemoh-hmg01` ran the reviewed source package with test version alpha.5 before the release version bump. Its package digest was `f32f50d1188ca330cb45fbaa7e6dd28b86e8ac9a6e384252f192ddc38de10071`. This is a source-qualification package, not the published alpha.6 image or package pin.

A direct HTTPS read of the served filter module matched source SHA-256 `f767f7df5433f037f2970f13c7fc26cbbb2d4e4fbcd1b0dca3ba1abaebe22eaa`. Browser checks covered completed multi-selection with connected focus, search alpine/nginx/alpine, retained search focus/caret, final URL and coverage-link agreement, Back/Forward and 390 px layout without observed overflow. Idle/success feedback remained empty and the five requested list removals stayed in place.

Collection scope and classification evidence is recorded in [HMG freshness qualification](../hmg-freshness-2026-11-04.md), with empty-report counterexamples in [native report qualification](../empty-results-2026-11-04.md). HMG snapshot timestamps remain collection evidence, not a claim of current inventory.

## Publication and limits

The release workflow must pass CI and verify package/tool bytes extracted from both published Linux platform manifests. Only its successful receipt supplies alpha.6 deployment pins. Local/HMG pins must not be substituted for a rebuilt release image.

The request-race test uses the pinned response coordinator and side-effect spies, not a complete browser history implementation. HMG checks used normal requests, without deliberate network-failure or concurrent-response injection. Physical-device, screen-reader and light-theme verification are not claimed. Runtime warning causes are fixed labels rather than raw registry logs.

No current-bot setting, production manifest, writer credential mount or upgrade PR merge was changed by source qualification or release preparation. Production writer stays disabled and upgrade merges stay manual. Published alpha.6 writer qualification and production promotion require separate authorization and evidence.
