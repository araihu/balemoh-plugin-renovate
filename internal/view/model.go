package view

import (
	"fmt"
	"github.com/a-h/templ"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/araihu/goshtoso/components/combobox"
	"github.com/araihu/goshtoso/components/table"
	"slices"
	"strings"
)

type Model struct {
	Request                                                                     ui.Request
	Filter                                                                      advisory.Filter
	Result                                                                      ui.QueryResult
	Meta                                                                        advisory.Meta
	Summary                                                                     advisory.Summary
	List                                                                        advisory.List
	Detail                                                                      advisory.Detail
	CurrentURL, BackURL, RestartURL, ResetURL, CoverageURL, NextURL, ResultsURL string
	Links                                                                       map[string]string
	Services                                                                    map[string]ui.ServiceIdentity
	RequestLinks                                                                map[string]string
	ActionURL, ActionError, StatusURL                                           string
	Selected                                                                    snapshot.Candidate
}

func text(p *string) string {
	if p == nil {
		return "Not recorded"
	}
	return *p
}

func tagLabel(value, digest *string) string {
	if value == nil && digest != nil {
		return "Untagged image"
	}
	return text(value)
}
func (m Model) Table() table.Config {
	rows := []table.Row{}
	for _, d := range m.List.Items {
		rows = append(rows, table.Row{ID: d.ID, Cells: map[string]table.Cell{"image": {Component: ImageCell(m, d, "table")}, "git": {Component: Reference(d)}, "proposed": {Component: Proposals(d)}, "assessment": {Component: Assessment(d)}, "pr": {Component: ListPRStatus(m.forItem(d))}}, Actions: ListActions(m.forItem(d))})
	}
	return table.Config{ID: m.Request.DOMID("table"), Caption: "Image and Git file groups.", RootClass: "image-upgrades-table", Columns: []table.Column{{Key: "image", Label: "Image / Git file"}, {Key: "git", Label: "Git reference"}, {Key: "proposed", Label: "Renovate candidates"}, {Key: "assessment", Label: "Assessment"}, {Key: "pr", Label: "PR status"}}, Rows: rows}
}
func (m Model) filterCombobox(name, title, current string, values []string, describe func(string) string) combobox.Config {
	options := make([]combobox.Option, 0, len(values))
	for _, value := range values {
		options = append(options, combobox.Option{Value: value, Label: describe(value)})
	}
	selected := []string{}
	if current != "" {
		selected = strings.Split(current, ",")
	}
	return combobox.Config{ID: m.Request.DOMID(name), Name: name, Label: title, Placeholder: "All", Mode: combobox.ModeMultiple, Source: combobox.Source{Static: options}, Selected: selected, EnableClearAll: true, DisablePersistence: true, TriggerAttrs: templ.Attributes{"aria-controls": m.Request.DOMID(name) + "-options", "aria-haspopup": "listbox"}}
}
func count(n int) string { return fmt.Sprint(n) }
func candidateLabel(u snapshot.Candidate) string {
	if u.NewValue == nil && u.NewDigest != nil && u.UpdateType == "digest" {
		return "Digest update"
	}
	return text(u.NewValue) + " · " + label(u.UpdateType)
}
func recovery(code string) string {
	switch code {
	case "not_found":
		return "This image group is no longer in the current snapshot."
	case "conflict":
		return "The observation changed. Reload the list to continue."
	case "permission_denied":
		return "This connection has not granted the upgrade query."
	case "invalid_argument":
		return "The supplied filter or image identity is invalid."
	default:
		return "No validated snapshot is available. The collector may still be starting."
	}
}

func shortDigest(p *string) string {
	if p == nil {
		return ""
	}
	if len(*p) > 16 {
		return (*p)[:15] + "…"
	}
	return *p
}

func label(value string) string {
	if text, ok := map[string]string{"updates_found": "Candidates observed", "unknown": "No conclusive result", "excluded": "Outside upgrade scope", "no_candidate": "No candidate in policy", "major": "Major", "minor": "Minor", "patch": "Patch", "digest": "Digest", "pinDigest": "Pin digest", "pin": "Pin version", "pr_open": "PR observed", "pending_checks": "Checks pending", "approval_required": "Approval required", "pr_limit": "PR limit", "fresh": "Current", "partial": "Partial", "complete": "Complete", "stale": "Stale", "eligible": "Eligible under reported policy", "release_age": "Waiting for release age", "scheduled": "Waiting for schedule", "rate_limited": "Rate limit reached", "ignored": "Ignored by policy"}[value]; ok {
		return text
	}
	return value
}
func workflowLabels(d advisory.Item) []string {
	labels := []string{}
	seen := map[string]bool{}
	for _, u := range d.Candidates {
		value := "Policy/workflow not established"
		if u.Policy != "unknown" {
			value = label(u.Policy)
		}
		if u.Workflow == "pr_open" {
			value += " · PR observed"
		}
		if !seen[value] {
			seen[value] = true
			labels = append(labels, value)
		}
	}
	return labels
}

func hasSecurity(d advisory.Item) bool {
	for _, u := range d.Candidates {
		if u.IsVulnerabilityAlert {
			return true
		}
	}
	return false
}

func (m Model) ResultCount() string {
	noun := "image groups"
	if m.List.Total == 1 {
		noun = "image group"
	}
	if len(m.List.Items) < m.List.Total {
		return fmt.Sprintf("Showing %d of %d matching %s.", len(m.List.Items), m.List.Total, noun)
	}
	return fmt.Sprintf("%d matching %s.", m.List.Total, noun)
}
func (m Model) Freshness() string {
	if m.Result.State == "fresh" || m.Result.State == "partial" {
		return "Current observation"
	}
	if m.Result.State == "stale" {
		return "Stale observation"
	}
	return "Collection unavailable"
}
func policyLabel(value string) string {
	if value == "unknown" {
		return "Not established by the report"
	}
	return label(value)
}
func workflowLabel(value string) string {
	switch value {
	case "unknown":
		return "PR status not established"
	case "pr_open":
		return "Pull request reported"
	case "branch_only":
		return "Branch reported, PR not established"
	case "none_observed":
		return "No workflow reported"
	}
	return "Workflow not interpreted"
}
func workflowOptionLabel(value string) string {
	if value == "pr_open" {
		return "PR reported"
	}
	return "PR unknown"
}
func reasonLabel(value string) string {
	if strings.HasPrefix(value, "cause_") {
		return causeDescription(value)
	}
	values := map[string]string{
		"source_problems":              "Renovate reported collection problems; results may be incomplete.",
		"unsupported_manager":          "Some image declarations use a manager outside this collector's scope.",
		"missing_identity":             "Some declarations could not be identified reliably.",
		"unassessed_dependencies":      "Some image groups could not be assessed.",
		"ambiguous_declarations":       "Declarations for this image group disagree, so no candidate is attributed to it.",
		"disabled":                     "Renovate policy excludes this declaration.",
		"chart_inherited":              "No image tag is set in these Helm values. The chart or parent values must supply it; this declaration is outside the explicit-image scan.",
		"lookup_unconfirmed":           "Renovate skipped the lookup or reported a warning. Review collection logs for the cause.",
		"negative_assessment_unproven": "Renovate returned no upgrade candidates. This report does not confirm whether the image is up to date.",
	}
	if text, ok := values[value]; ok {
		return text
	}
	return "An additional collection limitation was reported."
}
func managerLabel(value string) string {
	if text, ok := map[string]string{"kubernetes": "Kubernetes", "helm-values": "Helm values", "docker-compose": "Docker Compose", "dockerfile": "Dockerfile", "kustomize": "Kustomize", "regex": "Custom extraction rule"}[value]; ok {
		return text
	}
	return "Other image manager"
}
func managerSymbol(value string) string {
	switch value {
	case "dockerfile", "docker-compose":
		return "docker-docker"
	case "kubernetes", "helm-values", "kustomize":
		return "kubernetes-kubernetes"
	default:
		return ""
	}
}
func branchResultLabel(value string) string {
	if text, ok := map[string]string{"needs-approval": "Waiting for approval", "needs-pr-approval": "Waiting for PR creation approval", "not-scheduled": "Waiting for schedule", "pending": "Waiting for status checks", "branch-limit-reached": "Branch limit reached", "pr-limit-reached": "PR limit reached", "commit-per-run-limit-reached": "Commit limit per run reached", "commit-hourly-limit-reached": "Hourly commit limit reached", "pr-created": "Pull request created", "pr-edited": "Pull request manually edited; Renovate changes blocked"}[value]; ok {
		return text
	}
	return "Other result reported by Renovate"
}

func assessmentLabel(d advisory.Item) string {
	if d.Assessment == "excluded" {
		if slices.Contains(d.Reasons, "chart_inherited") {
			return "Inherited from chart"
		}
		if slices.Contains(d.Reasons, "disabled") {
			return "Disabled by policy"
		}
	}
	switch advisory.AssessmentKind(d.Dependency) {
	case "no_candidates_reported":
		return "No candidates reported"
	case "lookup_unconfirmed":
		return "Lookup unconfirmed"
	case "conflicting_declarations":
		return "Conflicting declarations"
	case "unknown":
		return "Upgrade status unknown"
	default:
		return label(d.Assessment)
	}
}
func (m Model) reviewGroups() int {
	return max(0, m.Meta.Coverage.Unknown-m.Meta.AssessmentBreakdown.NoCandidatesReported)
}
func (m Model) hasCollectionIssues() bool {
	if m.reviewGroups() > 0 || m.Meta.AssessmentBreakdown.UnidentifiedRecords > 0 {
		return true
	}
	return slices.ContainsFunc(m.Meta.Coverage.Reasons, func(r string) bool { return r != "unassessed_dependencies" })
}
func (m Model) collectionIssueDescription() string {
	parts := []string{}
	if n := m.reviewGroups(); n > 0 {
		noun := "image/file groups need"
		if n == 1 {
			noun = "image/file group needs"
		}
		parts = append(parts, fmt.Sprintf("%d %s review.", n, noun))
	}
	if n := m.Meta.AssessmentBreakdown.UnidentifiedRecords; n > 0 {
		noun := "source records could"
		if n == 1 {
			noun = "source record could"
		}
		parts = append(parts, fmt.Sprintf("%d %s not be identified.", n, noun))
	}
	if len(parts) == 0 {
		parts = append(parts, "Renovate reported collection limitations.")
	}
	return strings.Join(parts, " ") + " Counts cover the whole repository."
}
func noCandidatesDescription(n int) string {
	if n == 1 {
		return "Across this repository, 1 image/file group has no candidates reported."
	}
	return fmt.Sprintf("Across this repository, %d image/file groups have no candidates reported.", n)
}
