//go:build recoveryproof

// Opt-in retained-volume qualification; requires explicit operator-selected paths.
package requests

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

var recoveryMode = flag.String("request-recovery-mode", "", "serve or verify; never enables the writer")
var recoveryDirectory = flag.String("request-recovery-directory", "", "mounted request state directory")
var recoveryWork = flag.String("request-recovery-work", "/work", "private temporary fixture directory")

func volumeConfig(t *testing.T) Config {
	t.Helper()
	write := func(name, value string) string {
		p := filepath.Join(*recoveryWork, name)
		if err := os.WriteFile(p, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	c, err := Parse(map[string]string{"UPGRADES_REPOSITORY": "test/repo", "UPGRADES_CONTROLLER_DATA_DIR": *recoveryDirectory, "UPGRADES_CONTROLLER_WORK_DIR": *recoveryWork + "/engine", "UPGRADES_CONTROLLER_READ_BEARER_FILE": write("reader", strings.Repeat("r", 32)), "UPGRADES_CONTROLLER_ACTION_BEARER_FILE": write("action", strings.Repeat("a", 32)), "UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE": write("github-placeholder", strings.Repeat("g", 32)), "UPGRADES_CONTROLLER_PR_AUTHOR": "restore-proof[bot]", "UPGRADES_CONTROLLER_TLS_CERT": *recoveryWork + "/cert", "UPGRADES_CONTROLLER_TLS_KEY": *recoveryWork + "/key", "UPGRADES_CONTROLLER_WRITER_ENABLED": "false"})
	if err != nil || c.WriterEnabled {
		t.Fatal("disabled fixture configuration rejected", err)
	}
	return c
}
func volumeRecord(name, state string) Record {
	repo := snapshot.Repository{Platform: "github", Name: "test/repo", BaseBranch: "main", Commit: strings.Repeat("b", 40)}
	d := snapshot.Dependency{Manager: "kubernetes", PackageFile: "k8s/" + name + ".yaml", DependencyKey: `["nginx","container"]`, PackageName: "nginx", CurrentValue: new("1.25.3"), Assessment: "updates_found", SourceOccurrences: 1, Reasons: []string{}}
	d.ID = snapshot.ID(repo.Platform, repo.Name, repo.BaseBranch, d.Manager, d.PackageFile, d.DependencyKey, d.PackageName)
	u := snapshot.Candidate{NewValue: new("1.26.3"), UpdateType: "minor", Policy: "unknown", Workflow: "unknown"}
	u.ID = snapshot.ID(d.ID, u.NewValue, u.NewDigest, u.UpdateType)
	d.Candidates = []snapshot.Candidate{u}
	now := time.Now().UTC()
	id := snapshot.ID(repo, d.ID, u.ID)
	r := Record{SnapshotDigest: snapshot.ID("volume-proof", name), ID: id, Repository: repo, Dependency: d, Candidate: u, Branch: "balemoh-renovate/" + id, State: state, CreatedAt: now, UpdatedAt: now}
	if name == "known" {
		r.PR = &PullRequest{Number: 42, URL: "https://github.com/test/repo/pull/42", HeadSHA: strings.Repeat("c", 40), State: "open", Checks: "unknown"}
	}
	return r
}
func TestRequestVolumeRecovery(t *testing.T) {
	if *recoveryMode == "" {
		t.Skip("explicit homelab volume proof only")
	}
	if !filepath.IsAbs(*recoveryDirectory) || !filepath.IsAbs(*recoveryWork) {
		t.Fatal("absolute mounted paths required")
	}
	if err := os.MkdirAll(*recoveryWork, 0700); err != nil {
		t.Fatal(err)
	}
	config := volumeConfig(t)
	if *recoveryMode == "serve" {
		c, err := New(config)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		if len(c.store.list()) != 0 {
			t.Fatal("refusing to replace existing source records")
		}
		for _, r := range []Record{volumeRecord("running", "running"), volumeRecord("known", "open"), volumeRecord("queued", "queued")} {
			c.store.mu.Lock()
			err = c.store.save(r)
			c.store.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
		}
		if second, err := New(config); err == nil {
			second.Close()
			t.Fatal("second owner acquired the same volume")
		}
		if err := os.WriteFile(filepath.Join(config.DataDir, ".request-uncommitted"), []byte("incomplete unpublished record"), 0600); err != nil {
			t.Fatal(err)
		}
		server := httptest.NewTLSServer(c.Handler())
		defer server.Close()
		ca := filepath.Join(*recoveryWork, "source-ca")
		if err := os.WriteFile(ca, []byte(base64.StdEncoding.EncodeToString(server.TLS.Certificates[0].Certificate[0])), 0600); err != nil {
			t.Fatal(err)
		}
		info := map[string]string{"url": server.URL + "/v1/requests", "tokenFile": config.ReadBearerFile, "caFile": ca, "repository": "test/repo"}
		b, _ := json.Marshal(info)
		if err := os.WriteFile(filepath.Join(*recoveryWork, "source.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
		fmt.Println("source_records=3 exclusive_lock=verified atomic_fsync_save=verified writer_enabled=false")
		ctx, cancel := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		<-ctx.Done()
		return
	}
	if *recoveryMode != "verify" {
		t.Fatal("unknown recovery mode")
	}
	manifestBytes, err := os.ReadFile(filepath.Join(config.DataDir, ".backup-manifest"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files map[string]string `json:"files"`
	}
	if json.Unmarshal(manifestBytes, &manifest) != nil || len(manifest.Files) != 3 {
		t.Fatal("three backed-up records required")
	}
	for file, want := range manifest.Files {
		if !identifier.MatchString(strings.TrimSuffix(file, ".json")) || filepath.Base(file) != file {
			t.Fatal("unsafe backup path")
		}
		p := filepath.Join(config.DataDir, file)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != want {
			t.Fatal("restored checksum differs")
		}
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("restored record mode differs")
		}
	}
	c, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if len(c.store.list()) != 3 {
		t.Fatal("restored record count differs")
	}
	if second, err := New(config); err == nil {
		second.Close()
		t.Fatal("restored volume admits a second owner")
	}
	var executions, nonGET atomic.Int32
	known := volumeRecord("known", "open")
	p := nativePR{Number: 42, URL: known.PR.URL, State: "open"}
	p.User.Login = config.PRAuthor
	p.Head.Ref = known.Branch
	p.Head.SHA = known.PR.HeadSHA
	p.Head.Repo.FullName = config.Repository
	p.Base.Ref = config.BaseBranch
	p.Base.Repo.FullName = config.Repository
	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			nonGET.Add(1)
			w.WriteHeader(405)
			return
		}
		switch {
		case r.URL.Path == "/repos/test/repo/pulls":
			if strings.HasSuffix(r.URL.Query().Get("head"), known.Branch) {
				_ = json.NewEncoder(w).Encode([]nativePR{p})
			} else {
				_, _ = w.Write([]byte(`[]`))
			}
		case strings.HasSuffix(r.URL.Path, "/pulls/42/files"):
			_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": known.Dependency.PackageFile}})
		case strings.HasSuffix(r.URL.Path, "/pulls/42"):
			_ = json.NewEncoder(w).Encode(p)
		case strings.Contains(r.URL.Path, "/contents/"):
			body := "image: nginx:1.25.3\n"
			if r.URL.Query().Get("ref") != known.Repository.Commit {
				body = "image: nginx:1.26.3\n"
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(body))})
		case strings.HasSuffix(r.URL.Path, "/check-runs"):
			_, _ = w.Write([]byte(`{"total_count":0,"check_runs":[]}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"total_count":0,"statuses":[]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer githubServer.Close()
	c.github.endpoint = githubServer.URL
	c.github.client = githubServer.Client()
	c.execute = func(context.Context, Record) error { executions.Add(1); return fmt.Errorf("writer must never execute") }
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); c.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(15 * time.Second)
	verified := false
	for time.Now().Before(deadline) {
		running, tracked, queued := false, false, false
		for _, r := range c.store.list() {
			switch r.Dependency.PackageFile {
			case "k8s/running.yaml":
				running = r.State == "needs_attention" && r.Reason == "execution_interrupted"
			case "k8s/known.yaml":
				tracked = r.State == "open" && !r.TrackingError && r.TrackedAt != nil && r.PR != nil && r.PR.URL == known.PR.URL
			case "k8s/queued.yaml":
				queued = r.State == "queued"
			}
		}
		if running && tracked && queued {
			verified = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !verified || executions.Load() != 0 || nonGET.Load() != 0 {
		t.Fatal("restore transition, reconciliation or no-write invariant failed")
	}
	request := httptest.NewRequest("POST", "https://controller/v1/requests", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+c.action)
	response := httptest.NewRecorder()
	c.Handler().ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal("disabled writer accepted a request")
	}
	receipt := map[string]any{"restoredRecords": 3, "checksumsVerified": true, "modesVerified": true, "exclusiveLockVerified": true, "interruptedState": "needs_attention", "existingPRReconciled": true, "executorCalls": executions.Load(), "nonGETGitHubCalls": nonGET.Load(), "writerEnabled": false, "admissionStatus": response.Code, "githubEvidence": "local controlled fixture; no production PR interaction"}
	data, _ := json.MarshalIndent(receipt, "", "  ")
	if err := os.WriteFile(filepath.Join(*recoveryWork, "recovery-receipt.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(data))
}
