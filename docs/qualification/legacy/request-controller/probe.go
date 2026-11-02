// Copy into cmd/controller-proof in an isolated plugin source copy.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/caarlos0/env/v11"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:generate go tool envdoc -output environment.md -format markdown
type probeEnvironment struct {
	// Exact private fixture repository, checked before execution.
	Repository string `env:"GITHUB_REPOSITORY,required,notEmpty"`
	// GitHub checkout commit used as the immutable approved base.
	Commit     string `env:"GITHUB_SHA,required,notEmpty"`
	// Short-lived Actions token scoped only to the private fixture. Never printed.
	Token      string `env:"RENOVATE_TOKEN,required,notEmpty"`
	// Executable search path inherited by the isolated engine.
	Path       string `env:"PATH,required,notEmpty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	environment, parseErr := env.ParseAs[probeEnvironment]()
	if parseErr != nil {
		return fmt.Errorf("invalid isolated proof environment")
	}
	const repo = "guilycst/balemoh-renovate-engine-poc-20261101"
	if environment.Repository != repo {
		return fmt.Errorf("isolated repository required")
	}
	root := "/tmp/controller-proof-state"
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	write := func(name, value string) string {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte(value), 0600); err != nil {
			panic(err)
		}
		return p
	}
	cfg, err := requests.Parse(map[string]string{"UPGRADES_REPOSITORY": repo, "UPGRADES_CONTROLLER_DATA_DIR": root + "/requests", "UPGRADES_CONTROLLER_WORK_DIR": root + "/work", "UPGRADES_CONTROLLER_SNAPSHOT_FILE": root + "/snapshot.json", "UPGRADES_CONTROLLER_READ_BEARER_FILE": write("reader", strings.Repeat("r", 32)), "UPGRADES_CONTROLLER_ACTION_BEARER_FILE": write("action", strings.Repeat("a", 32)), "UPGRADES_CONTROLLER_GITHUB_READ_TOKEN_FILE": write("github-read", environment.Token), "UPGRADES_CONTROLLER_GITHUB_WRITE_TOKEN_FILE": write("github-write", environment.Token), "UPGRADES_CONTROLLER_PR_AUTHOR": "github-actions[bot]", "UPGRADES_CONTROLLER_WRITER_ENABLED": "true", "UPGRADES_CONTROLLER_TLS_CERT": root + "/cert", "UPGRADES_CONTROLLER_TLS_KEY": root + "/key", "PATH": environment.Path, "HOME": root, "TMPDIR": "/tmp"})
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	repository := snapshot.Repository{Platform: "github", Name: repo, BaseBranch: "main", Commit: environment.Commit}
	d := snapshot.Dependency{Manager: "kubernetes", PackageFile: "k8s/controller-selected.yaml", DependencyKey: `["docker.io/library/nginx",""]`, PackageName: "docker.io/library/nginx", CurrentValue: new("1.25.3"), Assessment: "updates_found", SourceOccurrences: 1, Reasons: []string{}}
	d.ID = snapshot.ID(repository.Platform, repo, "main", d.Manager, d.PackageFile, d.DependencyKey, d.PackageName)
	u := snapshot.Candidate{NewValue: new("1.26.3"), UpdateType: "minor", Policy: "unknown", Workflow: "unknown"}
	u.ID = snapshot.ID(d.ID, u.NewValue, u.NewDigest, u.UpdateType)
	d.Candidates = []snapshot.Candidate{u}
	s := snapshot.Snapshot{SchemaVersion: 1, RunID: "00000000-0000-4000-8000-000000000001", StartedAt: now.Add(-time.Minute), CompletedAt: now, Repository: repository, Producer: snapshot.Producer{RenovateVersion: snapshot.RenovateVersion, RenovateImage: snapshot.RenovateImage, AdapterVersion: snapshot.AdapterVersion, PolicyHash: snapshot.ID("controller-proof")}, Coverage: snapshot.Coverage{Scope: "Controlled controller proof", Status: "complete", SourceRecords: 1, Extracted: 1, Assessed: 1, Reasons: []string{}}, Dependencies: []snapshot.Dependency{d}}
	b, _ := json.Marshal(s)
	if _, err = snapshot.Decode(b, now); err != nil {
		return err
	}
	write("snapshot.json", string(b))
	c, err := requests.New(cfg)
	if err != nil {
		return err
	}
	defer c.Close()
	selection := requests.Selection{DependencyID: d.ID, CandidateID: u.ID, SnapshotDigest: requests.Digest(&s)}
	body, _ := json.Marshal(selection)
	submit := func() (requests.Record, error) {
		req := httptest.NewRequest(http.MethodPost, "https://controller/v1/requests", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, req)
		var r requests.Record
		if w.Code != 200 {
			return r, fmt.Errorf("admission status %d", w.Code)
		}
		return r, json.Unmarshal(w.Body.Bytes(), &r)
	}
	first, err := submit()
	if err != nil {
		return err
	}
	repeat, err := submit()
	if err != nil || first.ID != repeat.ID {
		return fmt.Errorf("duplicate admission")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); c.Run(ctx) }()
	defer func() { cancel(); <-done }()
	for ctx.Err() == nil {
		req := httptest.NewRequest("GET", "https://controller/v1/requests", nil)
		req.Header.Set("Authorization", "Bearer "+strings.Repeat("r", 32))
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, req)
		var catalog requests.Catalog
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalog) != nil {
			return fmt.Errorf("catalog unavailable")
		}
		if len(catalog.Requests) != 1 {
			return fmt.Errorf("incorrect durable count")
		}
		r := catalog.Requests[0]
		if r.State == "blocked" || r.State == "needs_attention" {
			return fmt.Errorf("execution %s reason %s", r.State, r.Reason)
		}
		if r.State == "open" && r.PR != nil {
			if r.TrackingError {
				return fmt.Errorf("tracking failed")
			}
			again, e := submit()
			if e != nil || again.ID != r.ID {
				return fmt.Errorf("repeat changed request")
			}
			data, _ := json.MarshalIndent(r, "", "  ")
			if e = os.WriteFile("/out/controller-receipt.json", data, 0644); e != nil {
				return e
			}
			fmt.Printf("PR %s, request %s, checks %s\n", r.PR.URL, r.ID, r.PR.Checks)
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("controller proof timeout")
}
