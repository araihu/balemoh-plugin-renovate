#!/usr/bin/env python3
"""Qualify the shipped controller only against the retained private fixture repository."""
import base64
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import signal
import ssl
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid

REPOSITORY = "guilycst/balemoh-renovate-engine-poc-20261101"
CASES = json.loads(Path("/qualification-cases.json").read_text())
CASE = os.environ["QUALIFICATION_CASE"]
NEGATIVE = CASE == "wrong-digest"
SPEC = CASES["pinned-tag" if NEGATIVE else CASE]
SELECTED = SPEC["file"]
BINARY = "/usr/local/bin/image-upgrades-controller"


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *_args, **_kwargs):
        return None


def encode(value):
    return json.dumps(value, separators=(",", ":"), ensure_ascii=False).encode()


def identity(*values):
    return hashlib.sha256(encode(values)).hexdigest()


def run():
    repository, commit, token = (os.environ.get(k, "") for k in ("GITHUB_REPOSITORY", "GITHUB_SHA", "RENOVATE_TOKEN"))
    if repository != REPOSITORY or not re.fullmatch(r"[a-f0-9]{40}", commit) or not token:
        raise ValueError("exact isolated repository, commit and fixture-only token required")
    read_token = os.environ.get("GITHUB_READ_TOKEN", "")
    assert read_token, "separate fixture-only read token required"
    pins = json.loads(Path("/qualification-pins.json").read_text())
    assert pins["version"] == "0.0.1-alpha.6"
    assert re.fullmatch(r"ghcr.io/araihu/balemoh-plugin-renovate@sha256:[a-f0-9]{64}", pins["image"])
    github = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def api(path):
        request = urllib.request.Request("https://api.github.com/repos/" + repository + path, headers={"Authorization": "Bearer " + read_token, "Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"})
        with github.open(request, timeout=30) as response:
            return json.load(response)

    def pull_inventory():
        rows = api("/pulls?state=all&per_page=100")
        assert len(rows) < 100
        return {r["number"]: {"head": r["head"]["sha"], "state": r["state"], "mergedAt": r["merged_at"]} for r in rows}

    def branches():
        rows = api("/branches?per_page=100")
        assert len(rows) < 100
        return {row["name"]: row["commit"]["sha"] for row in rows}

    def issues():
        rows = api("/issues?state=all&per_page=100")
        assert len(rows) < 100
        return sorted(r["number"] for r in rows if "pull_request" not in r)

    before_pulls, before_branches, before_issues = pull_inventory(), branches(), issues()
    assert before_branches["main"] == commit
    root = Path(tempfile.mkdtemp(prefix="alpha6-pr-ci-"))

    def private(name, data):
        path = root / name
        with path.open("xb") as output:
            path.chmod(0o600)
            output.write(data if isinstance(data, bytes) else data.encode())
        return str(path)

    now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
    timestamp = lambda value: value.isoformat().replace("+00:00", "Z")
    repo = {"platform": "github", "name": repository, "baseBranch": "main", "commit": commit}
    key = encode([SPEC['packageName'],SPEC['depType']]).decode()
    dependency_id = identity("github", repository, "main", SPEC["manager"], SELECTED, key, SPEC["packageName"])
    new_digest = "sha256:" + "0" * 64 if NEGATIVE else SPEC["newDigest"]
    candidate_id = identity(dependency_id, SPEC["newValue"], new_digest, SPEC["updateType"])
    candidate = {"id": candidate_id, "newValue": SPEC["newValue"], "newDigest": new_digest, "updateType": SPEC["updateType"], "policy": "unknown", "workflow": "unknown"}
    dependency = {"id": dependency_id, "manager": SPEC["manager"], "packageFile": SELECTED, "dependencyKey": key, "packageName": "docker.io/library/nginx", "sourceOccurrences": 1, "currentValue": SPEC["currentValue"], "currentDigest": SPEC["currentDigest"], "assessment": "updates_found", "candidates": [candidate], "reasons": []}
    snapshot = {"schemaVersion": 1, "runId": str(uuid.uuid4()), "startedAt": timestamp(now - dt.timedelta(minutes=1)), "completedAt": timestamp(now), "producer": {"renovateVersion": "44.197.0", "renovateImage": "ghcr.io/renovatebot/renovate@sha256:d3aa7ee1726eb31bf0b8b006f07bcde28747a820e450cf7f4010c45913f5257f", "adapterVersion": "0.0.3", "policyHash": identity("alpha6-isolated-writer")}, "repository": repo, "coverage": {"scope": "Controlled published-artifact writer qualification", "status": "complete", "sourceRecords": 1, "extracted": 1, "assessed": 1, "excluded": 0, "unknown": 0, "reasons": []}, "dependencies": [dependency]}
    snapshot_bytes = encode(snapshot)
    Path("/out/snapshot-" + CASE + ".json").write_bytes(snapshot_bytes)
    selection = {"dependencyId": dependency_id, "candidateId": candidate_id, "snapshotDigest": identity(snapshot_bytes.decode())}
    reader, action = secrets.token_hex(32), secrets.token_hex(32)
    child_env = dict(os.environ)
    child_env.update({"UPGRADES_REPOSITORY": repository, "UPGRADES_BASE_BRANCH": "main", "UPGRADES_CONTROLLER_DATA_DIR": str(root / "requests"), "UPGRADES_CONTROLLER_WORK_DIR": str(root / "work"), "UPGRADES_CONTROLLER_SNAPSHOT_FILE": private("snapshot.json", snapshot_bytes), "UPGRADES_CONTROLLER_ADDRESS": "127.0.0.1:18443", "UPGRADES_CONTROLLER_READ_BEARER_FILE": private("reader", reader), "UPGRADES_CONTROLLER_ACTION_BEARER_FILE": private("action", action), "UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE": private("github-read", read_token), "UPGRADES_CONTROLLER_GITHUB_WRITE_TOKEN_FILE": private("github-write", token), "UPGRADES_CONTROLLER_PR_AUTHOR": "guilycst", "UPGRADES_CONTROLLER_WRITER_ENABLED": "true", "UPGRADES_CONTROLLER_TLS_CERT": str(root / "cert.pem"), "UPGRADES_CONTROLLER_TLS_KEY": str(root / "key.pem"), "HOME": str(root), "TMPDIR": "/tmp"})
    subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-keyout", str(root / "key.pem"), "-out", str(root / "cert.pem"), "-subj", "/CN=127.0.0.1", "-addext", "subjectAltName=IP:127.0.0.1"], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    (root / "key.pem").chmod(0o600)
    context = ssl.create_default_context(cafile=str(root / "cert.pem"))
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect(), urllib.request.HTTPSHandler(context=context))

    def call(path, bearer=reader, body=None):
        request = urllib.request.Request("https://127.0.0.1:18443" + path, data=encode(body) if body is not None else None, headers={"Authorization": "Bearer " + bearer, "Content-Type": "application/json"})
        with client.open(request, timeout=20) as response:
            raw = response.read()
            return json.loads(raw) if raw else None

    log = open(private("controller.log", b""), "ab")
    process = None

    def start():
        child = subprocess.Popen([BINARY], env=child_env, stdout=log, stderr=log)
        until = time.monotonic() + 30
        while time.monotonic() < until:
            assert child.poll() is None, "controller startup failed"
            try:
                call("/healthz")
                return child
            except (urllib.error.URLError, TimeoutError):
                time.sleep(0.2)
        child.terminate()
        child.wait(timeout=10)
        raise TimeoutError("controller startup deadline")

    try:
        process = start()
        for bearer, status in [(None, 401), (reader, 401)]:
            try:
                call("/v1/requests", bearer or "invalid", selection)
                raise AssertionError("unauthorized action accepted")
            except urllib.error.HTTPError as error:
                assert error.code == status
        first = call("/v1/requests", action, selection)
        repeated = call("/v1/requests", action, selection)
        assert first["id"] == repeated["id"]
        transitions = []
        until = time.monotonic() + 12 * 60
        while time.monotonic() < until:
            assert process.poll() is None
            catalog = call("/v1/requests")
            assert catalog["enabled"] is True and len(catalog["requests"]) == 1
            record = catalog["requests"][0]
            state = {"state": record["state"], "reason": record.get("reason", "")}
            if not transitions or transitions[-1] != state:
                transitions.append(state)
            if NEGATIVE and record["state"] == "blocked":
                assert record["reason"] == "plan_not_actionable" and not record.get("pr")
                assert pull_inventory() == before_pulls and branches() == before_branches and issues() == before_issues
                Path("/out/qualification-" + CASE + ".json").write_text(json.dumps({"qualification":"passed","case":CASE,"image":pins["image"],"sourceCommit":pins["sourceCommit"],"version":pins["version"],"reason":record["reason"],"newPRs":0,"newBranches":0,"inventoryUnchanged":True,"dashboard":False,"automerge":False},indent=2)+"\n")
                print(json.dumps({"qualification":"passed","case":CASE,"newPRs":0}),flush=True)
                return
            assert record["state"] != "blocked", "preflight blocked: " + record.get("reason", "unknown")
            if record["state"] == "open" and record.get("pr") and not record["trackingError"] and record["pr"]["checks"] == "passing":
                break
            time.sleep(1)
        else:
            raise TimeoutError("verified PR and passing CI deadline")
        assert call("/v1/requests", action, selection)["id"] == record["id"]
        tracked = record["trackedAt"]
        process.send_signal(signal.SIGTERM)
        assert process.wait(timeout=15) == 0
        process = start()
        until = time.monotonic() + 90
        while time.monotonic() < until:
            retained = call("/v1/requests")["requests"]
            assert len(retained) == 1
            retained = retained[0]
            if retained["state"] == "open" and not retained["trackingError"] and retained["pr"]["checks"] == "passing" and retained.get("trackedAt", "") > tracked:
                break
            time.sleep(1)
        else:
            raise TimeoutError("restart tracking deadline")
        assert retained["id"] == record["id"] and retained["pr"] == record["pr"]
        assert call("/v1/requests", action, selection)["id"] == record["id"]
        backup_binary = "/usr/local/bin/image-upgrades-request-backup"
        ca_file = private("ca-base64", base64.b64encode(ssl.PEM_cert_to_DER_cert((root / "cert.pem").read_text())).decode())
        archive = str(root / "requests.tar")
        exported = subprocess.check_output([backup_binary, "export", "--repository", repository, "--url", "https://127.0.0.1:18443/v1/requests", "--token-file", str(root / "reader"), "--ca-file", ca_file, "--archive", archive])
        assert json.loads(exported)["records"] == 1
        restore = root / "restored"
        restore.mkdir(mode=0o700)
        restored = subprocess.check_output([backup_binary, "restore", "--repository", repository, "--archive", archive, "--destination", str(restore)])
        assert json.loads(restored)["records"] == 1
        repeated_restore = subprocess.run([backup_binary, "restore", "--repository", repository, "--archive", archive, "--destination", str(restore)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        assert repeated_restore.returncode != 0, "retained restore target overwritten"
        prior_tracked = retained["trackedAt"]
        process.send_signal(signal.SIGTERM)
        assert process.wait(timeout=15) == 0
        child_env["UPGRADES_CONTROLLER_DATA_DIR"] = str(restore / "state")
        process = start()
        until = time.monotonic() + 90
        while time.monotonic() < until:
            loaded = call("/v1/requests")["requests"]
            assert len(loaded) == 1
            retained = loaded[0]
            if retained["id"] == record["id"] and retained["state"] == "open" and retained["pr"] == record["pr"] and not retained["trackingError"] and retained["trackedAt"] > prior_tracked:
                break
            time.sleep(1)
        else:
            raise TimeoutError("restored tracking deadline")
        Path("/out/requests-" + CASE + ".tar").write_bytes(Path(archive).read_bytes())
        pull = api("/pulls/" + str(record["pr"]["number"]))
        assert pull["state"] == "open" and pull["merged_at"] is None and pull["auto_merge"] is None
        assert pull["head"]["ref"] == record["branch"] and pull["user"]["login"] == "guilycst"
        checks = api("/commits/" + pull["head"]["sha"] + "/check-runs?per_page=100")
        statuses = api("/commits/" + pull["head"]["sha"] + "/status?per_page=100")
        expected = [c for c in checks["check_runs"] if c["name"] == "selected-image"]
        assert len(expected) == 1 and expected[0]["head_sha"] == pull["head"]["sha"]
        assert expected[0]["status"] == "completed" and expected[0]["conclusion"] == "success"
        assert expected[0]["app"]["slug"] == "github-actions"
        legacy = [s for s in statuses["statuses"] if s["context"] == "fixture/selected-image"]
        assert len(legacy) == 1 and legacy[0]["state"] == "success" and statuses["state"] == "success"
        files = api("/pulls/" + str(pull["number"]) + "/files")
        assert len(files) == 1 and files[0]["filename"] == SELECTED and files[0]["additions"] == files[0]["deletions"] == 1
        before = api("/contents/" + SELECTED + "?ref=" + commit)
        after = api("/contents/" + SELECTED + "?ref=" + pull["head"]["sha"])
        old = base64.b64decode(before["content"])
        new = base64.b64decode(after["content"])
        assert old.count(SPEC["before"].encode()) == 1
        assert old.replace(SPEC["before"].encode(), SPEC["after"].encode()) == new
        after_pulls, after_branches = pull_inventory(), branches()
        assert set(after_pulls) - set(before_pulls) == {pull["number"]}
        assert all(after_pulls[k] == v for k, v in before_pulls.items())
        assert all(after_branches[k] == v for k, v in before_branches.items())
        assert set(after_branches) - set(before_branches) == {record["branch"]}
        assert issues() == before_issues
        result = {"image": pins["image"], "sourceCommit": pins["sourceCommit"], "version": pins["version"], "controllerBinarySHA256": hashlib.sha256(Path(BINARY).read_bytes()).hexdigest(), "repository": repository, "approvedBase": commit, "selectedFile": SELECTED, "from": SPEC["before"], "to": SPEC["after"], "case": CASE, "qualification": "passed", "request": retained, "transitions": transitions, "duplicateSubmissionsSameReceipt": True, "restartRetainedReceiptAndPR": True, "newPRs": 1, "newBranches": 1, "previousPRsUnchanged": True, "previousBranchesUnchanged": True, "ordinaryIssuesUnchanged": True, "exactSelectedDiffVerified": True, "mainUnchanged": True, "automerge": False, "merged": False, "credential": "writer: fine-grained fixture-only PAT expiring 2026-11-06; reader: short-lived read-only GITHUB_TOKEN", "ciScope": "real pull_request workflow, exact-head Checks and commit status required", "fixtureScope": "controlled initial snapshot; shipped native preflight verifies live Git content"}
        result["backupRestore"] = {"records":1,"sameRequestAndPR":True,"freshTrackingAfterRestore":True,"retainedTargetRejected":True}
        result["ciEvidence"] = {"headSha": pull["head"]["sha"], "controllerChecks": retained["pr"]["checks"], "checkRun": {k: expected[0][k] for k in ["id", "name", "head_sha", "status", "conclusion", "details_url"]}, "commitStatus": {k: legacy[0][k] for k in ["id", "context", "state", "target_url"]}}
        Path("/out/qualification-" + CASE + ".json").write_text(json.dumps(result, indent=2) + "\n")
        print(json.dumps({"qualification": "passed", "pullRequest": pull["html_url"], "newPRs": 1, "merged": False}), flush=True)
    finally:
        if process is not None and process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        log.close()


if __name__ == "__main__":
    try:
        run()
    except Exception as error:
        # Only fixed assertion messages and exception types reach the public log.
        safe = str(error) if isinstance(error, (AssertionError, TimeoutError, ValueError)) else type(error).__name__
        Path("/out/failure-" + CASE + ".json").write_text(json.dumps({"qualification": "failed", "reason": safe}) + "\n")
        raise SystemExit("Writer qualification failed; see controlled receipt")
