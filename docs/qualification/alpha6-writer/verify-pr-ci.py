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
changed = git("diff", "--name-only", base, head).decode().splitlines()
assert len(changed) == 1
selected = changed[0]
approved = json.loads(git("show", base + ":qualification/alpha6-cases.json"))
refs = {row["file"]: (row["before"], row["after"]) for row in approved.values()}
refs["k8s/alpha5-ci-selected.yaml"] = ("docker.io/library/nginx:1.25.3", "docker.io/library/nginx:1.26.3")
assert selected in refs
old_ref, new_ref = (value.encode() for value in refs[selected])
before, after = (git("show", commit + ":" + selected) for commit in [base, head])
assert before.count(old_ref) == 1
assert before.replace(old_ref, new_ref) == after
Path("pr-ci-receipt.json").write_text(json.dumps({"repository": repository, "pr": int(os.environ["PR_NUMBER"]), "base": base, "head": head, "event": "pull_request", "selectedFile": selected, "exactSelectedDiffVerified": True}, indent=2) + "\n")
