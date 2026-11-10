package requests

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRefreshFingerprintMatchesGitVisibleTree(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	git("init", "-q")
	write := func(name, content string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	artifact := "docs/inventory.md"
	before := "# Inventory\n\n- Source digest: `" + strings.Repeat("0", 64) + "`\n- Source files: 4\n"
	write(artifact, before, 0644)
	write(".gitignore", "ignored\n", 0644)
	write("image.yaml", "image: nginx:1.26.3\n", 0644)
	write("run.sh", "echo fixture\n", 0755)
	if err := os.Symlink("outside-missing-target", "link"); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	write("ignored", "ignored data", 0644)
	write("__pycache__/ignored", "ignored data", 0644)
	write(".worktrees/ignored", "ignored data", 0644)
	write("untracked.pyc", "ignored data", 0644)
	// Independently constructed byte stream includes modes and the symlink target.
	want := sha256.Sum256([]byte(".gitignore\x00100644\x00ignored\n\x00image.yaml\x00100644\x00image: nginx:1.26.3\n\x00link\x00120000\x00outside-missing-target\x00run.sh\x00100755\x00echo fixture\n\x00"))
	if err := RefreshFingerprint(artifact); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if !fingerprintOnly([]byte(before), after) || !strings.Contains(string(after), fmt.Sprintf("`%x`", want)) {
		t.Fatalf("wrong fingerprint: %s", after)
	}
	if err := RefreshFingerprint(artifact); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(artifact)
	if string(again) != string(after) {
		t.Fatal("fingerprint is not stable")
	}
	write("untracked.yaml", "image: alpine:3\n", 0644)
	if err := RefreshFingerprint(artifact); err != nil {
		t.Fatal(err)
	}
	changed, _ := os.ReadFile(artifact)
	if string(changed) == string(after) {
		t.Fatal("non-ignored untracked source omitted")
	}
	for _, name := range []string{"../escape.md", "/absolute.md", "docs/*.md", "docs/file;command.md", "docs/../escape.md", ".git/config.md", "docs//test.md", "docs/inventory.yaml"} {
		if validFingerprintPath(name) {
			t.Error("unsafe artifact path", name)
		}
	}
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../image.yaml", artifact); err != nil {
		t.Fatal(err)
	}
	if RefreshFingerprint(artifact) == nil {
		t.Fatal("symlink artifact accepted")
	}
}

func TestFingerprintChangeAllowsOnlyOneDigestLine(t *testing.T) {
	before := "# Inventory\n- Source digest: `" + strings.Repeat("a", 64) + "`\n- Source files: 4\n"
	after := strings.ReplaceAll(before, strings.Repeat("a", 64), strings.Repeat("b", 64))
	if !fingerprintOnly([]byte(before), []byte(after)) {
		t.Fatal("digest update rejected")
	}
	for _, wrong := range []string{after + "extra\n", strings.ReplaceAll(after, "files: 4", "files: 5"), after + after, strings.ReplaceAll(after, "Source digest", "Source hash"), strings.ReplaceAll(after, strings.Repeat("b", 64), "wrong")} {
		if fingerprintOnly([]byte(before), []byte(wrong)) {
			t.Fatal("unapproved content accepted")
		}
	}
}

func TestFingerprintProfileAndConfigurationAreOptIn(t *testing.T) {
	c, selection := setup(t)
	r, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	path := ".agents/skills/maintaining-homelab/references/current-inventory.md"
	for _, write := range []bool{false, true} {
		profile := engineProfile(r, "/work/test", write, path)
		command := "/usr/local/bin/image-upgrades-controller refresh-fingerprint " + path
		pattern := profile["allowedCommands"].([]string)[0]
		allowed := regexp.MustCompile(pattern)
		for _, extra := range []string{"; echo unsafe", " && echo unsafe", "\nother", "/extra"} {
			if allowed.MatchString(command + extra) {
				t.Fatal("nonliteral command admitted")
			}
		}
		if !allowed.MatchString(command) {
			t.Fatal("trusted command rejected")
		}
		tasks := profile["force"].(map[string]any)["postUpgradeTasks"].(map[string]any)
		if tasks["commands"].([]string)[0] != command || tasks["executionMode"] != "branch" || tasks["fileFilters"].([]string)[0] != path || profile["allowScripts"] != false {
			t.Fatal("unsafe artifact profile")
		}
	}
	profile := engineProfile(r, "/work/test", true, "")
	if len(profile["allowedCommands"].([]string)) != 0 || profile["force"].(map[string]any)["postUpgradeTasks"] != nil {
		t.Fatal("default profile enables commands")
	}
	values := map[string]string{"UPGRADES_REPOSITORY": "test/repo", "UPGRADES_CONTROLLER_READ_BEARER_FILE": "/reader", "UPGRADES_CONTROLLER_ACTION_BEARER_FILE": "/action", "UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE": "/read", "UPGRADES_CONTROLLER_PR_AUTHOR": "bot", "UPGRADES_CONTROLLER_TLS_CERT": "/cert", "UPGRADES_CONTROLLER_TLS_KEY": "/key"}
	for _, file := range []string{"", path, "../bad.md", "file;bad.md"} {
		values["UPGRADES_CONTROLLER_GENERATED_FINGERPRINT_FILE"] = file
		got, err := Parse(values)
		want := file == "" || file == path
		if (err == nil) != want || err == nil && got.GeneratedFingerprintFile != file {
			t.Fatal("invalid fingerprint configuration contract", file, err)
		}
	}
}
