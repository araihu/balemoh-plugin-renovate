# Isolated request-controller proof

This harness runs only in `guilycst/balemoh-renovate-engine-poc-20261101`. It must not target the homelab repository. `probe.go` goes under `cmd/controller-proof/main.go` in an isolated plugin source copy. `workflow.yaml` goes in the fixture repository's `.github/workflows/controller-proof.yaml`.

The fixture repository contains plugin source in `proof/plugin`, the companion unreleased SDK source in `proof/sdk`, and `proof/go.work` selecting both. That workspace explicitly replaces SDK alpha.4 with `./sdk`. Include the SDK's backend, credential, manifest, notifications, services, protocol and dependent packages. No ignored development directories, credentials or production artifacts are copied.

`k8s/controller-selected.yaml` contains `docker.io/library/nginx:1.25.3` and an unselected Alpine sidecar. Other Kubernetes files and Compose contain negative controls. The fixture repository's root Renovate policy enables configuration loading but disables all packages; the request's exact forced allow rule enables only the selected package/file. No hosted production policy is changed.

The probe creates a bounded approved snapshot with the exact native identity `docker.io/library/nginx` and empty dependency type, submits through the actual controller HTTP handler twice, and checks one durable receipt. It runs the production worker and real pinned Renovate process, then waits for the controller's GitHub association and records the observed PR. The initial snapshot is a controlled test input. The mandatory native full-dry-run verifies it against actual repository content before any write.

A private diagnostic source copy retained its preflight report under `/out` during qualification. Production controller code does not export that raw report. Failures before the final successful run created no PR. Native identity mismatches were rejected at selection_changed; correcting the test selection resolved them without relaxing production validation.

The workflow uses only its short-lived repository-scoped GITHUB_TOKEN. Actions PR creation was temporarily enabled on this isolated repository, then returned to false. The workflow is now manually disabled. Reproduction requires explicitly reenabling those test-only settings. Neither workflow schedules itself or merges PRs. GitHub does not normally run PR CI for PRs authored with GITHUB_TOKEN; this proof does not qualify production App-triggered CI.

See [receipt.json](receipt.json). It records the successful run, selected file, frozen base, exact PR head, unknown checks and manual merge state. PR #1 belongs to the earlier standalone-engine PoC; the new controller produced PR #2 only. Total two PRs is expected. There are no ordinary dashboard issues.
