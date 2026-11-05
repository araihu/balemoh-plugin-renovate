package view

import (
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"slices"
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
			switch r.State {
			case "queued", "running", "awaiting_pr", "open", "draft", "needs_attention":
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
