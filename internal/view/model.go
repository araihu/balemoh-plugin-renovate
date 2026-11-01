package view

import (
	"fmt"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	selectfield "github.com/araihu/goshtoso/components/select"
	"github.com/araihu/goshtoso/components/table"
)

type Model struct {
	Request                                                         ui.Request
	Filter                                                          advisory.Filter
	Result                                                          ui.QueryResult
	Meta                                                            advisory.Meta
	List                                                            advisory.List
	Detail                                                          advisory.Detail
	CurrentURL, BackURL, RestartURL, ResetURL, CoverageURL, NextURL string
	Links                                                           map[string]string
	Services                                                        map[string]ui.ServiceIdentity
}

func text(p *string) string {
	if p == nil {
		return "Not recorded"
	}
	return *p
}
func (m Model) Table() table.Config {
	rows := []table.Row{}
	for _, d := range m.List.Items {
		rows = append(rows, table.Row{ID: d.ID, Cells: map[string]table.Cell{"image": {Component: ImageCell(m, d, "table")}, "git": {Component: Reference(d)}, "proposed": {Component: Proposals(d)}, "assessment": {Component: Assessment(d)}}, Actions: DetailLink(m.Links[d.ID], d)})
	}
	return table.Config{ID: m.Request.DOMID("table"), Caption: "Image and Git file groups.", RootClass: "image-upgrades-table", Columns: []table.Column{{Key: "image", Label: "Image / Git file"}, {Key: "git", Label: "Git reference"}, {Key: "proposed", Label: "Renovate candidates"}, {Key: "assessment", Label: "Assessment"}}, Rows: rows}
}
func options(values []string, current string, describe func(string) string) []selectfield.Option {
	out := []selectfield.Option{{Value: "", Label: "All", Selected: current == ""}}
	for _, v := range values {
		out = append(out, selectfield.Option{Value: v, Label: describe(v), Selected: v == current})
	}
	return out
}
func count(n int) string { return fmt.Sprint(n) }
func candidateLabel(u snapshot.Candidate) string {
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
	if text, ok := map[string]string{"updates_found": "Candidates observed", "unknown": "Unassessed", "excluded": "Excluded by policy", "no_candidate": "No candidate in policy", "major": "Major", "minor": "Minor", "patch": "Patch", "digest": "Digest", "pinDigest": "Pin digest", "pin": "Pin version", "pr_open": "PR observed", "pending_checks": "Checks pending", "approval_required": "Approval required", "pr_limit": "PR limit", "fresh": "Current", "partial": "Partial", "complete": "Complete", "stale": "Stale", "eligible": "Eligible under reported policy", "release_age": "Waiting for release age", "scheduled": "Waiting for schedule", "rate_limited": "Rate limit reached", "ignored": "Ignored by policy"}[value]; ok {
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
	values := map[string]string{
		"source_problems":              "Renovate reported collection problems; results may be incomplete.",
		"unsupported_manager":          "Some image declarations use a manager outside this collector's scope.",
		"missing_identity":             "Some declarations could not be identified reliably.",
		"unassessed_dependencies":      "Some image groups could not be assessed.",
		"ambiguous_declarations":       "Declarations for this image group disagree, so no candidate is attributed to it.",
		"disabled":                     "Renovate policy excludes this declaration.",
		"lookup_unconfirmed":           "An image lookup could not be confirmed.",
		"negative_assessment_unproven": "The report provides no proven candidate or up-to-date conclusion for this group.",
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
