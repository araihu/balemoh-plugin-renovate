#!/usr/bin/env python3
"""Fail closed unless the scoped native report contains exactly the selected upgrade."""

import argparse
import json
from pathlib import Path

from profile import REPOSITORY, SCOPES


def verify(report, scope):
    assert not report.get("problems"), "global collection problems"
    assert set(report["repositories"]) == {REPOSITORY}, "foreign repository"
    repo = report["repositories"][REPOSITORY]
    assert repo["status"] in {"activated", "onboarded"} and not repo.get("problems"), "incomplete repository"
    updates = []
    for manager, files in repo["packageFiles"].items():
        for file in files:
            assert file["packageFile"] == SCOPES[scope], "foreign file"
            for dep in file["deps"]:
                for update in dep.get("updates", []):
                    updates.append((manager, file["packageFile"], dep["depName"], update["newValue"]))
    manager = "kubernetes" if scope == "kubernetes" else "docker-compose"
    assert updates == [(manager, SCOPES[scope], "docker.io/library/nginx", "1.26.3")], updates
    assert len(repo["branches"]) == 1, "unexpected branch count"
    branch = repo["branches"][0]
    assert len(branch["upgrades"]) == 1, "hidden grouped upgrade"
    native = branch["upgrades"][0]
    assert (native["packageFile"], native["depName"], native["newValue"]) == (SCOPES[scope], "docker.io/library/nginx", "1.26.3"), native
    return {"branch": branch["branchName"], "pr": branch.get("prNo"), "selectedUpgrades": len(updates)}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report", type=Path)
    parser.add_argument("scope", choices=SCOPES)
    args = parser.parse_args()
    print(json.dumps(verify(json.loads(args.report.read_text()), args.scope)))
