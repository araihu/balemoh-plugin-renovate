package view

import (
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
)

func TestOverviewWidgetCountsLabelsNavigationAndUncertainty(t *testing.T) {
	m := Model{BackURL: "/images?repository=example%2Frepo&assessment=updates_found&updateType=", CoverageURL: "/coverage", Summary: advisory.Summary{Assessments: map[string]int{"updates_found": 12}, Candidates: 18, SecurityCandidates: 2, UpdateTypes: map[string]int{"major": 3, "minor": 4, "patch": 5, "digest": 6}, RequestStates: map[string]int{"open": 2, "draft": 1, "merged": 1}}}
	m.Meta.Repository.Name = "example/repo"
	m.Meta.ControllerAvailable = true
	m.Meta.CompletedAt = time.Now()
	m.Result.State = "fresh"
	body := rendered(t, Widget(m))
	for _, want := range []string{"<article", "12 image groups with upgrades", "18 upgrade candidates", "2 Renovate security flags", "Open PRs", "Draft PRs", "Merged", "repository=example%2Frepo", "updateType=major", "Collected", "View image upgrades"} {
		if !strings.Contains(body, want) {
			t.Fatal("missing overview content", want)
		}
	}
	if strings.Contains(body, "Matching declarations") || strings.Contains(body, "Up to date") {
		t.Fatal("widget became a detail page or invented a conclusion")
	}
	m.Meta.ControllerAvailable = false
	m.Result.State = "stale"
	m.Meta.Coverage.Status = "partial"
	body = rendered(t, Widget(m))
	for _, want := range []string{"Stale data", "Partial coverage", "PR tracking unavailable"} {
		if !strings.Contains(body, want) {
			t.Fatal("hidden uncertainty", want)
		}
	}
	if strings.Contains(body, "Open PRs") {
		t.Fatal("stale PR count claimed as current")
	}
	m.Meta.ControllerAvailable = true
	m.Summary.RequestStates = nil
	m.Summary.UnconfirmedRequests = 1
	if body = rendered(t, Widget(m)); !strings.Contains(body, "1 request needs a tracking refresh") || strings.Contains(body, "No tracked requests") {
		t.Fatal("tracking failure presented as empty", body)
	}
	m.Summary = advisory.Summary{}
	body = rendered(t, Widget(m))
	if !strings.Contains(body, "No upgrade candidates reported") || strings.Contains(body, "up to date") || strings.Contains(body, "0 image groups with upgrades") {
		t.Fatal("misleading zero state", body)
	}
}

func TestRepeatedWidgetCardsNameTheirRepositoryDestinations(t *testing.T) {
	combined := ""
	for _, repo := range []string{"example/alpha", "example/beta"} {
		m := Model{BackURL: "/images?repository=" + repo, CoverageURL: "/coverage?repository=" + repo, Summary: advisory.Summary{Assessments: map[string]int{"updates_found": 1}, Candidates: 1, UpdateTypes: map[string]int{"major": 1}}}
		m.Meta.Repository.Name = repo
		m.Meta.Coverage.Status = "partial"
		combined += rendered(t, Widget(m))
	}
	for _, repo := range []string{"example/alpha", "example/beta"} {
		for _, purpose := range []string{"Major upgrade candidates for ", "Minor upgrade candidates for ", "Patch upgrade candidates for ", "Partial coverage for ", "View image upgrades for "} {
			label := `aria-label="` + purpose + repo + `"`
			if strings.Count(combined, label) != 1 {
				t.Fatal("ambiguous repeated-card destination", label)
			}
		}
	}
}
