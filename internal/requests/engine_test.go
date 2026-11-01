package requests

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEngineScopeAndPreflightRejectOtherChanges(t *testing.T) {
	c, s := setup(t)
	r, e := c.Submit(t.Context(), s)
	if e != nil {
		t.Fatal(e)
	}
	read, write := engineProfile(r, "/work/test", false), engineProfile(r, "/work/test", true)
	if read["dryRun"] != "full" || write["dryRun"] != nil {
		t.Fatal("incorrect writer mode")
	}
	read["dryRun"] = nil
	a, _ := json.Marshal(read)
	b, _ := json.Marshal(write)
	if string(a) != string(b) {
		t.Fatal("writer changed the approved scope")
	}
	force := write["force"].(map[string]any)
	for _, key := range []string{"automerge", "platformAutomerge", "dependencyDashboard", "dependencyDashboardApproval", "pruneStaleBranches"} {
		if force[key] != false {
			t.Fatal("unsafe override", key)
		}
	}
	if _, ok := force["enabled"]; ok {
		t.Fatal("global enable overrides exclusions")
	}
	if _, ok := write["checkedBranches"]; ok {
		t.Fatal("approval/check bypass enabled")
	}
	base := map[string]any{"repositories": map[string]any{r.Repository.Name: map[string]any{"branches": []any{map[string]any{"branchName": r.Branch, "upgrades": []any{map[string]any{"packageFile": r.Dependency.PackageFile, "depName": "nginx", "packageName": "nginx", "currentValue": "1.25.3", "newValue": "1.26.3"}}}}}}}
	data, _ := json.Marshal(base)
	if verifyPlan(data, r) != nil {
		t.Fatal("selected plan rejected")
	}
	for _, mutation := range []struct{ old, new string }{{"k8s/test.yaml", "k8s/other.yaml"}, {"1.26.3", "1.26.4"}, {`"packageName":"nginx"`, `"packageName":"alpine"`}, {r.Branch, "renovate/other"}} {
		wrong := strings.ReplaceAll(string(data), mutation.old, mutation.new)
		if verifyPlan([]byte(wrong), r) == nil {
			t.Fatal("unapproved plan accepted", mutation)
		}
	}
	r.Dependency.PackageName = "nginx*"
	if Supported(r.Dependency, r.Candidate) {
		t.Fatal("wildcard package treated as exact selection")
	}
	r.Dependency.PackageName = "nginx"
	r.Dependency.PackageFile = "k8s/*.yaml"
	if Supported(r.Dependency, r.Candidate) {
		t.Fatal("wildcard file treated as exact selection")
	}
}

func TestControllerEnvironmentIsDisabledByDefaultAndFailClosed(t *testing.T) {
	values := map[string]string{"UPGRADES_REPOSITORY": "test/repo", "UPGRADES_CONTROLLER_READ_BEARER_FILE": "/reader", "UPGRADES_CONTROLLER_ACTION_BEARER_FILE": "/action", "UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE": "/github", "UPGRADES_CONTROLLER_PR_AUTHOR": "test[bot]", "UPGRADES_CONTROLLER_TLS_CERT": "/tls/cert", "UPGRADES_CONTROLLER_TLS_KEY": "/tls/key", "RENOVATE_FORCE": "sensitive-sentinel"}
	c, e := Parse(values)
	if e != nil || c.WriterEnabled {
		t.Fatal("writer enabled by default", e)
	}
	for _, item := range c.commandEnvironment {
		if strings.Contains(item, "sensitive-sentinel") {
			t.Fatal("ambient Renovate environment inherited")
		}
	}
	for _, key := range []string{"UPGRADES_REPOSITORY", "UPGRADES_CONTROLLER_READ_BEARER_FILE", "UPGRADES_CONTROLLER_PR_AUTHOR"} {
		copy := map[string]string{}
		for k, v := range values {
			copy[k] = v
		}
		copy[key] = ""
		if _, e := Parse(copy); e == nil {
			t.Fatal("empty configuration accepted", key)
		}
	}
	values["UPGRADES_CONTROLLER_WRITER_ENABLED"] = "true"
	if _, e := Parse(values); e == nil {
		t.Fatal("writer enabled without dedicated credential")
	}
	values["UPGRADES_CONTROLLER_WRITER_ENABLED"] = "nonsense-sensitive-sentinel"
	if _, e := Parse(values); e == nil || strings.Contains(e.Error(), "sensitive-sentinel") {
		t.Fatal("invalid environment leaked or accepted")
	}
}
