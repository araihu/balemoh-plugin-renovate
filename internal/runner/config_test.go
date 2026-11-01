package runner

import (
	"testing"
)

func TestEnvironment(t *testing.T) {
	valid := map[string]string{"UPGRADES_REPOSITORY": "example/read-only", "UPGRADES_GITHUB_TOKEN_FILE": "/credentials/token"}
	c, e := Parse(valid)
	if e != nil || c.BaseBranch != "main" || c.WorkDir != "/work" {
		t.Fatal(e)
	}
	for _, change := range []map[string]string{{}, {"UPGRADES_REPOSITORY": " "}, {"UPGRADES_GITHUB_TOKEN_FILE": ""}, {"UPGRADES_BASE_BRANCH": "--help"}, {"UPGRADES_WORK_DIR": "relative"}} {
		values := map[string]string{}
		for k, v := range valid {
			values[k] = v
		}
		if len(change) == 0 {
			delete(values, "UPGRADES_REPOSITORY")
		}
		for k, v := range change {
			values[k] = v
		}
		if _, e := Parse(values); e == nil {
			t.Fatal("invalid environment accepted")
		}
	}
}
func TestPolicyCredentialsAndExternalPresets(t *testing.T) {
	for _, policy := range []any{map[string]any{"hostRules": []any{}}, map[string]any{"extends": []any{"github>other/repo"}}, map[string]any{"registryUrl": "https://user:secret@registry.test"}, map[string]any{"password": "private"}} {
		if safePolicy(policy) {
			t.Fatal("unsafe policy accepted")
		}
	}
	if !safePolicy(map[string]any{"extends": []any{"config:recommended"}, "pinDigests": true}) {
		t.Fatal("bundled policy rejected")
	}
}

func TestChildEnvironmentDoesNotInheritBotSettings(t *testing.T) {
	c, err := Parse(map[string]string{"UPGRADES_REPOSITORY": "example/read-only", "UPGRADES_GITHUB_TOKEN_FILE": "/credentials/token", "PATH": "/usr/local/bin:/usr/bin:/bin", "RENOVATE_DRY_RUN": "false", "GITHUB_TOKEN": "private-upstream-token", "NODE_OPTIONS": "untrusted-options"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.commandEnvironment) != 1 || c.commandEnvironment[0] != "PATH=/usr/local/bin:/usr/bin:/bin" {
		t.Fatal("unexpected inherited process configuration")
	}
}
