# Archived Renovate qualification harnesses

These harnesses and controlled fixtures moved from `guilycst/home-lab` at commit `0b0a6ae801053f4c0458cdf66d50ced0fb9f855c`. Their receipts describe the original implementation and original private fixture repositories. They are not CI workflows for this public repository and must not be installed or run against production unchanged.

Current runtime commands and tests live in `cmd/app-token`, `cmd/request-backup`, `internal/tracking`, `internal/requestbackup` and `internal/requests`. Storage and token-consumer probes are opt-in tests in `internal/requests`, enabled with `recoveryproof` and `refreshproof` build tags. Kubernetes resources in this archive are historical reproduction inputs, not product defaults.

The source reports and examples below record the original advisory PoC. For reproduction of the old Python runtime, use the immutable homelab commit above. The Go replacement requires its own published-image rollout qualification.

# Renovate report PoC evidence

Executed results are described in [poc-results.md](https://github.com/guilycst/home-lab/blob/0b0a6ae801053f4c0458cdf66d50ced0fb9f855c/docs/specs/spec-001-balemoh-image-upgrades/poc-results.md). This directory contains fixtures, one actual controlled report, a read-only inspector and sanitized structural observations of the real repository run. It contains no credentials or raw private repository report/logs.

## Files

- `fixtures/`: controlled manager inputs and an isolated Renovate policy. The missing registry/image and zero digest are deliberate test values.
- `controlled-report.json`: captured `44.197.0` local/lookup output. Candidate versions/timestamps reflect the execution date; this is not live update advice.
- `inspect_report.py`: standard-library evidence reader. It performs no network request, Git write or source mutation and is not a production snapshot importer.
- `observations.json`: execution results, image/report hashes, field inventory, counts and transport safety summary. All-manager and image-manager runs have distinct labels.

Verify the captured controlled report from the repository root:

```sh
python3 docs/specs/spec-001-balemoh-image-upgrades/poc/inspect_report.py \
  docs/specs/spec-001-balemoh-image-upgrades/poc/controlled-report.json --controlled
```

## Reproduce the controlled container run

From this directory, create an output directory outside the fixture and run the exact image. Local lookup has network access to registries, but no Git platform integration and no source credentials. Fixture mount is read-only. The container itself is disposable.

```sh
mkdir -p /tmp/balemoh-renovate-report-poc-output
docker run --rm \
  --mount "type=bind,source=$PWD/fixtures,target=/workspace,readonly" \
  --mount "type=bind,source=/tmp/balemoh-renovate-report-poc-output,target=/out" \
  --workdir /workspace \
  ghcr.io/renovatebot/renovate@sha256:d3aa7ee1726eb31bf0b8b006f07bcde28747a820e450cf7f4010c45913f5257f \
  --platform=local --dry-run=lookup --onboarding=false \
  --allow-scripts=false --allow-plugins=false --binary-source=global \
  --report-type=file --report-path=/out/report.json
```

For extraction-only verification, add `--network=none` to Docker and use `--dry-run=extract`. Neither local mode proves branch/PR workflow state; those were inspected in the remote full dry-run. Future registry results can differ while the field/edge-case checks remain useful.

## Remote run boundaries

Remote execution used `dryRun=full`, one explicit repository, no onboarding, no autodiscovery and no script/plugin execution. The successful profile forced only the six reviewed image managers, disabled changelog retrieval, blocked unapproved internal hosts and bounded HTTP timeouts. The operator settings lived outside the repository; its existing bot configuration was not changed.

The safety gateway authenticated reads on the host and supplied no real GitHub credential to Renovate. It accepted parsed GraphQL query operations and Git upload-pack reads, rejecting mutations. It was stopped after the run. Its custom endpoint means native direct-GitHub authentication remains untested. The gateway is not a requirement or proposed component of the production CronJob.

Raw evidence is retained locally under the task output directory named `renovate-report-poc`, with permissions restricted to the task user. The inspection hashes allow matching those reports without committing private data. Do not reuse an ordinary write-capable token for a reproduction that claims credential-level read-only isolation.
