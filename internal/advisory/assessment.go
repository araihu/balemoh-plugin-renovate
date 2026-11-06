package advisory

import (
	"slices"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

// AssessmentKind separates an empty report from explicit lookup or identity problems.
// It never upgrades an unknown assessment to an up-to-date conclusion.
func AssessmentKind(d snapshot.Dependency) string {
	if d.Assessment != "unknown" {
		return d.Assessment
	}
	switch {
	case slices.Contains(d.Reasons, "ambiguous_declarations"):
		return "conflicting_declarations"
	case slices.Contains(d.Reasons, "lookup_unconfirmed"):
		return "lookup_unconfirmed"
	case len(d.Reasons) == 1 && d.Reasons[0] == "negative_assessment_unproven":
		return "no_candidates_reported"
	default:
		return "unknown"
	}
}

type AssessmentBreakdown struct {
	UpdatesFound            int `json:"updatesFound"`
	NoCandidatesReported    int `json:"noCandidatesReported"`
	LookupUnconfirmed       int `json:"lookupUnconfirmed"`
	ConflictingDeclarations int `json:"conflictingDeclarations"`
	OtherUnknown            int `json:"otherUnknown"`
	NoCandidate             int `json:"noCandidate"`
	Excluded                int `json:"excluded"`
	UnidentifiedRecords     int `json:"unidentifiedRecords"`
}

func assessmentBreakdown(s *snapshot.Snapshot) AssessmentBreakdown {
	b := AssessmentBreakdown{}
	occurrences := 0
	for _, d := range s.Dependencies {
		occurrences += max(d.SourceOccurrences, 1)
		switch AssessmentKind(d) {
		case "updates_found":
			b.UpdatesFound++
		case "no_candidates_reported":
			b.NoCandidatesReported++
		case "lookup_unconfirmed":
			b.LookupUnconfirmed++
		case "conflicting_declarations":
			b.ConflictingDeclarations++
		case "no_candidate":
			b.NoCandidate++
		case "excluded":
			b.Excluded++
		default:
			b.OtherUnknown++
		}
	}
	b.UnidentifiedRecords = max(0, s.Coverage.SourceRecords-occurrences)
	return b
}

// LookupCause returns one recorded cause per group; older snapshots retain a clear fallback.
func LookupCause(d snapshot.Dependency) string {
	for _, r := range d.Reasons {
		switch r {
		case "cause_registry_access_denied", "cause_registry_rate_limited", "cause_internal_host_blocked", "cause_latest_tag_missing", "cause_manifest_tag_missing", "cause_empty_helm_tag", "cause_invalid_value", "cause_lookup_skipped", "cause_digest_resolution_failed", "cause_lookup_failed":
			return r
		}
	}
	return "cause_not_recorded"
}
func lookupCauseCounts(s *snapshot.Snapshot) map[string]int {
	counts := map[string]int{}
	for _, d := range s.Dependencies {
		if d.Assessment == "unknown" && slices.Contains(d.Reasons, "lookup_unconfirmed") {
			counts[LookupCause(d)]++
		}
	}
	return counts
}
