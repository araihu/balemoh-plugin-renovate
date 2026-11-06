package view

import (
	"github.com/araihu/goshtoso/components/badge"
	"github.com/araihu/goshtoso/components/table"
)

func (m Model) coverageTable() table.Config {
	b := m.Meta.AssessmentBreakdown
	rows := []table.Row{}
	for _, outcome := range []struct {
		key, title, hint string
		total            int
	}{
		{"candidates", "Candidates reported", "Renovate found one or more upgrade candidates for this image/file group.", b.UpdatesFound},
		{"empty", "No candidates reported", "Renovate returned no upgrades. This does not confirm that the images are up to date.", b.NoCandidatesReported},
		{"lookup", "Lookup unconfirmed", "A lookup was skipped or returned a warning. See the causes below.", b.LookupUnconfirmed},
		{"conflict", "Conflicting declarations", "Records for the same image/file disagree. Resolve the declarations before selecting an upgrade.", b.ConflictingDeclarations},
		{"excluded", "Outside upgrade scope", "Disabled by policy or inherited from a Helm chart. These are not lookup failures.", b.Excluded},
		{"policy", "No candidate under policy", "No upgrade candidate passed the reported policy.", b.NoCandidate},
		{"unknown", "Other unknown status", "The report did not establish an upgrade assessment. Open image details to inspect the recorded reason.", b.OtherUnknown},
	} {
		if outcome.total == 0 && outcome.key != "candidates" && outcome.key != "empty" {
			continue
		}
		rows = append(rows, table.Row{ID: outcome.key, Cells: map[string]table.Cell{"outcome": {Component: CoverageLabel(m, outcome.key, outcome.title, outcome.hint)}, "groups": {Text: count(outcome.total)}}})
	}
	return table.Config{ID: m.Request.DOMID("coverage-outcomes"), Caption: "Collection outcomes across the whole repository", RootClass: "image-upgrades-coverage-table", Columns: []table.Column{{Key: "outcome", Label: "Outcome"}, {Key: "groups", Label: "Groups", Align: "right"}}, Rows: rows}
}
func (m Model) causesTable() table.Config {
	rows := []table.Row{}
	for _, cause := range lookupCauseOrder {
		if n := m.Meta.LookupCauses[cause]; n > 0 {
			rows = append(rows, table.Row{ID: cause, Cells: map[string]table.Cell{"cause": {Component: CoverageCause(m, cause)}, "groups": {Text: count(n)}}})
		}
	}
	return table.Config{ID: m.Request.DOMID("coverage-causes"), Caption: "Lookup warning causes and next steps", RootClass: "image-upgrades-coverage-table", Columns: []table.Column{{Key: "cause", Label: "Cause / next step"}, {Key: "groups", Label: "Groups", Align: "right"}}, Rows: rows}
}
func (m Model) coverageTone() badge.Tone {
	if m.Result.State != "fresh" && m.Result.State != "partial" {
		return badge.ToneWarning
	}
	return badge.ToneDefault
}
func (m Model) coverageReviewLabel() string {
	if n := m.reviewGroups(); n > 0 {
		if n == 1 {
			return "1 group needs review"
		}
		return count(n) + " groups need review"
	}
	return "Collection needs review"
}
func causeNextStep(code string) string {
	switch code {
	case "cause_latest_tag_missing":
		return "Add an explicit image tag."
	case "cause_manifest_tag_missing":
		return "Verify the image tag."
	case "cause_registry_access_denied":
		return "Check package access."
	case "cause_registry_rate_limited":
		return "Retry collection later."
	case "cause_internal_host_blocked":
		return "Review registry network access."
	case "cause_empty_helm_tag":
		return "Resolve chart inheritance."
	case "cause_invalid_value":
		return "Check tag versioning."
	case "cause_not_recorded":
		return "Collect again with diagnostics."
	default:
		return "Inspect the image details."
	}
}
