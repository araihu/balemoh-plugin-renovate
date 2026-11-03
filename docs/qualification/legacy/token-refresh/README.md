Current opt-in Go source: `internal/requests/refresh_proof_test.go`. Instructions below describe the original qualified deployment.

# Isolated App refresh and restart proof

These resources are disposable and use only memory/emptyDir state. They mount the dedicated read-only App Secret, never the writer or collector PAT. They create no Service or public route. The fixed context is `admin@talos-k8s-homelab`, namespace `balemoh`, Pod `renovate-tracking-refresh-proof-20261101`. Never substitute a production Pod.

Build the opt-in `controller_test.go` inside an archived copy of plugin tag `v0.0.1-alpha.2`, under `internal/requests/token_refresh_test.go`. Use the ignored source directory `.local/renovate-request-readiness/refresh-source`, then run there:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOWORK=off \
  go test -c -tags=refreshproof -o ../controller-refresh-proof ./internal/requests
```

From the homelab repository root, first confirm the diagnostic names are unused. Generate the ConfigMap directly from the tracked runtime sources, so the fixture does not duplicate authentication code:

```sh
kubectl --context admin@talos-k8s-homelab -n balemoh create configmap \
  renovate-tracking-proof-code-20261101 \
  --from-file=k8s/renovate-requests/tracking_auth.py \
  --from-file=k8s/renovate-requests/tracking-token.py
kubectl --context admin@talos-k8s-homelab create \
  -f docs/specs/spec-001-balemoh-image-upgrades/poc/token-refresh/proof.json
```

The Pod is derived from the rendered inactive controller. Its native sidecar renews every 30 seconds. The main container waits for the diagnostic binary. Before copying, wait for the server container's `state.running.startedAt`; `Initialized` alone can precede container availability.

Choose an immutable PR head with successful Checks, write its SHA to `.local/renovate-request-readiness/refresh-head`, then copy that file to `/work/head` and the binary to `/work/controller-probe`, both in container `server`. No bearer or private-key value is needed locally. Wait for Pod Ready.

Run `python3 docs/specs/spec-001-balemoh-image-upgrades/poc/token-refresh/verify.py`. It checks ownership and absence of PVCs, then verifies renewal, sends SIGTERM only to diagnostic container processes, injects a synthetic invalid token, and recreates only this diagnostic Pod. The verifier requires `kubectl`, the local binary/head files above and access to the dedicated App Secret already installed in the cluster. It saves a credential-free receipt and logs under the ignored readiness directory.

After evidence is saved, delete only this Pod and `renovate-tracking-proof-code-20261101`. Do not delete any Secret, PVC or encrypted backup. The proof does not activate Argo paths or enable the writer. See [results and limits](https://github.com/guilycst/home-lab/blob/0b0a6ae801053f4c0458cdf66d50ced0fb9f855c/docs/specs/spec-001-balemoh-image-upgrades/token-refresh.md).
