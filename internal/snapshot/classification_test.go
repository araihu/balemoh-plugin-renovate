package snapshot

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestScopeClassificationKeepsActiveAndMalformedRecordsVisible(t *testing.T) {
	value := func(s string) *string { return &s }
	for _, tc := range []struct {
		name, manager, assessment, reason, scope, file string
		rows                                           []rawDependency
		records, groups                                int
	}{
		{name: "different disabled tags", manager: "helm-values", rows: []rawDependency{{Name: "nginx", Current: value("1"), Skip: "disabled"}, {Name: "nginx", Current: value("2"), Skip: "disabled"}}, records: 2, groups: 1, assessment: "excluded", reason: "disabled"},
		{name: "disabled and active", manager: "helm-values", rows: []rawDependency{{Name: "nginx", Current: value("1"), Skip: "disabled"}, {Name: "nginx", Current: value("2")}}, records: 2, groups: 1, assessment: "unknown", reason: "ambiguous_declarations"},
		{name: "named inherited Helm tag", manager: "helm-values", rows: []rawDependency{{Name: "nginx", Current: value(""), Skip: "invalid-value"}}, records: 1, groups: 1, assessment: "excluded", reason: "chart_inherited"},
		{name: "inherited and explicit", manager: "helm-values", rows: []rawDependency{{Name: "nginx", Current: value(""), Skip: "invalid-value"}, {Name: "nginx", Current: value("2")}}, records: 2, groups: 1, assessment: "unknown", reason: "ambiguous_declarations"},
		{name: "blank Helm override", manager: "helm-values", rows: []rawDependency{{Current: value(""), Skip: "invalid-name"}}, scope: "Empty Helm override records outside these counts: 1"},
		{name: "Dockerfile build argument", manager: "dockerfile", rows: []rawDependency{{Skip: "contains-variable"}}, scope: "Unresolved Dockerfile build-argument records outside these counts: 1"},
		{name: "unsafe inherited path", manager: "helm-values", file: "../secret", rows: []rawDependency{{Skip: "invalid-name"}}, records: 1, reason: "missing_identity"},
		{name: "invalid Dockerfile reference", manager: "dockerfile", rows: []rawDependency{{Skip: "invalid-name"}}, records: 1, reason: "missing_identity"},
		{name: "empty repository with explicit tag", manager: "helm-values", rows: []rawDependency{{Current: value("2"), Skip: "invalid-name"}}, records: 1, reason: "missing_identity"},
		{name: "blank Helm override with warning", manager: "helm-values", rows: []rawDependency{{Skip: "invalid-name", Warnings: []json.RawMessage{json.RawMessage(`{"message":"unexpected failure"}`)}}}, records: 1, reason: "missing_identity"},
		{name: "digest-only Helm image", manager: "helm-values", rows: []rawDependency{{Name: "nginx", Digest: value("sha256:" + strings.Repeat("a", 64)), Skip: "invalid-value"}}, records: 1, groups: 1, assessment: "unknown", reason: "lookup_unconfirmed"},
		{name: "ordinary empty result", manager: "kubernetes", rows: []rawDependency{{Name: "nginx", Current: value("1")}}, records: 1, groups: 1, assessment: "unknown", reason: "negative_assessment_unproven"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := range tc.rows {
				tc.rows[i].Datasource = "docker"
			}
			file := tc.file
			if file == "" {
				file = "images.yaml"
			}
			report := rawReport{Repositories: map[string]rawRepo{"example/read-only": {Status: "onboarded", Files: map[string][]rawFile{tc.manager: {{File: file, Deps: tc.rows}}}}}}
			data, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			s, err := Normalize(data, metadata())
			if err != nil {
				t.Fatal(err)
			}
			if s.Coverage.SourceRecords != tc.records || len(s.Dependencies) != tc.groups {
				t.Fatalf("unexpected counts: %+v", s.Coverage)
			}
			if tc.groups > 0 {
				d := s.Dependencies[0]
				if d.Assessment != tc.assessment || !slices.Contains(d.Reasons, tc.reason) || len(d.Candidates) != 0 {
					t.Fatalf("unexpected classification: %+v", d)
				}
				if tc.name == "different disabled tags" && (d.CurrentValue != nil || d.CurrentDigest != nil) {
					t.Fatal("arbitrarily selected a disabled version")
				}
			} else if tc.reason != "" && !slices.Contains(s.Coverage.Reasons, tc.reason) {
				t.Fatalf("lost error: %+v", s.Coverage)
			}
			if tc.scope != "" && !strings.Contains(s.Coverage.Scope, tc.scope) {
				t.Fatalf("scope omission not explained: %s", s.Coverage.Scope)
			}
			encoded, _ := json.Marshal(s)
			if _, err := Decode(encoded, time.Now()); err != nil {
				t.Fatalf("broke snapshot v1: %v", err)
			}
		})
	}
}
