"""Native Renovate digest selection proof, restricted to the retained private fixture."""
import base64
import hashlib
import http.server
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import threading
import urllib.parse
import urllib.request

REPOSITORY = "guilycst/balemoh-renovate-engine-poc-20261101"
IMAGE = "ghcr.io/araihu/balemoh-plugin-renovate@sha256:8a5fdb21078f1fcb64db4d4b42e9166128750df2b2e214cfeb778e1d0b52f07a"
PACKAGE = "127.0.0.1:5500/fixture/image"


def encode(value):
    return json.dumps(value, separators=(",", ":")).encode()


def digest(value):
    return "sha256:" + hashlib.sha256(value).hexdigest()


CONFIG = encode({"created": "2026-01-01T00:00:00Z", "architecture": "amd64", "os": "linux"})
MANIFESTS = {name: encode({"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json", "config": {"mediaType": "application/vnd.oci.image.config.v1+json", "digest": digest(CONFIG), "size": len(CONFIG)}, "layers": [], "annotations": {"fixture": name}}) for name in ["old", "approved", "changed"]}
OLD, APPROVED, CHANGED = (digest(MANIFESTS[name]) for name in ["old", "approved", "changed"])
state = {"target": "approved", "observed": [], "phase": "startup"}


class Registry(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_HEAD(self):
        self.do_GET()

    def do_GET(self):
        path = urllib.parse.urlsplit(self.path).path
        content_type, body = "application/json", b"{}"
        header = None
        if path.endswith("/tags/list"):
            body = encode({"name": "fixture/image", "tags": ["1.0.0", "1.0.1", "latest"]})
        elif "/manifests/" in path:
            ref = path.rsplit("/", 1)[1]
            selected = next((name for name, data in MANIFESTS.items() if digest(data) == ref), state["target"])
            body = MANIFESTS[selected]
            header = digest(body)
            content_type = "application/vnd.oci.image.manifest.v1+json"
            state["observed"].append({"ref": ref, "digest": header})
        elif "/blobs/" in path:
            body = CONFIG
        elif path.rstrip("/") != "/v2":
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Docker-Distribution-Api-Version", "registry/2.0")
        if header:
            self.send_header("Docker-Content-Digest", header)
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *_args, **_kwargs):
        return None


def run():
    repo, commit, token = (os.environ.get(key, "") for key in ["GITHUB_REPOSITORY", "GITHUB_SHA", "RENOVATE_TOKEN"])
    assert repo == REPOSITORY and re.fullmatch(r"[a-f0-9]{40}", commit) and token
    root = Path(tempfile.mkdtemp(prefix="native-digest-binding-"))
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def api(path):
        request = urllib.request.Request("https://api.github.com/repos/" + repo + path, headers={"Authorization": "Bearer " + token, "Accept": "application/vnd.github+json"})
        with client.open(request, timeout=30) as response:
            return json.load(response)

    def inventory():
        pulls, branches, issues = api("/pulls?state=all&per_page=100"), api("/branches?per_page=100"), api("/issues?state=all&per_page=100")
        assert max(len(pulls), len(branches), len(issues)) < 100
        return {p["number"]: {"head": p["head"]["sha"], "state": p["state"], "merged": p["merged_at"]} for p in pulls}, {b["name"]: b["commit"]["sha"] for b in branches}, sorted(i["number"] for i in issues if "pull_request" not in i)

    state["phase"] = "initial-inventory"
    initial = inventory()
    assert initial[1]["main"] == commit, "fixture main differs from workflow approval"
    state["phase"] = "native-version"
    assert subprocess.check_output(["/usr/local/renovate/node", "/usr/local/renovate/dist/renovate.js", "--version"], text=True).strip() == "44.197.0", "pinned native version mismatch"
    registry = http.server.ThreadingHTTPServer(("127.0.0.1", 5500), Registry)
    threading.Thread(target=registry.serve_forever, daemon=True).start()
    cases = [("tagged", "dockerfile", "digest-binding/Dockerfile", "1.0.0", "1.0.0", "digest"), ("pinned-tag", "kubernetes", "digest-binding/deployment.yaml", "1.0.0", "1.0.1", "patch"), ("untagged", "docker-compose", "digest-binding/compose.yaml", None, None, "digest")]
    results = []
    try:
        for label, manager, file, old_tag, new_tag, update_type in cases:
            branch = "balemoh-digest-binding/" + label + "-" + commit[:12]
            expected = "newDigest != '" + APPROVED + "'"
            expected += " or updateType != '" + update_type + "'"
            expected += " or " + ("$not($exists(newValue)) or newValue != '" + new_tag + "'" if new_tag else "$exists(newValue)")
            expression = "$exists(updateType) and ($not($exists(newDigest)) or " + expected + ")"

            def invoke(phase, write):
                state["phase"] = label + "/" + phase
                work = root / (label + "-" + phase)
                work.mkdir(mode=0o700)
                allow = {"matchPackageNames": ["/^" + re.escape(PACKAGE) + "$/"], "matchFileNames": [file], "enabled": True, "groupName": None, "groupSlug": None}
                if new_tag:
                    allow["allowedVersions"] = "/^" + re.escape(new_tag) + "$/"
                config = {"platform": "github", "repositories": [repo], "autodiscover": False, "onboarding": False, "requireConfig": "required", "dryRun": None if write else "full", "persistRepoData": True, "baseDir": str(work), "cacheDir": str(work / "cache"), "reportType": "file", "reportPath": str(work / "report.json"), "reportFormatting": False, "allowScripts": False, "allowPlugins": False, "allowedCommands": [], "internalHostAccess": "allow", "hostRules": [{"matchHost": "https://127.0.0.1:5500", "hostType": "docker", "insecureRegistry": True}], "force": {"automerge": False, "platformAutomerge": False, "dependencyDashboard": False, "dependencyDashboardApproval": False, "prCreation": "immediate", "prHourlyLimit": 10, "prConcurrentLimit": 10, "branchConcurrentLimit": 10, "pruneStaleBranches": False, "baseBranchPatterns": ["main"], "includePaths": [file], "enabledManagers": [manager], "kubernetes": {"managerFilePatterns": ["/^" + re.escape(file) + "$/"]}, "branchPrefix": "balemoh-digest-binding/", "branchTopic": label + "-" + commit[:12], "additionalBranchPrefix": "", "groupName": None, "groupSlug": None, "fetchChangeLogs": "off", "pinDigests": True, "packageRules": [{"matchPackageNames": ["*"], "enabled": False}, allow, {"matchPackageNames": ["/^" + re.escape(PACKAGE) + "$/"], "matchFileNames": [file], "matchJsonata": [expression], "enabled": False}]}}
                path = work / "config.json"
                path.write_bytes(encode(config))
                path.chmod(0o600)
                environment = {key: os.environ[key] for key in ["PATH", "SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS"] if key in os.environ}
                environment.update({"HOME": str(root), "TMPDIR": "/tmp", "RENOVATE_TOKEN": token, "RENOVATE_CONFIG_FILE": str(path), "LOG_LEVEL": "warn"})
                with (work / "run.log").open("xb") as log:
                    subprocess.run(["renovate"], env=environment, stdout=log, stderr=log, check=True, timeout=180)
                native = json.loads((work / "report.json").read_text())
                report = native.get("repositories", {}).get(repo, {})
                Path("/out/native-status.json").write_text(json.dumps({"phase": state["phase"], "globalProblems": [{"level": p.get("level"), "message": p.get("msg")} for p in native.get("problems", [])], "repositoryNames": list(native.get("repositories", {})), "status": report.get("status"), "problemLevels": [p.get("level") for p in report.get("problems", [])]}, indent=2)+"\n")
                assert not native.get("problems") and set(native["repositories"]) == {repo}, "native report is not a clean single-repository run"
                projection = {"case": label, "phase": phase, "status": report.get("status"), "problemCount": len(report.get("problems", [])), "branches": [{"branch": b.get("branchName"), "result": b.get("result"), "upgrades": [{key: u.get(key) for key in ["packageFile", "packageName", "currentValue", "currentDigest", "newValue", "newDigest", "updateType"]} for u in b.get("upgrades", [])]} for b in report.get("branches", [])]}
                Path("/out/progress.json").write_text(json.dumps(projection, indent=2) + "\n")
                print(json.dumps({"case": label, "phase": phase, "branches": len(report.get("branches", []))}), flush=True)
                return report

            state["target"] = "approved"
            before = inventory()
            plan = invoke("plan", False)
            assert len(plan["branches"]) == 1 and plan["branches"][0]["branchName"] == branch
            upgrades = plan["branches"][0]["upgrades"]
            assert len(upgrades) == 1 and upgrades[0]["newDigest"] == APPROVED
            assert upgrades[0].get("newValue") == new_tag
            state["target"] = "changed"
            state["observed"] = []
            denied = invoke("changed-write", True)
            assert any(r["digest"] == CHANGED and not r["ref"].startswith("sha256:") for r in state["observed"]), "changed registry target was not resolved"
            assert not denied["branches"] and inventory() == before, "changed digest caused an external effect"
            state["phase"] = label + "/approved-write"
            state["target"] = "approved"
            written = invoke("approved-write", True)
            after = inventory()
            added = set(after[0]) - set(before[0])
            assert len(added) == 1 and set(after[1]) - set(before[1]) == {branch}, "approved writer did not create exactly one selected PR/branch"
            assert all(after[0][key] == value for key, value in before[0].items())
            assert all(after[1][key] == value for key, value in before[1].items()) and after[2] == before[2]
            number = added.pop()
            pull = api("/pulls/" + str(number))
            assert pull["head"]["ref"] == branch and pull["base"]["sha"] == commit and pull["state"] == "open"
            assert pull["merged_at"] is None and pull["auto_merge"] is None
            files = api("/pulls/" + str(number) + "/files")
            assert len(files) == 1 and files[0]["filename"] == file
            def contents(ref):
                return base64.b64decode(api("/contents/" + file + "?ref=" + ref)["content"])
            old_reference = PACKAGE + (":" + old_tag if old_tag else "") + "@" + OLD
            new_reference = PACKAGE + (":" + new_tag if new_tag else "") + "@" + APPROVED
            original = contents(commit)
            assert original.count(old_reference.encode()) == 1
            assert original.replace(old_reference.encode(), new_reference.encode()) == contents(pull["head"]["sha"])
            assert written["branches"][0]["upgrades"][0]["newDigest"] == APPROVED
            results.append({"case": label, "manager": manager, "file": file, "branch": branch, "pr": number, "url": pull["html_url"], "head": pull["head"]["sha"], "preflight": {key: upgrades[0].get(key) for key in ["packageFile", "packageName", "currentValue", "currentDigest", "newValue", "newDigest", "updateType"]}, "nativeMismatchRule": expression, "changedDigestResolvedAndRejectedBeforeWrite": True, "negativeInventoryUnchanged": True, "approvedExactDiffVerified": True, "merged": False})
            Path("/out/passed-cases.json").write_text(json.dumps(results, indent=2)+"\n")
            print(json.dumps({"case": label, "passed": True, "pr": number}), flush=True)
        final = inventory()
        assert len(final[0]) == len(initial[0]) + 3 and len(final[1]) == len(initial[1]) + 3
        receipt = {"qualification": "passed", "image": IMAGE, "renovateVersion": "44.197.0", "repository": repo, "approvedBase": commit, "approvedDigest": APPROVED, "changedDigest": CHANGED, "oldDigest": OLD, "mechanism": "native post-lookup matchJsonata package rule disables mismatched updates", "registry": "controlled loopback-only OCI registry, mutable tag, fresh native cache each invocation", "internalHostAccess": "allow only inside isolated PoC; production block setting unchanged", "nativeBinaryPatched": False, "dashboard": False, "automerge": False, "newPRs": 3, "newBranches": 3, "upgradePRMerges": 0, "previousPRsAndBranchesUnchanged": True, "cases": results}
        Path("/out/qualification.json").write_text(json.dumps(receipt, indent=2) + "\n")
    finally:
        registry.shutdown()


if __name__ == "__main__":
    try:
        run()
    except Exception as error:
        safe = str(error) if isinstance(error, (AssertionError, TimeoutError)) else type(error).__name__
        Path("/out/failure.json").write_text(json.dumps({"qualification": "failed", "phase": state["phase"], "reason": safe}) + "\n")
        raise SystemExit("Native digest binding proof failed; see controlled receipt")
