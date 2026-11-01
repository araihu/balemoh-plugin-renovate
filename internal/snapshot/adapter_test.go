package snapshot

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func metadata() Metadata {
	now := time.Now().UTC().Truncate(time.Second)
	commit := strings.Repeat("a", 40)
	return Metadata{RunID: "00000000-0000-4000-8000-000000000001", StartedAt: now.Add(-time.Minute), CompletedAt: now, Producer: Producer{RenovateVersion, RenovateImage, AdapterVersion, ID("fixture-policy")}, Repository: Repository{"github", "example/read-only", "main", commit}, DryRun: "full", CheckoutCommit: commit, BeforeCommit: commit, AfterCommit: commit}
}
func fixture(t *testing.T) []byte {
	t.Helper()
	data, e := os.ReadFile("testdata/report.json")
	if e != nil {
		t.Fatal(e)
	}
	return data
}
func TestControlledReport(t *testing.T) {
	s, e := Normalize(fixture(t), metadata())
	if e != nil {
		t.Fatal(e)
	}
	if s.Coverage.Status != "partial" || s.Coverage.Unknown < 3 || s.Coverage.Assessed == 0 || s.Coverage.Excluded != 1 {
		t.Fatalf("coverage: %+v", s.Coverage)
	}
	ambiguous, age, digest := false, false, false
	managers := map[string]bool{}
	for _, d := range s.Dependencies {
		managers[d.Manager] = true
		if d.PackageFile == "k8s/duplicates.yaml" {
			ambiguous = d.Assessment == "unknown" && len(d.Candidates) == 0 && d.SourceOccurrences == 2 && d.CurrentValue == nil
		}
		for _, u := range d.Candidates {
			age = age || u.Policy == "pending_checks"
			digest = digest || u.UpdateType == "digest"
		}
	}
	if !ambiguous || !age || !digest || len(managers) != 6 {
		t.Fatalf("missing fixture evidence: ambiguity=%t age=%t digest=%t managers=%d", ambiguous, age, digest, len(managers))
	}
	data, _ := json.Marshal(s)
	if _, e := Decode(data, time.Now()); e != nil {
		t.Fatal(e)
	}
}
func TestProducerProof(t *testing.T) {
	for _, mutation := range []func(*Metadata){func(m *Metadata) { m.ExitCode = 1 }, func(m *Metadata) { m.DryRun = "extract" }, func(m *Metadata) { m.CheckoutCommit = "" }, func(m *Metadata) { m.AfterCommit = strings.Repeat("b", 40) }, func(m *Metadata) { m.Producer.RenovateVersion = "latest" }} {
		m := metadata()
		mutation(&m)
		if _, e := Normalize(fixture(t), m); e == nil {
			t.Fatal("accepted unverified run")
		}
	}
}
func TestValidationRejectsTampering(t *testing.T) {
	s, e := Normalize(fixture(t), metadata())
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Snapshot){func(s *Snapshot) { s.Dependencies[0].ID = strings.Repeat("b", 64) }, func(s *Snapshot) { s.Coverage.Status = "complete" }, func(s *Snapshot) { s.Coverage.SourceRecords = 0 }, func(s *Snapshot) { s.RunID = "not-a-uuid" }, func(s *Snapshot) { s.CompletedAt = time.Now().Add(time.Hour) }, func(s *Snapshot) { s.Dependencies[0].PackageFile = "../secret" }} {
		data, _ := json.Marshal(s)
		var copy Snapshot
		_ = json.Unmarshal(data, &copy)
		change(&copy)
		encoded, _ := json.Marshal(copy)
		if _, e := Decode(encoded, time.Now()); e == nil {
			t.Fatal("accepted invalid snapshot")
		}
	}
}
func TestOnlyReviewedManagers(t *testing.T) {
	var raw map[string]any
	_ = json.Unmarshal(fixture(t), &raw)
	repo := raw["repositories"].(map[string]any)["example/read-only"].(map[string]any)
	repo["packageFiles"].(map[string]any)["argocd"] = []any{map[string]any{"packageFile": "chart.yaml", "deps": []any{map[string]any{"datasource": "docker", "depName": "oci/chart", "updates": []any{}}}}}
	data, _ := json.Marshal(raw)
	s, e := Normalize(data, metadata())
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range s.Dependencies {
		if d.Manager == "argocd" {
			t.Fatal("OCI chart counted as image")
		}
	}
}
func TestPRLinksRequireExactAssociation(t *testing.T) {
	data := []byte(`{"repositories":{"example/read-only":{"status":"activated","packageFiles":{"kubernetes":[{"packageFile":"a.yaml","deps":[{"depName":"nginx","datasource":"docker","currentValue":"1","updates":[{"newValue":"2","updateType":"major","branchName":"renovate/nginx"}]}]}]},"branches":[{"branchName":"renovate/nginx","prNo":123,"result":"pr-edited","upgrades":[{"packageFile":"a.yaml","packageName":"nginx","depName":"nginx","manager":"kubernetes","updateType":"major","currentValue":"1","newValue":"2"}]}]}}}`)
	s, e := Normalize(data, metadata())
	if e != nil {
		t.Fatal(e)
	}
	c := s.Dependencies[0].Candidates[0]
	if c.PR == nil || c.PR.Number != 123 || c.Policy != "unknown" || c.Workflow != "pr_open" {
		t.Fatal(c)
	}
	s.Dependencies[0].Candidates[0].PR.URL = "https://attacker.example/pull/123"
	encoded, _ := json.Marshal(s)
	if _, e := Decode(encoded, time.Now()); e == nil {
		t.Fatal("accepted foreign PR URL")
	}
}

func TestSecurityFlagRequiresExactEvidence(t *testing.T) {
	base := `{"repositories":{"example/read-only":{"status":"activated","packageFiles":{"kubernetes":[{"packageFile":"a.yaml","deps":[{"depName":"nginx","datasource":"docker","currentValue":"1","updates":[{"newValue":"2","updateType":"major","branchName":"renovate/nginx"}]}]}]},"branches":[{"branchName":"renovate/nginx","upgrades":[{"packageFile":"a.yaml","packageName":"nginx","depName":"nginx","manager":"kubernetes","updateType":"major","currentValue":"1","newValue":"2","isVulnerabilityAlert":true}]}]}}}`
	for _, tc := range []struct {
		name, old, next string
		want            bool
	}{
		{name: "branch flag alone is not evidence"},
		{name: "wrong manager", old: `"manager":"kubernetes"`, next: `"manager":"regex"`},
		{name: "wrong file", old: `"packageFile":"a.yaml","packageName"`, next: `"packageFile":"other.yaml","packageName"`},
		{name: "wrong update type", old: `"updateType":"major","currentValue"`, next: `"updateType":"patch","currentValue"`},
		{name: "wrong dependency key", old: `"manager":"kubernetes"`, next: `"manager":"kubernetes","depType":"other"`},
		{name: "wrong current digest", old: `"isVulnerabilityAlert":true`, next: `"isVulnerabilityAlert":true,"currentDigest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`},
		{name: "branch title only", old: `"isVulnerabilityAlert":true`, next: `"prTitle":"[SECURITY] Fix CVE","labels":["security"]`},
		{name: "false is unknown", old: `"isVulnerabilityAlert":true`, next: `"isVulnerabilityAlert":false`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := base
			if tc.old != "" {
				data = strings.Replace(data, tc.old, tc.next, 1)
			}
			s, err := Normalize([]byte(data), metadata())
			if err != nil {
				t.Fatal(err)
			}
			c := s.Dependencies[0].Candidates[0]
			if c.IsVulnerabilityAlert != tc.want {
				t.Fatal("incorrect security attribution")
			}
		})
	}
	direct := strings.Replace(base, `"branchName":"renovate/nginx"}`, `"branchName":"renovate/nginx","isVulnerabilityAlert":true}`, 1)
	s, err := Normalize([]byte(direct), metadata())
	if err != nil || !s.Dependencies[0].Candidates[0].IsVulnerabilityAlert {
		t.Fatal("explicit update flag lost", err)
	}
	ordinary, _ := Normalize([]byte(strings.ReplaceAll(base, `"isVulnerabilityAlert":true`, `"isVulnerabilityAlert":false`)), metadata())
	if ordinary.Dependencies[0].Candidates[0].ID != s.Dependencies[0].Candidates[0].ID {
		t.Fatal("security enrichment changed candidate identity")
	}
	bad := strings.Replace(direct, `"isVulnerabilityAlert":true`, `"isVulnerabilityAlert":"true"`, 1)
	if _, err := Normalize([]byte(bad), metadata()); err == nil {
		t.Fatal("coerced security flag")
	}
}
