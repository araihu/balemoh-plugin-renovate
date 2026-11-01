package requests

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/credential"
)

var errEngine = errors.New("Renovate execution not confirmed")

func engineProfile(r Record, work string, write bool) map[string]any {
	var dryRun any = "full"
	if write {
		dryRun = nil
	}
	allow := map[string]any{"matchPackageNames": []string{"/^" + regexp.QuoteMeta(r.Dependency.PackageName) + "$/"}, "matchFileNames": []string{r.Dependency.PackageFile}, "enabled": true, "allowedVersions": "/^" + regexp.QuoteMeta(*r.Candidate.NewValue) + "$/", "groupName": nil, "groupSlug": nil}
	var identity []string
	if json.Unmarshal([]byte(r.Dependency.DependencyKey), &identity) == nil && len(identity) == 2 && identity[1] != "" {
		allow["matchDepTypes"] = []string{identity[1]}
	}
	return map[string]any{
		"platform": "github", "repositories": []string{r.Repository.Name}, "autodiscover": false, "onboarding": false, "requireConfig": "required",
		"dryRun": dryRun, "persistRepoData": true, "baseDir": work, "cacheDir": filepath.Join(work, "cache"), "reportType": "file", "reportPath": filepath.Join(work, "report.json"), "reportFormatting": false,
		"allowScripts": false, "allowPlugins": false, "allowedCommands": []string{}, "internalHostAccess": "block",
		"force": map[string]any{"automerge": false, "platformAutomerge": false, "dependencyDashboard": false, "dependencyDashboardApproval": false, "prCreation": "immediate", "pruneStaleBranches": false,
			"baseBranchPatterns": []string{r.Repository.BaseBranch}, "includePaths": []string{r.Dependency.PackageFile}, "enabledManagers": []string{r.Dependency.Manager},
			"kubernetes":   map[string]any{"managerFilePatterns": []string{"/^" + regexp.QuoteMeta(r.Dependency.PackageFile) + "$/"}},
			"branchPrefix": "balemoh-renovate/", "branchTopic": r.ID, "additionalBranchPrefix": "", "groupName": nil, "groupSlug": nil, "hashedBranchLength": nil,
			"fetchChangeLogs": "off", "pinDigests": false,
			"packageRules": []any{map[string]any{"matchPackageNames": []string{"*"}, "enabled": false}, allow},
		},
	}
}

func verifyPlan(data []byte, r Record) error {
	var native struct {
		Problems     []json.RawMessage `json:"problems"`
		Repositories map[string]struct {
			Problems []json.RawMessage `json:"problems"`
			Branches []struct {
				Name     string `json:"branchName"`
				Upgrades []struct {
					File    string  `json:"packageFile"`
					Name    string  `json:"depName"`
					Package string  `json:"packageName"`
					Current *string `json:"currentValue"`
					Value   *string `json:"newValue"`
					Digest  *string `json:"newDigest"`
				} `json:"upgrades"`
			} `json:"branches"`
		} `json:"repositories"`
	}
	if json.Unmarshal(data, &native) != nil || len(native.Problems) > 0 || len(native.Repositories) != 1 {
		return errEngine
	}
	repo, ok := native.Repositories[r.Repository.Name]
	if !ok || len(repo.Problems) > 0 || len(repo.Branches) != 1 || repo.Branches[0].Name != r.Branch {
		return errEngine
	}
	upgrades := repo.Branches[0].Upgrades
	if len(upgrades) != max(r.Dependency.SourceOccurrences, 1) {
		return errEngine
	}
	for _, u := range upgrades {
		if u.File != r.Dependency.PackageFile || u.Package != r.Dependency.PackageName || u.Current == nil || u.Value == nil || *u.Current != *r.Dependency.CurrentValue || *u.Value != *r.Candidate.NewValue || u.Digest != nil {
			return errEngine
		}
	}
	return nil
}

type engineFailure struct {
	phase          string
	mayHaveWritten bool
}

func (e *engineFailure) Error() string { return "execution requires review" }

func (c *Controller) runEngine(ctx context.Context, r Record) (result error) {
	phase := "prepare"
	mayHaveWritten := false
	defer func() {
		if result != nil {
			fmt.Fprintf(os.Stderr, "request=%s phase=%s code=execution_failed\n", r.ID, phase)
			result = &engineFailure{phase: phase, mayHaveWritten: mayHaveWritten}
		}
	}()

	if !c.config.WriterEnabled || !Supported(r.Dependency, r.Candidate) {
		return errEngine
	}
	token, e := credential.ReadBearerFile(c.config.GitHubWriteTokenFile)
	if e != nil {
		return errEngine
	}
	if e = os.MkdirAll(c.config.WorkDir, 0700); e != nil {
		return errEngine
	}
	work, e := os.MkdirTemp(c.config.WorkDir, "request-*")
	if e != nil {
		return errEngine
	}
	defer os.RemoveAll(work)
	version := exec.CommandContext(ctx, "/usr/local/renovate/node", "--use-openssl-ca", "/usr/local/renovate/dist/renovate.js", "--version")
	version.Env = c.config.commandEnvironment
	output, e := version.Output()
	if e != nil || strings.TrimSpace(string(output)) != snapshot.RenovateVersion {
		return errEngine
	}
	invoke := func(write bool) error {
		p := engineProfile(r, work, write)
		b, _ := json.Marshal(p)
		path := filepath.Join(work, "config.json")
		if os.WriteFile(path, b, 0600) != nil {
			return errEngine
		}
		command := exec.CommandContext(ctx, "renovate")
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
		command.WaitDelay = 5 * time.Second
		command.Env = append(append([]string{}, c.config.commandEnvironment...), "RENOVATE_CONFIG_FILE="+path, "RENOVATE_TOKEN="+token, "LOG_LEVEL=warn")
		log, e := os.OpenFile(filepath.Join(work, "run.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if e != nil {
			return errEngine
		}
		defer log.Close()
		command.Stdout = log
		command.Stderr = log
		if command.Run() != nil {
			return errEngine
		}
		return nil
	}
	phase = "preflight"
	started := time.Now().UTC()
	if invoke(false) != nil {
		return errEngine
	}
	phase = "provenance"
	base, e := c.github.base(ctx, r.Repository.Name, r.Repository.BaseBranch)
	if e != nil || base != r.Repository.Commit {
		return errEngine
	}
	for _, ref := range []string{"refs/remotes/origin/" + r.Repository.BaseBranch, "refs/heads/" + r.Repository.BaseBranch} {
		command := exec.CommandContext(ctx, "git", "-C", filepath.Join(work, "repos", "github", r.Repository.Name), "rev-parse", ref)
		command.Env = c.config.commandEnvironment
		out, err := command.Output()
		if err != nil || strings.TrimSpace(string(out)) != base {
			return errEngine
		}
	}
	phase = "selection_changed"
	data, e := readBounded(filepath.Join(work, "report.json"), 64<<20)
	if e != nil || verifyPlan(data, r) != nil {
		return errEngine
	}
	// Validate native image identities too. Metadata describes this bounded full dry-run only.
	var id [16]byte
	if _, e = rand.Read(id[:]); e != nil {
		return errEngine
	}
	id[6], id[8] = id[6]&15|64, id[8]&63|128
	now := time.Now().UTC()
	profile, _ := json.Marshal(engineProfile(r, work, false))
	m := snapshot.Metadata{RunID: fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), StartedAt: started, CompletedAt: now, Producer: snapshot.Producer{RenovateVersion: snapshot.RenovateVersion, RenovateImage: snapshot.RenovateImage, AdapterVersion: snapshot.AdapterVersion, PolicyHash: snapshot.ID(string(profile))}, Repository: r.Repository, DryRun: "full", CheckoutCommit: base, BeforeCommit: base, AfterCommit: base}
	s, e := snapshot.Normalize(data, m)
	if e != nil {
		return errEngine
	}
	count := 0
	for _, d := range s.Dependencies {
		for _, u := range d.Candidates {
			if d.ID != r.Dependency.ID || u.ID != r.Candidate.ID {
				return errEngine
			}
			count++
		}
	}
	if count != 1 {
		return errEngine
	}
	exists, err := c.github.branchExists(ctx, r)
	if err != nil || exists {
		return errEngine
	}
	// The writer receives the same scope and fixed tag, with no approval/check bypass.
	phase = "writer"
	mayHaveWritten = true
	if invoke(true) != nil {
		return errEngine
	}
	phase = "post_write"
	base, e = c.github.base(ctx, r.Repository.Name, r.Repository.BaseBranch)
	if e != nil || base != r.Repository.Commit {
		return errEngine
	}
	return nil
}
