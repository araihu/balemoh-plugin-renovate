#!/usr/bin/env python3
"""Read-only GitHub assertions for the single selected PoC PR."""

import argparse
import base64
import json
from pathlib import Path
import subprocess
from urllib.parse import quote

from profile import REPOSITORY


def api(path):
    return json.loads(subprocess.check_output(["gh", "api", f"repos/{REPOSITORY}/{path}"]))


def content(path, ref):
    record = api(f"contents/{path}?ref={quote(ref, safe='')}")
    return base64.b64decode(record["content"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("writer_artifact", type=Path)
    parser.add_argument("receipt", type=Path)
    parser.add_argument("--expected-head")
    args = parser.parse_args()
    root = args.writer_artifact
    before = (root / "base-before.txt").read_text().strip()
    assert before == (root / "base-after.txt").read_text().strip(), "base changed during run"
    assert api("git/ref/heads/main")["object"]["sha"] == before, "base changed after run"
    pulls = api("pulls?state=all&per_page=100")
    assert len(pulls) == 1, "unexpected PR count"
    pr = api(f"pulls/{pulls[0]['number']}")
    assert pr["user"]["login"] == "github-actions[bot]", "unexpected writer"
    assert pr["head"]["ref"] == "balemoh-poc/k8s-docker.io-library-nginx-1.x", "foreign branch"
    assert pr["head"]["repo"]["full_name"] == REPOSITORY, "foreign head repository"
    assert pr["base"]["ref"] == "main" and pr["base"]["repo"]["full_name"] == REPOSITORY
    assert pr["state"] == "open" and pr["merged_at"] is None and pr["auto_merge"] is None, "manual merge boundary violated"
    head = pr["head"]["sha"]
    if args.expected_head:
        assert head == args.expected_head, "repeat request changed existing PR head"
    files = api(f"pulls/{pr['number']}/files?per_page=100")
    assert len(files) == 1 and files[0]["filename"] == "k8s/selected.yaml", "foreign file changed"
    assert files[0]["additions"] == files[0]["deletions"] == 1, "unexpected change size"
    selected = content("k8s/selected.yaml", before)
    assert selected.count(b"docker.io/library/nginx:1.25.3") == 1
    assert content("k8s/selected.yaml", head) == selected.replace(b"docker.io/library/nginx:1.25.3", b"docker.io/library/nginx:1.26.3"), "wrong target or sidecar changed"
    for path in ["k8s/unselected.yaml", "compose.yaml", "Dockerfile", "renovate.json", ".balemoh-poc.json", ".github/workflows/proof.yaml", "profile.py"]:
        assert content(path, head) == content(path, before), f"unselected content changed: {path}"
    issues = api("issues?state=all&per_page=100")
    assert not [issue for issue in issues if "pull_request" not in issue], "dashboard or other issue created"
    branches = api("branches?per_page=100")
    assert {b["name"] for b in branches} == {"main", pr["head"]["ref"]}, "unexpected Git branch"
    receipt = {"repository": REPOSITORY, "pr": pr["number"], "url": pr["html_url"], "headSHA": head,
               "baseSHA": before, "state": pr["state"], "merged": False, "automerge": False,
               "changedFiles": [f["filename"] for f in files], "additions": 1, "deletions": 1,
               "pullRequests": len(pulls), "nonPrIssues": 0, "gitBranches": len(branches),
               "repeatHeadUnchanged": args.expected_head is not None}
    args.receipt.write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt))


if __name__ == "__main__":
    main()
