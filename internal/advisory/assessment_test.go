package advisory

import (
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestAssessmentBreakdownKeepsEmptyReportsDistinctFromProblems(t *testing.T) {
	s := testSnapshot(time.Now())
	s.Dependencies = nil
	for _, tc := range []struct {
		assessment string
		reasons    []string
		kind       string
	}{
		{"updates_found", nil, "updates_found"},
		{"unknown", []string{"negative_assessment_unproven"}, "no_candidates_reported"},
		{"unknown", []string{"lookup_unconfirmed"}, "lookup_unconfirmed"},
		{"unknown", []string{"ambiguous_declarations"}, "conflicting_declarations"},
		{"unknown", []string{"negative_assessment_unproven", "lookup_unconfirmed"}, "lookup_unconfirmed"},
		{"unknown", []string{"negative_assessment_unproven", "future_reason"}, "unknown"},
		{"unknown", nil, "unknown"},
		{"no_candidate", nil, "no_candidate"},
		{"excluded", []string{"disabled"}, "excluded"},
	} {
		d := snapshot.Dependency{Assessment: tc.assessment, Reasons: tc.reasons, SourceOccurrences: 2}
		if got := AssessmentKind(d); got != tc.kind {
			t.Fatalf("%v: %s", tc, got)
		}
		s.Dependencies = append(s.Dependencies, d)
	}
	s.Coverage.SourceRecords = 21
	s.Coverage.Unknown = 6
	c := &connection{snapshot: s}
	got := c.meta().AssessmentBreakdown
	want := AssessmentBreakdown{UpdatesFound: 1, NoCandidatesReported: 1, LookupUnconfirmed: 2, ConflictingDeclarations: 1, OtherUnknown: 2, NoCandidate: 1, Excluded: 1, UnidentifiedRecords: 3}
	if got != want {
		t.Fatalf("breakdown=%+v", got)
	}
	if got.UpdatesFound+got.NoCandidatesReported+got.LookupUnconfirmed+got.ConflictingDeclarations+got.OtherUnknown+got.NoCandidate+got.Excluded != len(s.Dependencies) {
		t.Fatal("group totals do not reconcile")
	}
	filtered, err := c.list([]byte(`{"assessment":"updates_found","limit":1}`))
	if err != nil || filtered.Total != 1 || filtered.AssessmentBreakdown != want {
		t.Fatalf("filter changed repository-wide breakdown: %v %+v", err, filtered.AssessmentBreakdown)
	}

	if s.Dependencies[1].Assessment != "unknown" {
		t.Fatal("presentation changed source assessment")
	}
	if !c.matches(s.Dependencies[1], Filter{Assessment: "unknown"}) {
		t.Fatal("legacy unknown filter lost empty reports")
	}
}

func TestLookupCauseCountsDoNotDoubleCountGroupsOrInventLegacyCauses(t *testing.T) {
	s := testSnapshot(time.Now())
	s.Dependencies = []snapshot.Dependency{
		{Assessment: "unknown", SourceOccurrences: 2, Reasons: []string{"lookup_unconfirmed", "cause_latest_tag_missing"}},
		{Assessment: "unknown", Reasons: []string{"lookup_unconfirmed"}},
		{Assessment: "unknown", Reasons: []string{"negative_assessment_unproven"}},
		{Assessment: "excluded", Reasons: []string{"disabled"}},
	}
	counts := lookupCauseCounts(s)
	if len(counts) != 2 || counts["cause_latest_tag_missing"] != 1 || counts["cause_not_recorded"] != 1 {
		t.Fatalf("wrong counts: %+v", counts)
	}
}
