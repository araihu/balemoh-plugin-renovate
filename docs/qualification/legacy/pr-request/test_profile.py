import copy
import unittest

from profile import REPOSITORY, profile
from verify_report import verify


class ProfileTests(unittest.TestCase):
    def test_isolation_and_manual_merge(self):
        for phase in ["discover", "observe", "write"]:
            for scope in ["kubernetes", "compose"]:
                with self.subTest(phase=phase, scope=scope):
                    p = profile(phase, scope)
                    self.assertEqual(p["repositories"], [REPOSITORY])
                    self.assertFalse(p["autodiscover"])
                    for key in ["dependencyDashboard", "dependencyDashboardApproval", "automerge", "platformAutomerge", "pruneStaleBranches"]:
                        self.assertFalse(p["force"][key])
                    self.assertNotIn("enabled", p["force"])
                    self.assertNotIn("checkedBranches", p)
                    self.assertEqual(p["dryRun"], None if phase == "write" else "full")

    def test_writer_and_observer_have_identical_selection(self):
        for scope in ["kubernetes", "compose"]:
            writer, observer = profile("write", scope), profile("observe", scope)
            writer["dryRun"] = "full"
            self.assertEqual(writer, observer)

    def test_gate_rejects_unselected_file_package_version_and_group(self):
        native = {"packageFile": "k8s/selected.yaml", "depName": "docker.io/library/nginx", "newValue": "1.26.3"}
        report = {"repositories": {REPOSITORY: {"status": "activated", "packageFiles": {"kubernetes": [{
            "packageFile": "k8s/selected.yaml", "deps": [{"depName": "docker.io/library/nginx", "updates": [{"newValue": "1.26.3"}]}]}]},
            "branches": [{"branchName": "balemoh-poc/test", "upgrades": [native]}]}}}
        self.assertEqual(verify(report, "kubernetes")["selectedUpgrades"], 1)
        for field, value in [("packageFile", "k8s/unselected.yaml"), ("depName", "docker.io/library/alpine"), ("newValue", "1.26.4")]:
            wrong = copy.deepcopy(report)
            wrong["repositories"][REPOSITORY]["branches"][0]["upgrades"][0][field] = value
            with self.subTest(field=field), self.assertRaises(AssertionError):
                verify(wrong, "kubernetes")
        wrong = copy.deepcopy(report)
        wrong["repositories"][REPOSITORY]["branches"][0]["upgrades"].append(native)
        with self.assertRaises(AssertionError):
            verify(wrong, "kubernetes")


if __name__ == "__main__":
    unittest.main()
