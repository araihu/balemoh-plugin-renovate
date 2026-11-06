package view

import (
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"slices"
	"strings"
)

func (m Model) RequestFor(candidate string) *requests.Record {
	var latest *requests.Record
	for _, r := range m.Meta.Controller.Requests {
		if r.Dependency.ID == m.Detail.Dependency.ID && r.Candidate.ID == candidate && (latest == nil || r.CreatedAt.After(latest.CreatedAt)) {
			value := r
			latest = &value
		}
	}
	return latest
}
func (m Model) OtherActiveRequest(candidate string) *requests.Record {
	for _, r := range m.Meta.Controller.Requests {
		if r.Dependency.ID == m.Detail.Dependency.ID && r.Candidate.ID != candidate {
			if activeRequest(r.State) {
				value := r
				return &value
			}
		}
	}
	return nil
}
func (m Model) CanRequest(c snapshot.Candidate) bool {
	return m.OtherActiveRequest(c.ID) == nil && slices.Contains(m.Request.ActionGrants, "upgrades.request") && m.Meta.ControllerAvailable && m.Meta.Controller.Enabled && (m.Result.State == "fresh" || m.Result.State == "partial") && requests.Supported(m.Detail.Dependency.Dependency, c) && c.PR == nil
}

func (m Model) CanReviewAgain(c snapshot.Candidate) bool {
	r := m.RequestFor(c.ID)
	return m.CanRequest(c) && (r == nil || r.State == "blocked" && r.SnapshotDigest != m.Meta.SnapshotDigest)
}

func requestState(value string) string {
	switch value {
	case "queued":
		return "Request queued"
	case "running":
		return "Renovate is processing the request"
	case "awaiting_pr":
		return "Waiting to observe the pull request"
	case "open":
		return "Pull request open"
	case "draft":
		return "Pull request is a draft"
	case "merged":
		return "Pull request merged"
	case "closed":
		return "Pull request closed without merging"
	case "blocked":
		return "Request blocked. Refresh the collection before reviewing another request."
	case "needs_attention":
		return "Execution needs review. A write may have completed; it will not be retried automatically."
	}
	return "Request status unknown"
}
func checksLabel(value string) string {
	switch value {
	case "passing":
		return "Reported checks passed. Merge eligibility is not established."
	case "pending":
		return "Checks pending"
	case "failing":
		return "Checks failed"
	}
	return "Checks unknown or incomplete"
}

// List rows reuse the detail page's eligibility rules with their own image identity.
func (m Model) forItem(d advisory.Item) Model {
	m.Detail.Dependency = d
	m.StatusURL = m.Links[d.ID]
	return m
}

func (m Model) listRequest() *requests.Record {
	var latest *requests.Record
	for _, r := range m.Meta.Controller.Requests {
		if r.Dependency.ID != m.Detail.Dependency.ID {
			continue
		}
		if latest == nil || activeRequest(r.State) && !activeRequest(latest.State) || activeRequest(r.State) == activeRequest(latest.State) && r.CreatedAt.After(latest.CreatedAt) {
			value := r
			latest = &value
		}
	}
	return latest
}

func activeRequest(state string) bool {
	return slices.Contains([]string{"queued", "running", "awaiting_pr", "open", "draft", "needs_attention"}, state)
}

func listRequestLabel(r requests.Record) string {
	label := map[string]string{"queued": "Queued", "running": "Creating PR", "awaiting_pr": "Awaiting PR", "open": "Open", "draft": "Draft", "merged": "Merged", "closed": "Closed", "blocked": "Blocked", "needs_attention": "Needs review"}[r.State]
	if label == "" {
		label = "Status unknown"
	}
	if r.PR != nil {
		label += " PR #" + count(r.PR.Number)
	}
	return label
}

func listChecksLabel(value string) string {
	switch value {
	case "passing":
		return "Checks passed"
	case "pending":
		return "Checks pending"
	case "failing":
		return "Checks failed"
	default:
		return "Checks unknown"
	}
}

func (m Model) reportedPRs() []snapshot.PR {
	seen := map[string]bool{}
	if r := m.listRequest(); r != nil && r.PR != nil {
		seen[r.PR.URL] = true
	}
	prs := []snapshot.PR{}
	for _, c := range m.Detail.Dependency.Candidates {
		if c.PR != nil && !seen[c.PR.URL] {
			seen[c.PR.URL] = true
			prs = append(prs, *c.PR)
		}
	}
	return prs
}

func reviewPRLabel(d advisory.Item, c snapshot.Candidate) string {
	if len(d.Candidates) == 1 {
		return "Review PR request"
	}
	label := "Review PR for " + candidateLabel(c)
	if c.NewDigest != nil && (c.NewValue == nil || c.UpdateType == "digest") {
		label += " " + shortDigest(c.NewDigest)
	}
	return label
}

func (m Model) requestChecks(r requests.Record, compact bool) string {
	if !m.Meta.ControllerAvailable || r.TrackingError {
		return "Checks unknown or incomplete"
	}
	if r.PR == nil {
		return ""
	}
	if r.State == "merged" || r.State == "closed" {
		return "Last reported " + strings.ToLower(listChecksLabel(r.PR.Checks))
	}
	if compact {
		return listChecksLabel(r.PR.Checks)
	}
	return checksLabel(r.PR.Checks)
}

func requestGuidance(state string) string {
	switch state {
	case "open":
		return "Review and merge the PR manually in GitHub."
	case "draft":
		return "Review the draft in GitHub before marking it ready for a manual merge."
	case "closed":
		return "View the closed PR in GitHub for the reason it was closed."
	default:
		return ""
	}
}
