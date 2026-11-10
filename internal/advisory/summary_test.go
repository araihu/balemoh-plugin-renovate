package advisory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestSummaryCountsAllMatchingCandidatesAndLatestTrackedRequests(t *testing.T) {
	now := time.Now().UTC()
	s := testSnapshot(now)
	base := s.Dependencies[0]
	s.Dependencies = nil
	for i := 0; i < 9; i++ {
		d := base
		d.ID = snapshot.ID("group", i)
		u := base.Candidates[0]
		u.ID = snapshot.ID(d.ID, "major")
		u.IsVulnerabilityAlert = i == 0
		minor := u
		minor.ID = snapshot.ID(d.ID, "minor")
		minor.UpdateType = "minor"
		minor.IsVulnerabilityAlert = false
		d.Candidates = []snapshot.Candidate{u, minor}
		s.Dependencies = append(s.Dependencies, d)
	}
	d := s.Dependencies[0]
	record := requests.Record{ID: "record", Dependency: d, Candidate: d.Candidates[0], State: "open", CreatedAt: now, PR: &requests.PullRequest{URL: "https://github.com/example/read-only/pull/1"}}
	old := record
	old.State = "closed"
	old.CreatedAt = now.Add(-time.Hour)
	errorRecord := record
	errorRecord.Dependency = s.Dependencies[1]
	errorRecord.Candidate = s.Dependencies[1].Candidates[0]
	errorRecord.TrackingError = true
	errorRecord.PR = &requests.PullRequest{URL: "https://github.com/example/read-only/pull/2"}
	foreign := record
	foreign.Dependency.ID = "another-group"
	c := &connection{snapshot: s, controllerAvailable: true, controller: requests.Catalog{Requests: []requests.Record{old, record, errorRecord, foreign}}}
	result := c.summary(Filter{Assessment: "updates_found"})
	if result.Assessments["updates_found"] != 9 || result.Candidates != 18 || result.UpdateTypes["major"] != 9 || result.UpdateTypes["minor"] != 9 || result.SecurityCandidates != 1 || result.RequestStates["open"] != 1 || result.RequestStates["closed"] != 0 || result.UnconfirmedRequests != 1 {
		t.Fatalf("wrong complete summary: %+v", result)
	}
	result = c.summary(Filter{UpdateType: "minor"})
	if result.Candidates != 9 || result.SecurityCandidates != 0 || len(result.RequestStates) != 0 || result.UnconfirmedRequests != 0 {
		t.Fatal("request/type scope leaked", result)
	}
	result = c.summary(Filter{Repository: "foreign/repository"})
	if result.Candidates != 0 || len(result.Assessments) != 0 || len(result.RequestStates) != 0 {
		t.Fatal("repository scope leaked", result)
	}
	c.controller.Requests = append(c.controller.Requests, requests.Record{Dependency: s.Dependencies[2], Candidate: s.Dependencies[2].Candidates[0], State: "open", CreatedAt: now, PR: record.PR})
	if result = c.summary(Filter{}); result.RequestStates["open"] != 1 {
		t.Fatal("one PR counted twice", result)
	}
	if strings.Contains(result.Repository.Name, "foreign") {
		t.Fatal("summary changed source identity")
	}
}

func TestSummarySharedPRUsesFreshestEvidenceRegardlessOfOrder(t *testing.T) {
	now := time.Now().UTC()
	s := testSnapshot(now)
	d := s.Dependencies[0]
	second := d.Candidates[0]
	second.ID = snapshot.ID("second")
	d.Candidates = append(d.Candidates, second)
	s.Dependencies[0] = d
	old := requests.Record{Dependency: d, Candidate: d.Candidates[0], State: "open", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute), PR: &requests.PullRequest{URL: "https://github.com/example/read-only/pull/1"}}
	newest := old
	newest.Candidate = second
	newest.State = "merged"
	newest.TrackedAt = &now
	for _, tc := range []struct {
		name    string
		edit    func(*requests.Record)
		state   string
		unknown int
	}{
		{"newest merged", func(*requests.Record) {}, "merged", 0},
		{"newest failed refresh", func(r *requests.Record) { r.TrackingError = true }, "", 1},
		{"equal-time conflicting state", func(r *requests.Record) { r.TrackedAt = nil; r.UpdatedAt = old.UpdatedAt }, "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fresh := newest
			tc.edit(&fresh)
			for _, records := range [][]requests.Record{{old, fresh}, {fresh, old}} {
				c := &connection{snapshot: s, controller: requests.Catalog{Requests: records}}
				for i := 0; i < 1000; i++ {
					got := c.summary(Filter{})
					if got.UnconfirmedRequests != tc.unknown || tc.state != "" && (len(got.RequestStates) != 1 || got.RequestStates[tc.state] != 1) || tc.state == "" && len(got.RequestStates) != 0 {
						t.Fatalf("unstable shared PR: %+v", got)
					}
				}
			}
		})
	}
}

func TestSummarySchemaAcceptsMoreThanTenThousandSecurityCandidates(t *testing.T) {
	now := time.Now().UTC()
	s := testSnapshot(now)
	base := s.Dependencies[0]
	s.Dependencies = nil
	for i := 0; i < 501; i++ {
		d := base
		d.PackageFile = fmt.Sprintf("k8s/%d.yaml", i)
		d.ID = snapshot.ID(s.Repository.Platform, s.Repository.Name, s.Repository.BaseBranch, d.Manager, d.PackageFile, d.DependencyKey, d.PackageName)
		d.Candidates = nil
		for j := 0; j < 20; j++ {
			u := base.Candidates[0]
			u.NewValue = new(fmt.Sprintf("2.%d.0", j))
			u.IsVulnerabilityAlert = true
			u.ID = snapshot.ID(d.ID, u.NewValue, u.NewDigest, u.UpdateType)
			d.Candidates = append(d.Candidates, u)
		}
		s.Dependencies = append(s.Dependencies, d)
	}
	s.Coverage.SourceRecords = 501
	s.Coverage.Extracted = 501
	s.Coverage.Assessed = 501
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := snapshot.Decode(raw, now)
	if err != nil {
		t.Fatal("fixture must be an accepted snapshot", err)
	}
	c := &connection{snapshot: accepted}
	result := c.summary(Filter{})
	if result.SecurityCandidates != 10020 {
		t.Fatal("truncated security count", result.SecurityCandidates)
	}
	raw, err = os.ReadFile("../../schemas/summary-output.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource("urn:summary-test", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("urn:summary-test")
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err = decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if err = schema.Validate(value); err != nil {
		t.Fatal("valid candidate count rejected by output schema", err)
	}
}
