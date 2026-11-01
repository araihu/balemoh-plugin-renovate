//go:build container

package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPinnedEngineProfile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	folder := t.TempDir()
	if e := os.Chmod(folder, 0755); e != nil {
		t.Fatal(e)
	}
	profile, _ := profile(Config{Repository: "example/read-only", BaseBranch: "main"}, "/work")
	data, _ := json.Marshal(profile)
	path := filepath.Join(folder, "config.json")
	if e := os.WriteFile(path, data, 0644); e != nil {
		t.Fatal(e)
	}
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "--entrypoint", "renovate-config-validator", "-v", folder+":/config:ro", snapshot.RenovateImage, "--strict", "/config/config.json")
	if output, e := command.CombinedOutput(); e != nil {
		t.Fatalf("pinned engine rejected actual profile: %v\n%s", e, output)
	}
}

func TestPinnedEngineSecurityReport(t *testing.T) {
	for result, policy := range map[string]string{"needs-approval": "approval_required", "needs-pr-approval": "approval_required", "branch-limit-reached": "pr_limit", "pr-limit-reached": "pr_limit", "commit-per-run-limit-reached": "rate_limited", "commit-hourly-limit-reached": "rate_limited", "not-scheduled": "scheduled", "pending": "pending_checks"} {
		t.Run(result, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			script, err := os.ReadFile("testdata/renovate-security-report.mjs")
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "--network", "none", "--entrypoint", "node", snapshot.RenovateImage, "--input-type=module", "-", result)
			command.Stdin = bytes.NewReader(script)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("pinned report writer failed: %v\n%s", err, output)
			}
			_, data, ok := bytes.Cut(output, []byte("REPORT:"))
			if !ok {
				t.Fatal("report writer did not return report")
			}
			now := time.Now().UTC()
			commit := strings.Repeat("a", 40)
			m := snapshot.Metadata{RunID: "00000000-0000-4000-8000-000000000001", StartedAt: now.Add(-time.Minute), CompletedAt: now, Producer: snapshot.Producer{RenovateVersion: snapshot.RenovateVersion, RenovateImage: snapshot.RenovateImage, AdapterVersion: snapshot.AdapterVersion, PolicyHash: snapshot.ID("synthetic-security-report")}, Repository: snapshot.Repository{Platform: "github", Name: "example/read-only", BaseBranch: "main", Commit: commit}, DryRun: "full", CheckoutCommit: commit, BeforeCommit: commit, AfterCommit: commit}
			normalized, err := snapshot.Normalize(data, m)
			if err != nil || len(normalized.Dependencies) != 1 || !normalized.Dependencies[0].Candidates[0].IsVulnerabilityAlert {
				t.Fatal("native security flag lost by writer/adapter", err)
			}
			candidate := normalized.Dependencies[0].Candidates[0]
			if candidate.PR == nil || candidate.PR.Number != 42 || candidate.Policy != policy {
				t.Fatal("real branch projection lost PR/policy association", candidate)
			}
		})
	}
}

// An empty /tmp and read-only root match the first Kubernetes collector run.
func TestCollectorColdReadOnlyImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	folder := t.TempDir()
	if err := os.Chmod(folder, 0755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(folder, "collector")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/collector")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0", "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build collector: %v\n%s", err, output)
	}
	token := filepath.Join(folder, "token")
	if err := os.WriteFile(token, []byte(strings.Repeat("x", 40)), 0644); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "--read-only", "--user", "12021:12021", "--entrypoint", "/collector",
		"--tmpfs", "/tmp:rw,nosuid,nodev,uid=12021,gid=12021,mode=1770",
		"--tmpfs", "/work:rw,nosuid,nodev,uid=12021,gid=12021,mode=1770",
		"--tmpfs", "/snapshots:rw,nosuid,nodev,uid=12021,gid=12021,mode=1770",
		"-v", binary+":/collector:ro", "-v", token+":/token:ro",
		"-e", "HOME=/work", "-e", "TMPDIR=/tmp", "-e", "UPGRADES_REPOSITORY=example/read-only", "-e", "UPGRADES_GITHUB_TOKEN_FILE=/token", snapshot.RenovateImage)
	output, err := command.CombinedOutput()
	// Network is disabled: preparation must pass, then provenance must fail closed.
	if err == nil || !bytes.Contains(output, []byte("phase=provenance code=collection_failed")) {
		t.Fatalf("cold collector did not reach provenance: %v\n%s", err, output)
	}
}
