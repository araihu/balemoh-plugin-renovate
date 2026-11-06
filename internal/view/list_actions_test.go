package view

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func listActionModel() Model {
	c := snapshot.Candidate{ID: "candidate-a", NewValue: new("1.1.0"), UpdateType: "minor"}
	d := advisory.Item{Dependency: snapshot.Dependency{ID: "image-a", Manager: "kubernetes", PackageFile: "a.yaml", PackageName: "nginx", CurrentValue: new("1.0.0"), Assessment: "updates_found", Candidates: []snapshot.Candidate{c}}}
	m := Model{Request: ui.Request{ActionGrants: []string{"upgrades.request"}}, Detail: advisory.Detail{Dependency: d}, RequestLinks: map[string]string{c.ID: "/a/request/candidate-a?search=nginx"}, Links: map[string]string{d.ID: "/a?search=nginx"}}
	m.Result.State = "fresh"
	m.Meta.ControllerAvailable = true
	m.Meta.Controller.Enabled = true
	m.Meta.SnapshotDigest = "fresh"
	return m
}
func rendered(t *testing.T, c templ.Component) string {
	t.Helper()
	var b bytes.Buffer
	if err := c.Render(t.Context(), &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestListActionsScopeEachRowAndKeepExistingReviewRoute(t *testing.T) {
	m := listActionModel()
	a := m.Detail.Dependency
	b := a
	b.ID = "image-b"
	b.PackageFile = "b.yaml"
	b.Candidates = []snapshot.Candidate{{ID: "candidate-b", NewValue: new("2.0.0"), UpdateType: "major"}}
	m.Links[b.ID] = "/b?search=nginx"
	m.RequestLinks["candidate-b"] = "/b/request/candidate-b?search=nginx"
	m.Meta.Controller.Requests = []requests.Record{{ID: "request-b", Dependency: b.Dependency, Candidate: b.Candidates[0], State: "open", PR: &requests.PullRequest{Number: 15, URL: "https://github.com/test/repo/pull/15", Checks: "passing"}}}
	for _, tc := range []struct {
		d          advisory.Item
		want, deny string
	}{{a, "/a/request/candidate-a?search=nginx", "pull/15"}, {b, "https://github.com/test/repo/pull/15", "/b/request/"}} {
		body := rendered(t, ListActions(m.forItem(tc.d)))
		if !strings.Contains(body, tc.want) || strings.Contains(body, tc.deny) || strings.Contains(body, "<form") {
			t.Fatal("wrong row/action scope", body)
		}
	}
	if body := rendered(t, ListPRStatus(m.forItem(b))); !strings.Contains(body, "Open PR #15") || !strings.Contains(body, "Checks passed") {
		t.Fatal(body)
	}
	if body := rendered(t, ListPRStatus(m.forItem(a))); strings.Contains(body, "Open PR") || !strings.Contains(body, "Not requested") {
		t.Fatal(body)
	}
}

func TestListReviewLinksRespectExistingAdmissionAndTerminalStates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(*Model)
		allow bool
	}{
		{"ready", func(*Model) {}, true},
		{"stale", func(m *Model) { m.Result.State = "stale" }, false},
		{"no grant", func(m *Model) { m.Request.ActionGrants = nil }, false},
		{"disabled", func(m *Model) { m.Meta.Controller.Enabled = false }, false},
		{"unavailable", func(m *Model) { m.Meta.ControllerAvailable = false }, false},
		{"pin unsupported", func(m *Model) { m.Detail.Dependency.Candidates[0].UpdateType = "pinDigest" }, false},
		{"closed", func(m *Model) {
			m.Meta.Controller.Requests = []requests.Record{{Dependency: m.Detail.Dependency.Dependency, Candidate: m.Detail.Dependency.Candidates[0], State: "closed"}}
		}, false},
		{"merged", func(m *Model) {
			m.Meta.Controller.Requests = []requests.Record{{Dependency: m.Detail.Dependency.Dependency, Candidate: m.Detail.Dependency.Candidates[0], State: "merged"}}
		}, false},
		{"blocked same snapshot", func(m *Model) {
			m.Meta.Controller.Requests = []requests.Record{{Dependency: m.Detail.Dependency.Dependency, Candidate: m.Detail.Dependency.Candidates[0], State: "blocked", SnapshotDigest: "fresh"}}
		}, false},
		{"blocked new snapshot", func(m *Model) {
			m.Meta.Controller.Requests = []requests.Record{{Dependency: m.Detail.Dependency.Dependency, Candidate: m.Detail.Dependency.Candidates[0], State: "blocked", SnapshotDigest: "old"}}
		}, true},
		{"other candidate active", func(m *Model) {
			m.Meta.Controller.Requests = []requests.Record{{Dependency: m.Detail.Dependency.Dependency, Candidate: snapshot.Candidate{ID: "another"}, State: "queued"}}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := listActionModel()
			tc.edit(&m)
			body := rendered(t, ListActions(m.forItem(m.Detail.Dependency)))
			if strings.Contains(body, "/a/request/") != tc.allow {
				t.Fatal(body)
			}
		})
	}
}

func TestListTrackingFailureNeverClaimsPassingChecks(t *testing.T) {
	for _, available := range []bool{true, false} {
		for _, trackingError := range []bool{true, false} {
			m := listActionModel()
			m.Meta.ControllerAvailable = available
			m.Meta.Controller.Requests = []requests.Record{{Dependency: m.Detail.Dependency.Dependency, State: "open", TrackingError: trackingError, PR: &requests.PullRequest{Number: 15, URL: "https://github.com/test/repo/pull/15", Checks: "passing"}}}
			body := rendered(t, ListPRStatus(m))
			unavailable := !available || trackingError
			if strings.Contains(body, "Checks passed") == unavailable || strings.Contains(body, "Tracking unavailable") != unavailable {
				t.Fatal(body)
			}
			if !strings.Contains(rendered(t, ListActions(m)), "pull/15") {
				t.Fatal("retained PR must remain reachable")
			}
		}
	}
}

func TestListShowsReportedPRWithoutInventingLiveStatusAndDeduplicates(t *testing.T) {
	m := listActionModel()
	pr := &snapshot.PR{Number: 7, URL: "https://github.com/test/repo/pull/7"}
	m.Detail.Dependency.Candidates[0].PR = pr
	m.Detail.Dependency.Candidates = append(m.Detail.Dependency.Candidates, m.Detail.Dependency.Candidates[0])
	body := rendered(t, ListPRStatus(m))
	if !strings.Contains(body, "Reported PR #7") || !strings.Contains(body, "Status not verified") || strings.Contains(body, "Checks passed") {
		t.Fatal(body)
	}
	actions := rendered(t, ListActions(m))
	if strings.Count(actions, `href="`+pr.URL+`"`) != 1 || strings.Contains(actions, "/a/request/") {
		t.Fatal(actions)
	}
}

func TestListKeepsActiveRequestVisibleWhenCandidateIsFilteredOut(t *testing.T) {
	m := listActionModel()
	d := m.Detail.Dependency.Dependency
	now := time.Now()
	m.Meta.Controller.Requests = []requests.Record{{ID: "active", Dependency: d, Candidate: snapshot.Candidate{ID: "filtered-out"}, State: "running", CreatedAt: now.Add(-time.Minute)}, {ID: "terminal", Dependency: d, State: "closed", CreatedAt: now}}
	if r := m.listRequest(); r == nil || r.ID != "active" {
		t.Fatal("active group request hidden by a newer terminal record")
	}
	body := rendered(t, ListActions(m))
	if strings.Contains(body, "/a/request/") || !strings.Contains(body, "View request") {
		t.Fatal(body)
	}
}

func TestMultipleCandidateReviewActionsNameAndBindEachTarget(t *testing.T) {
	m := listActionModel()
	m.Detail.Dependency.Candidates = append(m.Detail.Dependency.Candidates, snapshot.Candidate{ID: "candidate-major", NewValue: new("2.0.0"), UpdateType: "major"})
	m.RequestLinks["candidate-major"] = "/a/request/candidate-major"
	body := rendered(t, ListActions(m.forItem(m.Detail.Dependency)))
	for _, want := range []string{"Review PR for 1.1.0 · Minor", "Review PR for 2.0.0 · Major", "/a/request/candidate-a", "/a/request/candidate-major"} {
		if !strings.Contains(body, want) {
			t.Fatal("ambiguous or missing candidate action", want)
		}
	}
}

func TestPRStatePresentationAcrossListAndDetails(t *testing.T) {
	for _, tc := range []struct {
		state, guidance string
		terminal        bool
	}{
		{"open", "Review and merge the PR manually in GitHub.", false},
		{"draft", "Review the draft in GitHub before marking it ready for a manual merge.", false},
		{"merged", "", true},
		{"closed", "View the closed PR in GitHub for the reason it was closed.", true},
	} {
		t.Run(tc.state, func(t *testing.T) {
			m := listActionModel()
			r := requests.Record{Dependency: m.Detail.Dependency.Dependency, Candidate: m.Detail.Dependency.Candidates[0], State: tc.state, PR: &requests.PullRequest{Number: 15, URL: "https://github.com/test/repo/pull/15", Checks: "passing"}}
			m.Meta.Controller.Requests = []requests.Record{r}
			detail := rendered(t, RequestStatus(m, r))
			actions := rendered(t, ListActions(m))
			status := rendered(t, ListPRStatus(m))
			for _, body := range []string{detail, actions} {
				if !strings.Contains(body, "View PR #15") || strings.Contains(body, ">Open PR") {
					t.Fatal("ambiguous PR navigation", body)
				}
			}
			if strings.Contains(actions, "/a/request/") {
				t.Fatal("existing PR offered a new request", actions)
			}
			if tc.guidance != "" && !strings.Contains(detail, tc.guidance) {
				t.Fatal("missing state guidance", detail)
			}
			if tc.state != "open" && strings.Contains(detail, "Review and merge the PR manually") {
				t.Fatal("pending merge guidance for non-open PR", detail)
			}
			for _, body := range []string{detail, status} {
				if strings.Contains(body, "Last reported checks passed") != tc.terminal {
					t.Fatal("terminal checks must be historical", body)
				}
				if tc.terminal && strings.Contains(body, "Merge eligibility") {
					t.Fatal("terminal PR presented as awaiting merge", body)
				}
			}
		})
	}
}

func TestDetailUnavailableTrackingSuppressesRetainedPassingChecks(t *testing.T) {
	for _, available := range []bool{true, false} {
		for _, trackingError := range []bool{true, false} {
			m := listActionModel()
			m.Meta.ControllerAvailable = available
			r := requests.Record{State: "open", TrackingError: trackingError, PR: &requests.PullRequest{Number: 15, URL: "https://github.com/test/repo/pull/15", Checks: "passing"}}
			body := rendered(t, RequestStatus(m, r))
			unavailable := !available || trackingError
			if strings.Contains(body, "Reported checks passed") == unavailable || strings.Contains(body, "Request tracking unavailable") != unavailable {
				t.Fatal(body)
			}
		}
	}
}
