package advisory

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testSnapshot(now time.Time) *snapshot.Snapshot {
	r := snapshot.Repository{Platform: "github", Name: "example/read-only", BaseBranch: "main", Commit: strings.Repeat("a", 40)}
	value := "1"
	newValue := "2"
	d := snapshot.Dependency{Manager: "kubernetes", PackageFile: "a.yaml", DependencyKey: `["nginx",""]`, PackageName: "nginx", CurrentValue: &value, Assessment: "updates_found", SourceOccurrences: 1, Reasons: []string{}, Candidates: []snapshot.Candidate{}}
	d.ID = snapshot.ID(r.Platform, r.Name, r.BaseBranch, d.Manager, d.PackageFile, d.DependencyKey, d.PackageName)
	u := snapshot.Candidate{NewValue: &newValue, UpdateType: "major", Policy: "unknown", Workflow: "unknown"}
	u.ID = snapshot.ID(d.ID, u.NewValue, u.NewDigest, u.UpdateType)
	d.Candidates = append(d.Candidates, u)
	return &snapshot.Snapshot{SchemaVersion: 1, RunID: "00000000-0000-4000-8000-000000000001", StartedAt: now.Add(-time.Minute), CompletedAt: now, Producer: snapshot.Producer{RenovateVersion: snapshot.RenovateVersion, RenovateImage: snapshot.RenovateImage, AdapterVersion: snapshot.AdapterVersion, PolicyHash: snapshot.ID("test")}, Repository: r, Coverage: snapshot.Coverage{Scope: "test fixture", Status: "complete", SourceRecords: 1, Extracted: 1, Assessed: 1, Reasons: []string{}}, Dependencies: []snapshot.Dependency{d}}
}
func testConnection(source *httptest.Server) *connection {
	return &connection{id: "00000000-0000-4000-8000-000000000002", revision: "1", config: Config{SnapshotURL: source.URL, BrowserRepositoryURL: "https://github.com/example/read-only", IntervalSeconds: 30, MaxSnapshotAgeSeconds: 3600}, client: source.Client(), observation: 1, state: "unavailable", seen: map[string]bool{}}
}
func TestRetainedQueries304AndFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	s := testSnapshot(now)
	var calls atomic.Int32
	var state atomic.Int32
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch state.Load() {
		case 1:
			w.WriteHeader(304)
		case 2:
			w.WriteHeader(503)
			_, _ = w.Write([]byte("secret-upstream-error"))
		case 3:
			_, _ = w.Write([]byte(`{"invalid":true}`))
		default:
			w.Header().Set("ETag", `"test"`)
			_ = json.NewEncoder(w).Encode(s)
		}
	}))
	defer source.Close()
	c := testConnection(source)
	b := New(ctx)
	b.connections[c.id] = c
	if !c.observe(ctx) {
		t.Fatal("initial fetch failed")
	}
	revision := c.observation
	query := func() backend.Result {
		r, e := b.Query(ctx, backend.Request{ConnectionID: c.id, OperationID: "upgrades.list", Input: json.RawMessage(`{}`)})
		if e != nil {
			t.Fatal(e)
		}
		if r.Validate() != nil {
			t.Fatal("invalid result")
		}
		return r
	}
	for range 5 {
		query()
	}
	if calls.Load() != 1 {
		t.Fatal("query performed source I/O")
	}
	state.Store(1)
	if !c.observe(ctx) || c.observation != revision || !query().ObservedAt.Equal(now) {
		t.Fatal("304 altered producer observation")
	}
	for _, failure := range []int32{2, 3} {
		state.Store(failure)
		if c.observe(ctx) {
			t.Fatal("accepted failure")
		}
		result := query()
		if result.State != "stale" || !strings.Contains(string(result.Data), "nginx") || strings.Contains(result.SourceError.Message, "secret") {
			t.Fatal("failure cleared or leaked data")
		}
	}
	state.Store(0)
	if !c.observe(ctx) || query().State != "fresh" {
		t.Fatal("recovery failed")
	}
	c.snapshot.CompletedAt = now.Add(-2 * time.Hour)
	old := c.observation
	if query().State != "stale" || c.observation != old+1 {
		t.Fatal("age did not transition")
	}
	query()
	if c.observation != old+1 {
		t.Fatal("unchanged reads increment revision")
	}
}
func TestRejectOlderAndConflictingRuns(t *testing.T) {
	ctx := context.Background()
	s := testSnapshot(time.Now().UTC())
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(s) }))
	defer source.Close()
	c := testConnection(source)
	if !c.observe(ctx) {
		t.Fatal("initial")
	}
	s.Dependencies[0].Reasons = []string{"changed"}
	if c.observe(ctx) {
		t.Fatal("conflicting reused run admitted")
	}
	s.RunID = "00000000-0000-4000-8000-000000000003"
	s.CompletedAt = s.CompletedAt.Add(-time.Hour)
	s.StartedAt = s.CompletedAt.Add(-time.Minute)
	if c.observe(ctx) {
		t.Fatal("older run admitted")
	}
}
func TestInvalidConfigurePreservesCollector(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer source.Close()
	b := New(ctx)
	cfg := Config{SnapshotURL: source.URL, BrowserRepositoryURL: "https://github.com/example/read-only"}
	encoded, _ := json.Marshal(cfg)
	ca := base64.StdEncoding.EncodeToString(source.Certificate().Raw)
	request := backend.Config{ConnectionID: "c", Revision: "1", Config: encoded, Secrets: map[string]string{"snapshot-ca": ca}}
	if _, e := b.Configure(ctx, request); e != nil {
		t.Fatal(e)
	}
	old := b.connections["c"]
	request.Revision = "2"
	request.Config = json.RawMessage(`{"snapshotURL":"http://insecure.test"}`)
	if _, e := b.Configure(ctx, request); e == nil || b.connections["c"] != old {
		t.Fatal("invalid config replaced collector")
	}
	if e := b.CloseConnection(ctx, "c"); e != nil {
		t.Fatal(e)
	}
}
func TestPaginationAndMissingDetail(t *testing.T) {
	c := &connection{id: "c", revision: "1", observation: 12, config: Config{MaxSnapshotAgeSeconds: 43200, IntervalSeconds: 60}, state: "fresh", snapshot: testSnapshot(time.Now())}
	now := time.Now()
	c.success = &now
	d := c.snapshot.Dependencies[0]
	d.ID = strings.Repeat("b", 64)
	c.snapshot.Dependencies = append(c.snapshot.Dependencies, d)
	b := New(context.Background())
	b.connections[c.id] = c
	first, e := c.list([]byte(`{"limit":1}`))
	if e != nil || first.NextCursor == "" {
		t.Fatal("no cursor")
	}
	input, _ := json.Marshal(Filter{Limit: 1, Cursor: first.NextCursor, Revision: first.Revision})
	if _, e := c.list(input); e != nil {
		t.Fatal(e)
	}
	c.observation++
	if _, e := c.list(input); backend.Code(e) != "conflict" {
		t.Fatal("mixed observation pages")
	}
	_, e = b.Query(context.Background(), backend.Request{ConnectionID: "c", OperationID: "upgrades.get", Input: json.RawMessage(`{"dependencyId":"` + strings.Repeat("f", 64) + `"}`)})
	if backend.Code(e) != "not_found" {
		t.Fatal(e)
	}
}

func TestCandidateFilterDoesNotShowOtherUpdateTypes(t *testing.T) {
	s := testSnapshot(time.Now())
	value := "1.1"
	candidate := s.Dependencies[0].Candidates[0]
	candidate.NewValue = &value
	candidate.UpdateType = "minor"
	candidate.ID = snapshot.ID(s.Dependencies[0].ID, candidate.NewValue, candidate.NewDigest, candidate.UpdateType)
	s.Dependencies[0].Candidates = append(s.Dependencies[0].Candidates, candidate)
	c := &connection{id: "c", revision: "1", observation: 1, snapshot: s}
	list, err := c.list([]byte(`{"updateType":"minor"}`))
	if err != nil || len(list.Items) != 1 || len(list.Items[0].Candidates) != 1 || list.Items[0].Candidates[0].UpdateType != "minor" {
		t.Fatal("candidate filter leaked other update types")
	}
	if len(c.snapshot.Dependencies[0].Candidates) != 2 {
		t.Fatal("filter mutated the retained observation")
	}
}

func TestServiceMappingRequiresExactDependencyID(t *testing.T) {
	id := snapshot.ID("mapped dependency")
	service := snapshot.ID("service")
	c := &connection{config: Config{ServiceMappings: []Mapping{{DependencyID: id, ServiceID: service}}}}
	mapped := snapshot.Dependency{ID: id, PackageName: "nginx", PackageFile: "compose.yaml"}
	other := mapped
	other.ID = snapshot.ID("same image in another group")
	if c.item(mapped).ServiceID != service || c.item(other).ServiceID != "" {
		t.Fatal("service mapping guessed identity from image/file")
	}
	mapped.CurrentValue = new("2.0")
	if c.item(mapped).ServiceID != service {
		t.Fatal("version change broke exact mapping")
	}
}
