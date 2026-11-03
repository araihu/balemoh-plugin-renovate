"""Validate only the synthetic selected file at the PR's exact head."""
import json
import os
import re
import subprocess
from pathlib import Path

repository = os.environ["CI_REPOSITORY"]
base, head = os.environ["PR_BASE"], os.environ["PR_HEAD"]
assert repository == "guilycst/balemoh-renovate-engine-poc-20261101"
assert all(re.fullmatch(r"[a-f0-9]{40}", value) for value in [base, head])
def git(*args):
    return subprocess.check_output(["git", *args])
assert git("rev-parse", "HEAD").decode().strip() == head
selected = "k8s/alpha5-ci-selected.yaml"
assert git("diff", "--name-only", base, head).decode().splitlines() == [selected]
before, after = (git("show", commit + ":" + selected) for commit in [base, head])
assert before.count(b"docker.io/library/nginx:1.25.3") == 1
assert before.replace(b"docker.io/library/nginx:1.25.3", b"docker.io/library/nginx:1.26.3") == after
assert b"docker.io/library/alpine:3.19.0" in after
Path("pr-ci-receipt.json").write_text(json.dumps({"repository": repository, "pr": int(os.environ["PR_NUMBER"]), "base": base, "head": head, "event": "pull_request", "selectedFile": selected, "exactSelectedDiffVerified": True}, indent=2) + "\n")
