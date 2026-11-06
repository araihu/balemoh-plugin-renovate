package view

import (
	"bytes"
	"strings"
	"testing"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestCoverageExplainsInheritedAndTemplateScope(t *testing.T) {
	m := Model{Meta: advisory.Meta{Coverage: snapshot.Coverage{Scope: "8 empty Helm overrides and 1 build template are outside image counts."}, AssessmentBreakdown: advisory.AssessmentBreakdown{Excluded: 2}}}
	var out bytes.Buffer
	if err := TechnicalEvidence(m).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{m.Meta.Coverage.Scope, "Collection documentation"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("missing scope explanation %q", text)
		}
	}
	if got := assessmentLabel(advisory.Item{Dependency: snapshot.Dependency{Assessment: "excluded", Reasons: []string{"chart_inherited"}}}); got != "Inherited from chart" {
		t.Fatal(got)
	}
}

func TestEmptyReportsAreNeutralWhileCollectionProblemsRemainVisible(t *testing.T) {
	for _, tc := range []struct {
		name     string
		b        advisory.AssessmentBreakdown
		coverage snapshot.Coverage
		warning  bool
	}{
		{"empty reports only", advisory.AssessmentBreakdown{NoCandidatesReported: 123}, snapshot.Coverage{Status: "partial", Unknown: 123, Reasons: []string{"unassessed_dependencies"}}, false},
		{"lookup and conflicts", advisory.AssessmentBreakdown{NoCandidatesReported: 123, LookupUnconfirmed: 17, ConflictingDeclarations: 1, UnidentifiedRecords: 9}, snapshot.Coverage{Status: "partial", Unknown: 141, Reasons: []string{"source_problems", "missing_identity", "unassessed_dependencies"}}, true},
		{"repository problem without affected group", advisory.AssessmentBreakdown{}, snapshot.Coverage{Status: "partial", Reasons: []string{"source_problems"}}, true},
		{"future unknown reason", advisory.AssessmentBreakdown{OtherUnknown: 1}, snapshot.Coverage{Status: "partial", Unknown: 1}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{Meta: advisory.Meta{AssessmentBreakdown: tc.b, Coverage: tc.coverage}, CoverageURL: "/coverage?assessment=updates_found"}
			var out bytes.Buffer
			if err := ObservationWarnings(m).Render(t.Context(), &out); err != nil {
				t.Fatal(err)
			}
			body := out.String()
			if strings.Contains(body, "Collection needs review") != tc.warning {
				t.Fatalf("wrong warning state: %s", body)
			}
			if strings.Contains(body, "141 image/file groups are unassessed") {
				t.Fatal("old mixed warning retained")
			}
			if tc.b.NoCandidatesReported > 0 && !strings.Contains(body, "Across this repository, 123 image/file groups have no candidates reported.") {
				t.Fatal("neutral state missing repository scope")
			}
			if tc.b.LookupUnconfirmed == 17 {
				for _, want := range []string{"18 image/file groups need review.", "9 source records could not be identified.", "Counts cover the whole repository."} {
					if !strings.Contains(body, want) {
						t.Fatalf("missing %s", want)
					}
				}
			}
		})
	}
}
func TestAssessmentLabelsAndCoverageExplainUncertainty(t *testing.T) {
	for _, tc := range []struct{ reason, want string }{
		{"negative_assessment_unproven", "No candidates reported"},
		{"lookup_unconfirmed", "Lookup unconfirmed"},
		{"ambiguous_declarations", "Conflicting declarations"},
		{"future_reason", "Upgrade status unknown"},
	} {
		d := advisory.Item{Dependency: snapshot.Dependency{Assessment: "unknown", Reasons: []string{tc.reason}}}
		if got := assessmentLabel(d); got != tc.want {
			t.Fatal(got)
		}
	}
	m := Model{Meta: advisory.Meta{AssessmentBreakdown: advisory.AssessmentBreakdown{NoCandidatesReported: 123, LookupUnconfirmed: 17, ConflictingDeclarations: 1, UnidentifiedRecords: 9}}}
	var out bytes.Buffer
	if err := CoverageSummary(m).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"regardless of list filters", "does not confirm that the images are up to date", "outside these group counts", "17", "123"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestCoverageUsesTablesAndKeepsWarningsFindable(t *testing.T) {
	m := Model{Meta: advisory.Meta{Coverage: snapshot.Coverage{Extracted: 182, SourceRecords: 210, Unknown: 131, Reasons: []string{"source_problems", "unassessed_dependencies"}}, AssessmentBreakdown: advisory.AssessmentBreakdown{UpdatesFound: 39, NoCandidatesReported: 115, LookupUnconfirmed: 16, Excluded: 12}, LookupCauses: map[string]int{"cause_latest_tag_missing": 9, "cause_registry_access_denied": 3, "cause_registry_rate_limited": 1, "cause_internal_host_blocked": 3}}}
	m.Result.State = "stale"
	var out bytes.Buffer
	if err := Coverage(m).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{"<table", "<caption", "182 image/file groups", "210 declarations", "16 groups need review", "Stale observation", "Default latest tag not found", "Add an explicit image tag.", "Check package access.", "About No candidates reported", "Scope and technical details"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Count(body, "<table ") != 2 || strings.Contains(body, `role="alert"`) || strings.Contains(body, "The collector publishes fixed cause labels") {
		t.Fatal("coverage must use two tables without duplicate warning banners or implementation prose")
	}
	if strings.Contains(body, "Conflicting declarations") {
		t.Fatal("zero conflict row adds noise")
	}
}
func TestCoverageKeepsUnidentifiedAndRepositoryFailuresVisible(t *testing.T) {
	for _, tc := range []struct {
		name    string
		b       advisory.AssessmentBreakdown
		reasons []string
		want    string
	}{
		{"unidentified", advisory.AssessmentBreakdown{UnidentifiedRecords: 9}, []string{"missing_identity"}, "9 declarations could not be identified"},
		{"repository failure", advisory.AssessmentBreakdown{}, []string{"source_problems"}, "Collection needs review"},
		{"unknown", advisory.AssessmentBreakdown{OtherUnknown: 1}, nil, "Other unknown status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{Meta: advisory.Meta{AssessmentBreakdown: tc.b, Coverage: snapshot.Coverage{Unknown: tc.b.OtherUnknown, Reasons: tc.reasons}}}
			var out bytes.Buffer
			if err := Coverage(m).Render(t.Context(), &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatal("lost runtime warning", tc.want)
			}
			if strings.Count(out.String(), "<table ") != 1 {
				t.Fatal("empty warning table should be omitted")
			}
		})
	}
}

func TestCurrentOmitsRepeatedCopyButKeepsObservationMetadata(t *testing.T) {
	m := Model{Meta: advisory.Meta{Coverage: snapshot.Coverage{Status: "partial", Unknown: 131, Reasons: []string{"source_problems"}}, AssessmentBreakdown: advisory.AssessmentBreakdown{NoCandidatesReported: 115, LookupUnconfirmed: 16}}}
	m.Result.State = "stale"
	var out bytes.Buffer
	if err := Current(m).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, removed := range []string{"Renovate candidates from Git", "Observation is stale", "Collection needs review", "Filters apply automatically.", "Across this repository"} {
		if strings.Contains(body, removed) {
			t.Fatalf("list retained removed copy %q", removed)
		}
	}
	for _, retained := range []string{"Observed at", "Stale observation", "Partial", `data-filter-status role="status" aria-live="polite"></p>`} {
		if !strings.Contains(body, retained) {
			t.Fatalf("list lost metadata or live error region %q", retained)
		}
	}
}
