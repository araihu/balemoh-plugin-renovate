package requests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func setup(t *testing.T) (*Controller, Selection) {
	t.Helper()
	dir := t.TempDir()
	write := func(name, value string) string {
		p := filepath.Join(dir, name)
		if e := os.WriteFile(p, []byte(value), 0600); e != nil {
			t.Fatal(e)
		}
		return p
	}
	values := map[string]string{"UPGRADES_REPOSITORY": "test/repo", "UPGRADES_CONTROLLER_DATA_DIR": filepath.Join(dir, "state"), "UPGRADES_CONTROLLER_WORK_DIR": filepath.Join(dir, "work"), "UPGRADES_CONTROLLER_SNAPSHOT_FILE": filepath.Join(dir, "snapshot.json"), "UPGRADES_CONTROLLER_READ_BEARER_FILE": write("reader", strings.Repeat("r", 32)), "UPGRADES_CONTROLLER_ACTION_BEARER_FILE": write("action", strings.Repeat("a", 32)), "UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE": write("github-read", strings.Repeat("g", 32)), "UPGRADES_CONTROLLER_GITHUB_WRITE_TOKEN_FILE": write("github-write", strings.Repeat("w", 32)), "UPGRADES_CONTROLLER_TLS_CERT": filepath.Join(dir, "tls.crt"), "UPGRADES_CONTROLLER_TLS_KEY": filepath.Join(dir, "tls.key"), "UPGRADES_CONTROLLER_PR_AUTHOR": "renovate-test[bot]", "UPGRADES_CONTROLLER_WRITER_ENABLED": "true"}
	cfg, e := Parse(values)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Truncate(time.Second)
	repo := snapshot.Repository{Platform: "github", Name: "test/repo", BaseBranch: "main", Commit: strings.Repeat("b", 40)}
	d := snapshot.Dependency{Manager: "kubernetes", PackageFile: "k8s/test.yaml", DependencyKey: `["nginx","container"]`, PackageName: "nginx", CurrentValue: new("1.25.3"), Assessment: "updates_found", SourceOccurrences: 1, Reasons: []string{}}
	d.ID = snapshot.ID(repo.Platform, repo.Name, repo.BaseBranch, d.Manager, d.PackageFile, d.DependencyKey, d.PackageName)
	u := snapshot.Candidate{NewValue: new("1.26.3"), UpdateType: "minor", Policy: "unknown", Workflow: "unknown"}
	u.ID = snapshot.ID(d.ID, u.NewValue, u.NewDigest, u.UpdateType)
	d.Candidates = []snapshot.Candidate{u}
	s := snapshot.Snapshot{SchemaVersion: 1, RunID: "00000000-0000-4000-8000-000000000001", StartedAt: now.Add(-time.Minute), CompletedAt: now, Repository: repo, Producer: snapshot.Producer{RenovateVersion: snapshot.RenovateVersion, RenovateImage: snapshot.RenovateImage, AdapterVersion: snapshot.AdapterVersion, PolicyHash: snapshot.ID("test")}, Coverage: snapshot.Coverage{Scope: "test", Status: "complete", SourceRecords: 1, Extracted: 1, Assessed: 1, Reasons: []string{}}, Dependencies: []snapshot.Dependency{d}}
	b, _ := json.Marshal(s)
	if _, e := snapshot.Decode(b, time.Now()); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(cfg.SnapshotFile, b, 0600); e != nil {
		t.Fatal(e)
	}
	c, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(c.Close)
	return c, Selection{DependencyID: d.ID, CandidateID: u.ID, SnapshotDigest: Digest(&s)}
}

func TestDurableConcurrentAdmissionAndRoleSeparation(t *testing.T) {
	c, selection := setup(t)
	var wg sync.WaitGroup
	ids := make(chan string, 20)
	for range 20 {
		wg.Go(func() {
			r, e := c.Submit(t.Context(), selection)
			if e != nil {
				t.Error(e)
				return
			}
			ids <- r.ID
		})
	}
	wg.Wait()
	close(ids)
	var id string
	for got := range ids {
		if id != "" && got != id {
			t.Fatal("duplicate intent identity")
		}
		id = got
	}
	if len(c.store.list()) != 1 {
		t.Fatal("duplicate durable requests")
	}
	if _, e := openStore(c.config.DataDir); e == nil {
		t.Fatal("second controller acquired storage")
	}
	encoded, _ := json.Marshal(selection)
	for _, test := range []struct {
		method, token string
		status        int
	}{{"POST", c.reader, 401}, {"GET", c.action, 401}, {"POST", c.action, 200}, {"GET", c.reader, 200}} {
		r := httptest.NewRequest(test.method, "/v1/requests", strings.NewReader(string(encoded)))
		r.Header.Set("Authorization", "Bearer "+test.token)
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%s got=%d want=%d", test.method, w.Code, test.status)
		}
	}
	c.Close()
	reopened, e := New(c.config)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if len(reopened.store.list()) != 1 || reopened.store.list()[0].ID != id {
		t.Fatal("durable receipt lost")
	}
}

func TestAdmissionRejectsDisabledStaleForeignAndUnsupportedSelections(t *testing.T) {
	for _, kind := range []string{"disabled", "stale", "foreign", "digest", "extra"} {
		t.Run(kind, func(t *testing.T) {
			c, s := setup(t)
			switch kind {
			case "disabled":
				c.config.WriterEnabled = false
			case "stale":
				s.SnapshotDigest = strings.Repeat("0", 64)
			case "foreign":
				s.DependencyID = strings.Repeat("0", 64)
			case "digest":
				data, _ := os.ReadFile(c.config.SnapshotFile)
				var snap snapshot.Snapshot
				_ = json.Unmarshal(data, &snap)
				d := &snap.Dependencies[0]
				u := &d.Candidates[0]
				u.NewDigest = new("sha256:" + strings.Repeat("a", 64))
				u.ID = snapshot.ID(d.ID, u.NewValue, u.NewDigest, u.UpdateType)
				data, _ = json.Marshal(snap)
				_ = os.WriteFile(c.config.SnapshotFile, data, 0600)
				s.CandidateID = u.ID
				s.SnapshotDigest = Digest(&snap)
			case "extra":
				r := httptest.NewRequest("POST", "/v1/requests", strings.NewReader(`{"dependencyId":"x","command":"merge"}`))
				r.Header.Set("Authorization", "Bearer "+c.action)
				w := httptest.NewRecorder()
				c.Handler().ServeHTTP(w, r)
				if w.Code != 400 {
					t.Fatal(w.Code)
				}
				return
			}
			if _, e := c.Submit(t.Context(), s); e == nil {
				t.Fatal("invalid selection admitted")
			}
			if len(c.store.list()) != 0 {
				t.Fatal("rejected action changed store")
			}
		})
	}
}

func githubFixture(t *testing.T, c *Controller, r Record) (*nativePR, *bool) {
	t.Helper()
	p := &nativePR{Number: 42, URL: "https://github.com/test/repo/pull/42", State: "open"}
	p.User.Login = c.config.PRAuthor
	p.Head.Ref = r.Branch
	p.Head.SHA = strings.Repeat("c", 40)
	p.Head.Repo.FullName = r.Repository.Name
	p.Base.Ref = "main"
	p.Base.Repo.FullName = r.Repository.Name
	present := new(bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" {
			t.Error("tracker mutated GitHub")
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(req.URL.Path, "/pulls"):
			if req.URL.Query().Get("head") != "test:"+r.Branch || req.URL.Query().Get("base") != "main" || req.URL.Query().Get("state") != "all" {
				t.Error("unscoped PR lookup")
			}
			if *present {
				_ = json.NewEncoder(w).Encode([]nativePR{*p})
			} else {
				_, _ = w.Write([]byte(`[]`))
			}
		case strings.HasSuffix(req.URL.Path, "/pulls/42"):
			_ = json.NewEncoder(w).Encode(p)
		case strings.HasSuffix(req.URL.Path, "/pulls/42/files"):
			_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": r.Dependency.PackageFile}})
		case strings.Contains(req.URL.Path, "/contents/"):
			value := "image: nginx:1.25.3\n"
			if req.URL.Query().Get("ref") != r.Repository.Commit {
				value = "image: nginx:1.26.3\n"
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(value))})
		case strings.HasSuffix(req.URL.Path, "/git/ref/heads/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{"object": map[string]string{"sha": r.Repository.Commit}})
		case strings.Contains(req.URL.Path, "/git/matching-refs/"):
			_, _ = w.Write([]byte(`[]`))
		case strings.HasSuffix(req.URL.Path, "/check-runs"):
			_, _ = w.Write([]byte(`{"total_count":1,"check_runs":[{"status":"completed","conclusion":"success"}]}`))
		case strings.HasSuffix(req.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"total_count":0,"statuses":[]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	c.github.endpoint = server.URL
	c.github.client = server.Client()
	return p, present
}

func TestWorkerTracksManualPRLifecycleWithoutDuplicateExecution(t *testing.T) {
	c, s := setup(t)
	r, e := c.Submit(t.Context(), s)
	if e != nil {
		t.Fatal(e)
	}
	p, present := githubFixture(t, c, r)
	calls := 0
	c.execute = func(context.Context, Record) error { calls++; *present = true; return nil }
	c.cycle(t.Context())
	if calls != 1 || c.store.list()[0].State != "awaiting_pr" {
		t.Fatal("request did not execute once")
	}
	c.cycle(t.Context())
	got := c.store.list()[0]
	if calls != 1 || got.State != "open" || got.PR.Checks != "passing" {
		t.Fatalf("incorrect PR reconciliation: %+v", got)
	}
	if _, e = c.Submit(t.Context(), s); e != nil {
		t.Fatal(e)
	}
	c.cycle(t.Context())
	if calls != 1 {
		t.Fatal("repeated action executed engine again")
	}
	p.Draft = true
	c.cycle(t.Context())
	if c.store.list()[0].State != "draft" {
		t.Fatal("draft state lost")
	}
	p.Draft = false
	p.State = "closed"
	c.cycle(t.Context())
	if c.store.list()[0].State != "closed" {
		t.Fatal("manual close state lost")
	}
	p.State = "open"
	c.cycle(t.Context())
	if c.store.list()[0].State != "open" {
		t.Fatal("reopened state lost")
	}
	now := time.Now()
	p.MergedAt = &now
	p.State = "closed"
	c.cycle(t.Context())
	if c.store.list()[0].State != "merged" || calls != 1 {
		t.Fatal("manual merge reconciliation failed")
	}
}

func TestRestartDoesNotReplayInterruptedExecution(t *testing.T) {
	c, s := setup(t)
	r, e := c.Submit(t.Context(), s)
	if e != nil {
		t.Fatal(e)
	}
	r.State = "running"
	c.store.mu.Lock()
	e = c.store.save(r)
	c.store.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
	next, e := New(c.config)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	next.execute = func(context.Context, Record) error { t.Fatal("interrupted write replayed"); return nil }
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	next.Run(ctx)
	if next.store.list()[0].State != "needs_attention" {
		t.Fatal("ambiguous execution was not retained")
	}
}

func TestGitHubWrongAuthorAndAutomergeFailClosed(t *testing.T) {
	for _, kind := range []string{"author", "automerge"} {
		t.Run(kind, func(t *testing.T) {
			c, s := setup(t)
			r, e := c.Submit(t.Context(), s)
			if e != nil {
				t.Fatal(e)
			}
			p, present := githubFixture(t, c, r)
			*present = true
			switch kind {
			case "author":
				p.User.Login = "someone"
			case "automerge":
				p.AutoMerge = json.RawMessage(`{"merge_method":"squash"}`)
			}
			_, err := c.github.pull(t.Context(), r)
			if err == nil {
				t.Fatal("untrusted PR association")
			}
		})
	}
}

type responseTransport func(*http.Request) (*http.Response, error)

func (f responseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCheckEvidenceAndHeadRace(t *testing.T) {
	for _, kind := range []string{"empty", "neutral", "skipped", "truncated", "pending", "failure", "read_error", "head_race"} {
		t.Run(kind, func(t *testing.T) {
			c, s := setup(t)
			r, e := c.Submit(t.Context(), s)
			if e != nil {
				t.Fatal(e)
			}
			_, present := githubFixture(t, c, r)
			*present = true
			original := c.github.client.Transport
			c.github.client.Transport = responseTransport(func(req *http.Request) (*http.Response, error) {
				response, err := original.RoundTrip(req)
				if err != nil {
					return response, err
				}
				body := ""
				if strings.HasSuffix(req.URL.Path, "/check-runs") {
					switch kind {
					case "empty":
						body = `{"total_count":0,"check_runs":[]}`
					case "neutral", "skipped":
						body = `{"total_count":1,"check_runs":[{"status":"completed","conclusion":"` + kind + `"}]}`
					case "truncated":
						body = `{"total_count":2,"check_runs":[{"status":"completed","conclusion":"success"}]}`
					case "pending":
						body = `{"total_count":1,"check_runs":[{"status":"in_progress"}]}`
					case "failure":
						body = `{"total_count":1,"check_runs":[{"status":"completed","conclusion":"failure"}]}`
					case "read_error":
						response.StatusCode = 403
					}
				}
				if kind == "head_race" && strings.HasSuffix(req.URL.Path, "/pulls/42") {
					b, _ := io.ReadAll(response.Body)
					body = strings.ReplaceAll(string(b), strings.Repeat("c", 40), strings.Repeat("d", 40))
				}
				if body != "" {
					response.Body.Close()
					response.Body = io.NopCloser(strings.NewReader(body))
				}
				return response, nil
			})
			pr, err := c.github.pull(t.Context(), r)
			if pr == nil {
				t.Fatal("basic PR state lost", err)
			}
			want := "unknown"
			if kind == "pending" {
				want = "pending"
			}
			if kind == "failure" {
				want = "failing"
			}
			if pr.Checks != want {
				t.Fatalf("%s checks = %s, want %s", kind, pr.Checks, want)
			}
			if (kind == "read_error") != (err != nil) {
				t.Fatal("incorrect tracking error", err)
			}
		})
	}
}

func TestTerminalReceiptsAreStartingStatesWithoutExecution(t *testing.T) {
	for _, state := range []string{"closed", "merged"} {
		t.Run(state, func(t *testing.T) {
			c, s := setup(t)
			r, e := c.Submit(t.Context(), s)
			if e != nil {
				t.Fatal(e)
			}
			p, present := githubFixture(t, c, r)
			*present = true
			p.State = "closed"
			if state == "merged" {
				now := time.Now()
				p.MergedAt = &now
			}
			r.State = state
			r.PR = &PullRequest{Number: 42, URL: p.URL, HeadSHA: p.Head.SHA, State: state, Checks: "unknown"}
			c.store.mu.Lock()
			e = c.store.save(r)
			c.store.mu.Unlock()
			if e != nil {
				t.Fatal(e)
			}
			c.execute = func(context.Context, Record) error { t.Fatal("terminal receipt executed"); return nil }
			got, e := c.Submit(t.Context(), s)
			if e != nil || got.ID != r.ID || got.State != state {
				t.Fatal("terminal receipt changed", e)
			}
			c.cycle(t.Context())
			if c.store.list()[0].State != state {
				t.Fatal("terminal state lost")
			}
		})
	}
}

func TestAdmissionReservesBoundedCatalogCapacity(t *testing.T) {
	c, selection := setup(t)
	r, e := c.Submit(t.Context(), selection)
	if e != nil {
		t.Fatal(e)
	}
	r.Repository.Commit = strings.Repeat("e", 40)
	r.ID = snapshot.ID(r.Repository, r.Dependency.ID, r.Candidate.ID)
	r.Branch = "balemoh-renovate/" + r.ID
	r.Dependency.Reasons = []string{strings.Repeat("x", 512<<10)}
	c.store.mu.Lock()
	e = c.store.save(r)
	c.store.mu.Unlock()
	if e == nil || len(c.store.list()) != 1 {
		t.Fatal("oversized catalog admitted")
	}
}
