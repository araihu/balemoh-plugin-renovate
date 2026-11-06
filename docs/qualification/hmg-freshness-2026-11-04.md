# HMG freshness and classification qualification

HMG namespace `balemoh-hmg01` is a disposable session environment. Production collector, hosted bot, writer settings, secrets, manifests and upgrade-merge policy were not changed. No new credential or access grant was created.

The latest successful production read-only snapshot, completed 4 November 2026 at 19:19:56 UTC, replaced the initial 3 November seed. A separate run of the updated read-only collector then scanned actual GitHub main with the existing dedicated collector read credential. The native engine remained pinned to Renovate 44.197.0, forced full dry-run, scripts/plugins disabled and internal-host access blocked. The temporary local credential copy was removed after success.

The classified snapshot is run `aaf211b1-2b16-4853-bed1-a21fdeab8734`, completed at 2026-11-04 19:59:25 UTC, for commit `436d932cd531851dfcb45f3a47329afce5036f75`. It contains 182 groups from 210 explicit source records: 39 candidate groups, 115 groups with no candidates reported, 16 lookup warnings, 12 exclusions and no conflicting declarations. Eight blank Helm overrides and one unresolved Dockerfile build argument are explained outside the image counts. There are no remaining unassigned in-scope source records. These observations do not establish that empty results are up to date.

The HMG ConfigMap and plugin package were updated in place; API/UI containers reloaded while the controller and its request storage were retained. The existing controller successfully serves the new snapshot with TLS and bearer authentication, proving continued snapshot-v1 consumption. Production and HMG writers remain disabled.

Regression coverage includes disabled-only versus mixed active groups, inherited versus explicit Helm values, digest-only references, genuine malformed and unsafe-path records, warnings on blank overrides and unchanged empty-result uncertainty. Race tests, vet, package build and browser list/detail/coverage checks cover the change.

HMG still uses an explicitly refreshed snapshot ConfigMap. This session update installs no new recurring collection schedule; it will become stale again without a later refresh. Sixteen real lookup warnings remain, including nine missing-tag references covered by unmerged homelab PR #847. No merge, production rollout or registry-access expansion was performed.
