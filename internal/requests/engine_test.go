package requests

import (
	"encoding/json"
	"os"
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
	if force["statusCheckWhen"].(map[string]any)["minimumReleaseAge"] != "never" {
		t.Fatal("release-age status publication requires ungranted status write access")
	}
	for _, key := range []string{"minimumReleaseAge", "minimumReleaseAgeBehaviour", "internalChecksFilter", "ignoreTests"} {
		if _, overridden := force[key]; overridden {
			t.Fatal("repository eligibility or check policy overridden", key)
		}
	}
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
	base := map[string]any{"repositories": map[string]any{r.Repository.Name: map[string]any{"branches": []any{map[string]any{"branchName": r.Branch, "result": "done", "upgrades": []any{map[string]any{"packageFile": r.Dependency.PackageFile, "depName": "nginx", "packageName": "nginx", "currentValue": "1.25.3", "newValue": "1.26.3"}}}}}}}
	data, _ := json.Marshal(base)
	if verifyFullDryRunPlan(data, r) != nil {
		t.Fatal("selected plan rejected")
	}
	for _, result := range []string{"pr-created", "no-work"} {
		if verifyFullDryRunPlan([]byte(strings.ReplaceAll(string(data), `"result":"done"`, `"result":"`+result+`"`)), r) != nil {
			t.Fatal("selected full dry-run plan rejected", result)
		}
	}
	for _, result := range []string{"pr-limit-reached", "branch-limit-reached", "pending", "not-scheduled", "needs-approval", "needs-pr-approval", "commit-hourly-limit-reached", "error", "unknown", ""} {
		t.Run(result, func(t *testing.T) {
			deferred := strings.ReplaceAll(string(data), `"result":"done"`, `"result":"`+result+`"`)
			if verifyFullDryRunPlan([]byte(deferred), r) == nil {
				t.Fatal("non-actionable selected plan admitted", result)
			}
		})
	}
	for _, mutation := range []struct{ old, new string }{{"k8s/test.yaml", "k8s/other.yaml"}, {"1.26.3", "1.26.4"}, {`"packageName":"nginx"`, `"packageName":"alpine"`}, {r.Branch, "renovate/other"}} {
		wrong := strings.ReplaceAll(string(data), mutation.old, mutation.new)
		if verifyFullDryRunPlan([]byte(wrong), r) == nil {
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

func TestNativeFullDryRunNoWorkStillRequiresExactSelection(t *testing.T) {
	// Controlled projection from the pinned Renovate 44.197.0 alpha.4 diagnostic.
	data, err := os.ReadFile("../../docs/qualification/alpha4-writer/native-preflight-diagnostic.json")
	if err != nil {
		t.Fatal(err)
	}
	c, selection := setup(t)
	r, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	r.Repository.Name = "guilycst/balemoh-renovate-engine-poc-20261101"
	r.Branch = "balemoh-renovate/f48204ca4a67aa18325dddfe7118ce362972df078f3ce9d8f04e0d109fc4ff03"
	r.Dependency.PackageFile = "k8s/alpha4-selected.yaml"
	r.Dependency.PackageName = "docker.io/library/nginx"
	if verifyFullDryRunPlan(data, r) != nil {
		t.Fatal("native selected full dry-run no-work rejected")
	}
	for _, mutation := range []struct{ old, new string }{
		{`"branches": [`, `"problems": [{}], "branches": [`},
		{`"upgrades": [`, `"upgrades": [], "ignored": [`},
		{r.Branch, "renovate/other"},
		{r.Dependency.PackageFile, "k8s/other.yaml"},
		{r.Dependency.PackageName, "docker.io/library/alpine"},
		{"1.25.3", "1.25.4"},
		{"1.26.3", "1.26.4"},
		{`"newDigest": null`, `"newDigest": "sha256:unapproved"`},
	} {
		t.Run(mutation.new, func(t *testing.T) {
			wrong := strings.ReplaceAll(string(data), mutation.old, mutation.new)
			if wrong == string(data) || verifyFullDryRunPlan([]byte(wrong), r) == nil {
				t.Fatal("invalid no-work plan admitted")
			}
		})
	}
	r.Dependency.SourceOccurrences = 2
	if verifyFullDryRunPlan(data, r) == nil {
		t.Fatal("partial source occurrence plan admitted")
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

func TestFullDryRunTimestampOptionalWarnings(t *testing.T) {
	c, selection := setup(t)
	r, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	warnings := []map[string]any{
		{"name": "renovate", "level": 40, "msg": "Some release(s) did not have a releaseTimestamp, but as we're running with minimumReleaseAgeBehaviour=timestamp-optional, proceeding. See debug logs for more information"},
		{"name": "renovate", "level": 40, "msg": "Some upgrade(s) did not have a releaseTimestamp, but as we're running with minimumReleaseAgeBehaviour=timestamp-optional, proceeding. See debug logs for more information"},
	}
	for _, tc := range []struct {
		name     string
		problems any
		global   bool
		wantOK   bool
	}{
		{"optional release timestamp", warnings[:1], false, true},
		{"optional upgrade timestamp", warnings[1:], false, true},
		{"both optional warnings", warnings, false, true},
		{"unknown warning", []map[string]any{{"name": "renovate", "level": 40, "msg": "Package lookup failures"}}, false, false},
		{"mixed warnings", append(append([]map[string]any{}, warnings...), map[string]any{"name": "renovate", "level": 40, "msg": "Package lookup failures"}), false, false},
		{"error level", []map[string]any{{"name": "renovate", "level": 50, "msg": warnings[0]["msg"]}}, false, false},
		{"different reporter", []map[string]any{{"name": "other", "level": 40, "msg": warnings[0]["msg"]}}, false, false},
		{"changed policy", []map[string]any{{"name": "renovate", "level": 40, "msg": strings.ReplaceAll(warnings[0]["msg"].(string), "timestamp-optional", "timestamp-required")}}, false, false},
		{"malformed problem", []any{nil}, false, false},
		{"global warning", warnings, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := map[string]any{"branches": []any{map[string]any{"branchName": r.Branch, "result": "no-work", "upgrades": []any{map[string]any{"packageFile": r.Dependency.PackageFile, "packageName": r.Dependency.PackageName, "currentValue": r.Dependency.CurrentValue, "newValue": r.Candidate.NewValue}}}}}
			plan := map[string]any{"repositories": map[string]any{r.Repository.Name: repo}}
			if tc.global {
				plan["problems"] = tc.problems
			} else {
				repo["problems"] = tc.problems
			}
			data, _ := json.Marshal(plan)
			if got := verifyFullDryRunPlan(data, r) == nil; got != tc.wantOK {
				t.Fatalf("plan accepted=%v, want %v", got, tc.wantOK)
			}
			if tc.wantOK {
				wrong := strings.ReplaceAll(string(data), r.Branch, "renovate/unapproved")
				if verifyFullDryRunPlan([]byte(wrong), r) == nil {
					t.Fatal("warning exception admitted a different branch")
				}
			}
		})
	}
}
