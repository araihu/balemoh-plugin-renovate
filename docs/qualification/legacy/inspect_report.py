#!/usr/bin/env python3
"""Inspect Renovate 44.197.0 reports without exposing raw report/log content.

This is a PoC evidence reader, not the production snapshot adapter. In
particular, an empty update array is counted as an observation, not a proof
that a dependency has no eligible update.
"""

from __future__ import annotations

import argparse
from collections import Counter, defaultdict
import hashlib
import json
from pathlib import Path


def inspect(report_path: Path, controlled: bool) -> dict:
    raw = report_path.read_bytes()
    report = json.loads(raw)
    if not isinstance(report.get("repositories"), dict):
        raise ValueError("Missing repository records")
    result = {
        "reportSha256": hashlib.sha256(raw).hexdigest(),
        "reportBytes": len(raw),
        "topLevelKeys": sorted(report),
        "globalProblemCount": len(report.get("problems", [])),
        "repositories": {},
    }
    for repository, record in report["repositories"].items():
        rows = []
        manager_counts = Counter()
        other_count = 0
        dependency_fields = set()
        update_fields = set()
        branches = record.get("branches", [])
        for manager, files in record.get("packageFiles", {}).items():
            for package_file in files:
                for dependency in package_file.get("deps", []):
                    if dependency.get("datasource") != "docker":
                        other_count += 1
                        continue
                    rows.append((manager, package_file["packageFile"], dependency))
                    manager_counts[manager] += 1
                    dependency_fields.update(dependency)
                    for update in dependency.get("updates", []):
                        update_fields.update(update)
        counts = Counter()
        update_types = Counter()
        identity_groups = defaultdict(list)
        for manager, package_file, dependency in rows:
            updates = dependency.get("updates", [])
            if dependency.get("skipReason") == "disabled":
                counts["excluded"] += 1
            elif dependency.get("skipReason") or dependency.get("warnings"):
                counts["unknown_or_warned"] += 1
            elif updates:
                counts["updates_observed"] += 1
            else:
                counts["empty_updates_observed"] += 1
            for update in updates:
                update_types[update.get("updateType", "unknown")] += 1
            key = (manager, package_file, dependency.get("packageName"), dependency.get("depType"))
            identity_groups[key].append(dependency)
        summary = {
            "recordKeys": sorted(record),
            "status": record.get("status"),
            "problemCount": len(record.get("problems", [])),
            "dockerDependencies": len(rows),
            "otherDependencies": other_count,
            "dockerManagerCounts": dict(sorted(manager_counts.items())),
            "observations": dict(sorted(counts.items())),
            "updateTypes": dict(sorted(update_types.items())),
            "candidateEntries": sum(len(d.get("updates", [])) for _, _, d in rows),
            "pendingCheckEntries": sum(bool(u.get("pendingChecks")) for _, _, d in rows for u in d.get("updates", [])),
            "ociChartRecords": sum(manager == "argocd" for manager, _, _ in rows),
            "dependencyFields": sorted(dependency_fields),
            "updateFields": sorted(update_fields),
            "branches": len(branches),
            "branchesWithPR": sum(branch.get("prNo") is not None for branch in branches),
            "branchFields": sorted({key for branch in branches for key in branch}),
            "branchResults": dict(sorted(Counter(str(branch.get("result")) for branch in branches).items())),
            "ambiguousIdentityGroups": sum(len(items) > 1 for items in identity_groups.values()),
            "hasSourceCommitField": any("sha" in key.lower() or "commit" in key.lower() for key in record),
        }
        if controlled:
            summary["controlledCases"] = controlled_cases(rows, branches)
        result["repositories"][repository] = summary
    return result


def controlled_cases(rows: list, branches: list) -> dict:
    def select(name: str, package_file: str | None = None) -> dict:
        matches = [d for _, file, d in rows if d.get("depName") == name and (package_file is None or file == package_file)]
        if len(matches) != 1:
            raise ValueError(f"Fixture selection is ambiguous: {name}")
        return matches[0]

    disabled = select("registry.example.invalid/disabled")
    failed = select("ghcr.io/renovate-report-poc-missing/not-a-real-image")
    fixed = select("alpine")
    digest = select("nginx", "k8s/digest.yaml")
    age = select("docker.io/library/alpine", "k8s/age.yaml")
    grouped = [d for _, file, d in rows if file in {"compose.yaml", "k8s/deployment.yaml"} and d.get("depName") in {"nginx", "docker.io/library/nginx"}]
    cases = {
        "disabled": {"skipReason": disabled.get("skipReason"), "updates": len(disabled.get("updates", []))},
        "lookupFailure": {"skipReason": failed.get("skipReason"), "warnings": len(failed.get("warnings", [])), "updates": len(failed.get("updates", [])), "hasCurrentVersion": "currentVersion" in failed},
        "fixedVersion": {"warnings": len(fixed.get("warnings", [])), "updates": len(fixed.get("updates", [])), "currentVersion": fixed.get("currentVersion"), "fixedVersion": fixed.get("fixedVersion")},
        "digestChange": [{key: u.get(key) for key in ("updateType", "newValue", "newDigest")} for u in digest.get("updates", [])],
        "releaseAge": {"dependencyKeys": sorted(age), "updates": age.get("updates", [])},
        "groupedBranches": sorted({u["branchName"] for d in grouped for u in d.get("updates", []) if "branchName" in u}),
        "localPlatformBranchRecords": len(branches),
    }
    assert disabled.get("skipReason") == "disabled" and not disabled.get("updates")
    assert failed.get("warnings") and not failed.get("updates") and "currentVersion" not in failed
    assert not fixed.get("warnings") and not fixed.get("updates") and fixed.get("currentVersion") == "3.19.0"
    assert any(u.get("updateType") == "digest" and u.get("newDigest") for u in digest.get("updates", []))
    assert cases["groupedBranches"] == ["renovate/poc-nginx"]
    assert any(u.get("pendingChecks") is True for u in age.get("updates", []))
    assert {manager for manager, _, _ in rows} == {"docker-compose", "dockerfile", "helm-values", "kubernetes", "kustomize", "regex"}
    duplicates = [d for _, file, d in rows if file == "k8s/duplicates.yaml" and d.get("depName") == "nginx"]
    assert len(duplicates) == 2 and len({d["currentValue"] for d in duplicates}) == 2
    assert all(not {"containerName", "namespace", "resourceName"}.intersection(d) for d in duplicates)
    cases["duplicateDeclarations"] = {"records": len(duplicates), "distinctCurrentValues": len({d["currentValue"] for d in duplicates}), "resourceLocatorPresent": False}
    return cases


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report", type=Path)
    parser.add_argument("--controlled", action="store_true")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    output = json.dumps(inspect(args.report, args.controlled), indent=2) + "\n"
    if args.output:
        args.output.write_text(output)
    else:
        print(output, end="")


if __name__ == "__main__":
    main()
