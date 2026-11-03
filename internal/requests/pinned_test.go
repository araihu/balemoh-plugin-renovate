package requests

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

type bindingCase struct {
	Case, Manager, File, Branch, Head, NativeMismatchRule string
	PR                                                    int
	Preflight                                             json.RawMessage
}

func bindingProof(t *testing.T) (string, []bindingCase) {
	t.Helper()
	data, err := os.ReadFile("../../docs/qualification/digest-binding/qualification.json")
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Qualification, ApprovedBase string
		Cases                       []bindingCase
	}
	if json.Unmarshal(data, &proof) != nil || proof.Qualification != "passed" || len(proof.Cases) != 3 {
		t.Fatal("missing native proof")
	}
	return proof.ApprovedBase, proof.Cases
}

func bindingRecord(t *testing.T, tc bindingCase) Record {
	t.Helper()
	base, _ := bindingProof(t)
	r := Record{Repository: snapshot.Repository{Platform: "github", Name: "test/repo", BaseBranch: "main", Commit: base}, State: "running", SnapshotDigest: strings.Repeat("a", 64), CreatedAt: time.Now().UTC().Add(-time.Minute), UpdatedAt: time.Now().UTC()}
	if json.Unmarshal(tc.Preflight, &r.Dependency) != nil || json.Unmarshal(tc.Preflight, &r.Candidate) != nil {
		t.Fatal("invalid proof")
	}
	r.Dependency.Manager = tc.Manager
	r.Dependency.DependencyKey = `["127.0.0.1:5500/fixture/image",""]`
	r.Dependency.Assessment = "updates_found"
	r.Dependency.SourceOccurrences = 1
	r.Dependency.ID = snapshot.ID(r.Repository.Platform, r.Repository.Name, r.Repository.BaseBranch, r.Dependency.Manager, r.Dependency.PackageFile, r.Dependency.DependencyKey, r.Dependency.PackageName)
	r.Candidate.ID = snapshot.ID(r.Dependency.ID, r.Candidate.NewValue, r.Candidate.NewDigest, r.Candidate.UpdateType)
	r.ID = snapshot.ID(r.Repository, r.Dependency.ID, r.Candidate.ID)
	r.Branch = "balemoh-renovate/" + r.ID
	return r
}

func TestPinnedAdmissionAndQualifiedNativeBinding(t *testing.T) {
	_, cases := bindingProof(t)
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			r := bindingRecord(t, tc)
			if !Supported(r.Dependency, r.Candidate) || r.Validate() != nil {
				t.Fatal("qualified pinned shape rejected")
			}
			read, write := engineProfile(r, "/work/test", false), engineProfile(r, "/work/test", true)
			read["dryRun"] = nil
			a, _ := json.Marshal(read)
			b, _ := json.Marshal(write)
			if string(a) != string(b) {
				t.Fatal("writer changes approved scope")
			}
			force := write["force"].(map[string]any)
			rules := force["packageRules"].([]any)
			if force["pinDigests"] != true || len(rules) != 3 {
				t.Fatal("native digest guard missing")
			}
			rule := rules[2].(map[string]any)
			if rule["enabled"] != false || rule["matchJsonata"].([]string)[0] != tc.NativeMismatchRule {
				t.Fatal("binding differs from live-qualified native rule")
			}
			if r.Candidate.NewValue == nil {
				if _, ok := rules[1].(map[string]any)["allowedVersions"]; ok {
					t.Fatal("invented tag constraint")
				}
			}
			for _, mutate := range []struct {
				name   string
				change func(*Record)
			}{
				{"missing-old-digest", func(r *Record) { r.Dependency.CurrentDigest = nil }},
				{"missing-new-digest", func(r *Record) { r.Candidate.NewDigest = nil }},
				{"uppercase-digest", func(r *Record) { r.Candidate.NewDigest = new("sha256:" + strings.Repeat("A", 64)) }},
				{"short-digest", func(r *Record) { r.Candidate.NewDigest = new("sha256:abcd") }},
				{"other-algorithm", func(r *Record) { r.Candidate.NewDigest = new("sha512:" + strings.Repeat("a", 128)) }},
				{"pin", func(r *Record) { r.Candidate.UpdateType = "pinDigest" }},
				{"regex", func(r *Record) { r.Dependency.Manager = "regex" }},
				{"ignored", func(r *Record) { r.Candidate.Policy = "ignored" }},
				{"mixed-tag-presence", func(r *Record) { r.Dependency.CurrentValue = nil; r.Candidate.NewValue = new("1.0.1") }},
			} {
				t.Run(mutate.name, func(t *testing.T) {
					bad := r
					mutate.change(&bad)
					if Supported(bad.Dependency, bad.Candidate) {
						t.Fatal("unsupported pinned shape admitted")
					}
				})
			}
			if r.Candidate.UpdateType == "digest" {
				same := r
				same.Candidate.NewDigest = same.Dependency.CurrentDigest
				if Supported(same.Dependency, same.Candidate) {
					t.Fatal("unchanged digest admitted")
				}
				changed := r
				changed.Candidate.NewValue = new("2.0.0")
				if Supported(changed.Dependency, changed.Candidate) {
					t.Fatal("digest update changed tag")
				}
			}
		})
	}
}

func TestPinnedPreflightRequiresFullApprovedReference(t *testing.T) {
	_, cases := bindingProof(t)
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			r := bindingRecord(t, tc)
			var upgrade map[string]any
			_ = json.Unmarshal(tc.Preflight, &upgrade)
			plan := func(row map[string]any) []byte {
				data, _ := json.Marshal(map[string]any{"repositories": map[string]any{r.Repository.Name: map[string]any{"branches": []any{map[string]any{"branchName": r.Branch, "result": "no-work", "upgrades": []any{row}}}}}})
				return data
			}
			if verifyFullDryRunPlan(plan(upgrade), r) != nil {
				t.Fatal("native approved plan rejected")
			}
			for _, field := range []string{"currentValue", "newValue", "currentDigest", "newDigest", "updateType", "packageFile", "packageName"} {
				for _, value := range []any{nil, "unapproved"} {
					t.Run(field+"/"+string(mustJSON(t, value)), func(t *testing.T) {
						bad := map[string]any{}
						for k, v := range upgrade {
							bad[k] = v
						}
						bad[field] = value
						if upgrade[field] == value {
							return
						}
						if verifyFullDryRunPlan(plan(bad), r) == nil {
							t.Fatal("changed or missing reference evidence admitted")
						}
					})
				}
			}
		})
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestPinnedSubmitRetainsApprovedTargetAndDeduplicates(t *testing.T) {
	_, cases := bindingProof(t)
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			c, _ := setup(t)
			r := bindingRecord(t, tc)
			data, err := os.ReadFile(c.config.SnapshotFile)
			if err != nil {
				t.Fatal(err)
			}
			var s snapshot.Snapshot
			if json.Unmarshal(data, &s) != nil {
				t.Fatal("invalid setup snapshot")
			}
			r.Candidate.Policy = "unknown"
			r.Candidate.Workflow = "unknown"
			r.Dependency.Reasons = []string{}
			r.Dependency.Candidates = []snapshot.Candidate{r.Candidate}
			s.Repository = r.Repository
			s.Dependencies = []snapshot.Dependency{r.Dependency}
			data, err = json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = snapshot.Decode(data, time.Now()); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(c.config.SnapshotFile, data, 0600); err != nil {
				t.Fatal(err)
			}
			selection := Selection{DependencyID: r.Dependency.ID, CandidateID: r.Candidate.ID, SnapshotDigest: Digest(&s)}
			first, err := c.Submit(t.Context(), selection)
			if err != nil {
				t.Fatal(err)
			}
			second, err := c.Submit(t.Context(), selection)
			if err != nil || second.ID != first.ID || len(c.store.list()) != 1 {
				t.Fatal("duplicate pinned approval", err)
			}
			if string(mustJSON(t, first.Candidate)) != string(mustJSON(t, r.Candidate)) || string(mustJSON(t, first.Dependency)) != string(mustJSON(t, r.Dependency)) {
				t.Fatal("approval lost target reference")
			}
		})
	}
}
