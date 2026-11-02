Current opt-in Go source: `internal/requests/volume_proof_test.go`. Instructions below describe the original qualified deployment.

# Opt-in cross-node request-state qualification

These resources contain synthetic data only. They are not part of the active Kustomize overlay. Both PVCs are retained; never apply this fixture over a production claim or overwrite an existing state directory.

The source is an archived copy of `araihu/balemoh-plugin-renovate` tag `v0.0.1-alpha.2`. Copy `volume_test.go` into its `internal/requests` directory as `volume_recovery_test.go`, then build with:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOWORK=off \
  go test -c -tags=recoveryproof -o recovery-proof ./internal/requests
```

`proof.yaml` uses the published plugin image and pinned Restic 0.19.1 binary. It creates one source claim/Pod and a distinct restored claim/Pod, with anti-affinity to put the restored copy on another node. It requires the prepared `renovate-plugin-backup` password and the existing backup S3 account secret. Neither Pod receives the writer token or a Kubernetes service-account token.

Create these diagnostic resources only after checking that their names are unused. Copy the test binary and `k8s/renovate-requests/backup.py` into `/work` on each Pod. Run the source binary with:

```sh
/work/recovery-proof -test.run=^TestRequestVolumeRecovery$ -test.v \
  -request-recovery-mode=serve \
  -request-recovery-directory=/data/state -request-recovery-work=/work
```

It persists three records through the actual store, tests the exclusive owner lock, leaves one unpublished temporary file, and exposes the real read-only catalog handler over loopback HTTPS. `/work/source.json` contains the local URL and credential file paths, not token values. Writer remains false.

Run `backup.py export` using those paths and `--repository test/repo --archive /work/requests.tar`. Initialize only the dedicated repository prefix once, then back up the archive through Restic stdin with filename `requests.tar` and proof tag `renovate-requests-proof-20261101`. Keep the resulting snapshot ID. Run `check --read-data`; verify that a deliberately wrong password fails. Do not use retention against the existing homelab backup root.

On the other Pod, use `restic --no-cache dump SNAPSHOT_ID /requests.tar` to a fresh archive path. Compare its SHA-256 with the original archive, then:

```sh
python3 /work/backup.py restore --repository test/repo \
  --archive /work/from-objectstore.tar --destination /data
/work/recovery-proof -test.run=^TestRequestVolumeRecovery$ -test.v \
  -request-recovery-mode=verify \
  -request-recovery-directory=/data/state -request-recovery-work=/work
```

The verify phase checks every restored checksum and private mode, native record loading, lock exclusion, interrupted-state recovery, a known PR through a local controlled GitHub fixture, zero executor/non-GET calls and disabled action admission. It writes `/work/recovery-receipt.json`. This is not a live GitHub or merge test.

Capture logs and the receipt before removing only the temporary Pods. Preserve both PVCs, the encrypted snapshot and the password recovery path. A monthly production-data drill must use a fresh claim and matching real read-only controller validation, not reseed these synthetic fixtures. See [qualification and operational limits](https://github.com/guilycst/home-lab/blob/0b0a6ae801053f4c0458cdf66d50ced0fb9f855c/docs/specs/spec-001-balemoh-image-upgrades/request-volume-recovery.md).
