package runner

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/artifact"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

var ErrRun = errors.New("collector failed; last snapshot retained")

func Run(ctx context.Context, c Config) (result error) {
	phase := "prepare"
	defer func() {
		if result != nil {
			fmt.Fprintf(os.Stderr, "phase=%s code=collection_failed\n", phase)
		}
	}()
	if err := os.MkdirAll(c.PublishDir, 0750); err != nil {
		return ErrRun
	}
	lock, err := os.OpenFile(filepath.Join(c.PublishDir, "collector.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return ErrRun
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return ErrRun
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	tokenBytes, err := os.ReadFile(c.TokenFile)
	if err != nil {
		return ErrRun
	}
	token := strings.TrimSpace(string(tokenBytes))
	if len(token) < 16 || strings.ContainsAny(token, "\r\n\t ") {
		return ErrRun
	}
	// Per-run empty storage prevents stale reports and cached checkout provenance.
	work, err := os.MkdirTemp(c.WorkDir, "scan-*")
	if err != nil {
		return ErrRun
	}
	defer os.RemoveAll(work)
	if runtimeVersion(ctx, c.commandEnvironment) != "44.197.0" {
		return ErrRun
	}
	phase = "provenance"
	before, err := remoteCommit(ctx, c, token)
	if err != nil {
		return ErrRun
	}
	config, _ := profile(c, work)
	configPath := filepath.Join(work, "config.json")
	encoded, _ := json.Marshal(config)
	if os.WriteFile(configPath, encoded, 0600) != nil {
		return ErrRun
	}
	// Configuration from the invoking environment cannot enable writer mode.
	environment := append([]string{}, c.commandEnvironment...)
	environment = append(environment, "RENOVATE_CONFIG_FILE="+configPath, "RENOVATE_TOKEN="+token, "LOG_LEVEL=warn")
	phase = "scan"
	started := time.Now().UTC()
	command := exec.CommandContext(ctx, "renovate")
	command.Env = environment
	// Raw output may contain registry addresses. Keep it in disposable private storage.
	log, err := os.OpenFile(filepath.Join(work, "run.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return ErrRun
	}
	command.Stdout = log
	command.Stderr = log
	runErr := command.Run()
	_ = log.Close()
	if runErr != nil {
		return ErrRun
	}
	phase = "provenance"
	completed := time.Now().UTC()
	after, err := remoteCommit(ctx, c, token)
	if err != nil || before != after {
		return ErrRun
	}
	checkout := filepath.Join(work, "repos", "github", c.Repository)
	commit, err := git(ctx, checkout, c.commandEnvironment, "rev-parse", "refs/remotes/origin/"+c.BaseBranch)
	if err != nil || strings.TrimSpace(string(commit)) != before {
		return ErrRun
	}
	head, err := git(ctx, checkout, c.commandEnvironment, "rev-parse", "refs/heads/"+c.BaseBranch)
	if err != nil || strings.TrimSpace(string(head)) != before {
		return ErrRun
	}
	phase = "policy"
	policyBytes, err := git(ctx, checkout, c.commandEnvironment, "show", before+":renovate.json")
	if err != nil {
		return ErrRun
	}
	var policy any
	if json.Unmarshal(policyBytes, &policy) != nil || !safePolicy(policy) {
		return ErrRun
	}
	policyProfile := maps.Clone(config)
	for _, key := range []string{"baseDir", "cacheDir", "reportPath"} {
		delete(policyProfile, key)
	}
	policyHash := snapshot.ID(policy, policyProfile, snapshot.RenovateImage)
	report, err := os.ReadFile(filepath.Join(work, "report.json"))
	if err != nil {
		return ErrRun
	}
	var uuid [16]byte
	if _, err = rand.Read(uuid[:]); err != nil {
		return ErrRun
	}
	uuid[6] = uuid[6]&0x0f | 0x40
	uuid[8] = uuid[8]&0x3f | 0x80
	runID := fmt.Sprintf("%x-%x-%x-%x-%x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
	m := snapshot.Metadata{RunID: runID, StartedAt: started, CompletedAt: completed, Producer: snapshot.Producer{RenovateVersion: snapshot.RenovateVersion, RenovateImage: snapshot.RenovateImage, AdapterVersion: snapshot.AdapterVersion, PolicyHash: policyHash}, Repository: snapshot.Repository{Platform: "github", Name: c.Repository, BaseBranch: c.BaseBranch, Commit: before}, ExitCode: 0, DryRun: "full", CheckoutCommit: before, BeforeCommit: before, AfterCommit: after}
	phase = "normalize"
	s, err := snapshot.Normalize(report, m)
	if err != nil {
		return ErrRun
	}
	phase = "publish"
	data, err := json.Marshal(s)
	if err != nil || artifact.Publish(c.PublishDir, data) != nil {
		return ErrRun
	}
	fmt.Printf("run=%s repository=%s extracted=%d unknown=%d coverage=%s\n", s.RunID, c.Repository, s.Coverage.Extracted, s.Coverage.Unknown, s.Coverage.Status)
	return nil
}
func runtimeVersion(ctx context.Context, environment []string) string {
	// The containerbase wrapper writes tool initialization logs on a cold /tmp.
	command := exec.CommandContext(ctx, "/usr/local/renovate/node", "--use-openssl-ca", "/usr/local/renovate/dist/renovate.js", "--version")
	command.Env = environment
	out, e := command.Output()
	if e != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
func git(ctx context.Context, path string, environment []string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...)
	command.Env = environment
	return command.Output()
}
func remoteCommit(ctx context.Context, c Config, token string) (string, error) {
	u := "https://api.github.com/repos/" + c.Repository + "/git/ref/heads/" + url.PathEscape(c.BaseBranch)
	r, e := http.NewRequestWithContext(ctx, "GET", u, nil)
	if e != nil {
		return "", ErrRun
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Accept", "application/vnd.github+json")
	r.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(r)
	if e != nil {
		return "", ErrRun
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return "", ErrRun
	}
	var record struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&record) != nil || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(record.Object.SHA) {
		return "", ErrRun
	}
	return record.Object.SHA, nil
}

// Only bundled presets are reproducible from the pinned Renovate image.
// Credential-bearing configuration is rejected before hashing or publication.
func safePolicy(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			lower := strings.ToLower(k)
			for _, bad := range []string{"token", "password", "secret", "credential", "privatekey", "hostrules", "username", "endpoint", "env"} {
				if strings.Contains(lower, bad) {
					return false
				}
			}
			if k == "extends" {
				items, ok := v.([]any)
				if !ok {
					return false
				}
				for _, item := range items {
					s, ok := item.(string)
					if !ok || !strings.HasPrefix(s, "config:") {
						return false
					}
				}
			}
			if !safePolicy(v) {
				return false
			}
		}
	case []any:
		for _, item := range x {
			if !safePolicy(item) {
				return false
			}
		}
	case string:
		if strings.Contains(x, "://") {
			u, e := url.Parse(x)
			if e == nil && (u.User != nil || u.RawQuery != "") {
				return false
			}
		}
	}
	return true
}

func profile(c Config, work string) (map[string]any, map[string]any) {
	force := map[string]any{"dryRun": "full", "automerge": false, "enabledManagers": []string{"kubernetes", "helm-values", "kustomize", "dockerfile", "docker-compose", "custom.regex"}, "fetchChangeLogs": "off", "baseBranchPatterns": []string{c.BaseBranch}}
	config := map[string]any{"platform": "github", "repositories": []string{c.Repository}, "autodiscover": false, "onboarding": false, "requireConfig": "required", "dryRun": "full", "baseDir": work, "cacheDir": filepath.Join(work, "cache"), "persistRepoData": true, "reportType": "file", "reportPath": filepath.Join(work, "report.json"), "reportFormatting": false, "internalHostAccess": "block", "allowScripts": false, "allowPlugins": false, "allowedCommands": []string{}, "force": force}

	return config, force
}
