# Generated fingerprint qualification

The pinned Renovate 44.197.0 post-upgrade task ran without network access against a disposable copy of home-lab base `80dd261be98cf3fcc732fded11528a3104cf814b`. It replaced all three OpenTofu image references in `k8s/tofu-runner/workflowtemplate-tofu-apply.yaml` with the exact PR #864 target and refreshed only the source digest line in the existing inventory. The original homelab generator accepted the resulting artifact with `--check`.

The reusable regression is `TestPinnedNativeFingerprintTask`, run with `go test -tags container ./internal/requests`. It invokes the native hook and trusted helper, rejects an appended shell command and uses no GitHub credentials. Unit tests cover path confinement, modes, symlinks, ignored and untracked sources, changed image content, unexpected files and backup metadata compatibility.

This proves native hook execution and fingerprint parity. It is not a new real-GitHub writer qualification or a production rollout. All upgrade PR merges remain manual.
