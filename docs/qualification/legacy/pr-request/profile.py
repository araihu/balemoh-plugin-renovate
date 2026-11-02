#!/usr/bin/env python3
"""Generate fixed synthetic PoC profiles, never production repository input."""

import argparse
import json
from pathlib import Path

REPOSITORY = "guilycst/balemoh-renovate-engine-poc-20261101"
IMAGE = "ghcr.io/renovatebot/renovate@sha256:d3aa7ee1726eb31bf0b8b006f07bcde28747a820e450cf7f4010c45913f5257f"
SCOPES = {"kubernetes": "k8s/selected.yaml", "compose": "compose.yaml"}


def profile(phase, scope):
    config = {
        "platform": "github", "repositories": [REPOSITORY], "autodiscover": False,
        "onboarding": False, "requireConfig": "required", "configFileNames": [".balemoh-poc.json"],
        "dryRun": None if phase == "write" else "full",
        "baseDir": "/tmp/renovate-poc", "cacheDir": "/tmp/renovate-poc/cache",
        "reportType": "file", "reportPath": "/out/report.json", "reportFormatting": False,
        "allowScripts": False, "allowPlugins": False, "allowedCommands": [],
        "internalHostAccess": "block",
        "force": {
            "baseBranchPatterns": ["main"],
            "enabledManagers": ["kubernetes", "docker-compose", "dockerfile"],
            "kubernetes": {"managerFilePatterns": ["/^k8s/.+\\.yaml$/"]},
            "dependencyDashboard": False, "dependencyDashboardApproval": False,
            "prCreation": "immediate", "automerge": False, "platformAutomerge": False,
            "pruneStaleBranches": False, "fetchChangeLogs": "off", "pinDigests": False,
            "branchPrefix": "balemoh-poc/", "prHourlyLimit": 0, "prConcurrentLimit": 0,
            "packageRules": [
                {"matchPackageNames": ["*"], "enabled": False},
                {"matchPackageNames": ["docker.io/library/nginx"],
                 "matchFileNames": list(SCOPES.values()) if phase == "discover" else [SCOPES[scope]],
                 "enabled": True, "allowedVersions": "/^1\\.26\\.3$/", "groupName": None,
                 "additionalBranchPrefix": "{{packageFileDir}}-"},
            ],
        },
    }
    if phase == "discover":
        config["force"]["packageRules"] = [{
            "matchPackageNames": ["docker.io/library/nginx"], "allowedVersions": "/^1\\.26\\.3$/",
        }]
    if phase != "discover":
        config["force"]["includePaths"] = [SCOPES[scope]]
    return config


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["discover", "write", "observe"])
    parser.add_argument("scope", choices=SCOPES)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    args.output.write_text(json.dumps(profile(args.phase, args.scope), indent=2) + "\n")
