package pluginui

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

type actionHost struct {
	host
	actions int
	failure string
	detail  advisory.Detail
}

func (h *actionHost) Query(context.Context, string, json.RawMessage) (ui.QueryResult, error) {
	b, _ := json.Marshal(h.detail)
	return ui.QueryResult{Data: b, State: "fresh"}, nil
}
func (h *actionHost) Action(_ context.Context, id string, b json.RawMessage) (backend.ActionResult, error) {
	h.actions++
	var s requests.Selection
	if id != "upgrades.request" || json.Unmarshal(b, &s) != nil || s.DependencyID != h.detail.Dependency.ID || s.CandidateID != h.detail.Dependency.Candidates[0].ID || s.SnapshotDigest != h.detail.Meta.SnapshotDigest {
		return backend.ActionResult{}, backend.Fail("invalid_argument")
	}
	if h.failure != "" {
		return backend.ActionResult{}, backend.Fail(h.failure)
	}
	return backend.ActionResult{Data: json.RawMessage(`{"requestId":"` + strings.Repeat("d", 64) + `"}`)}, nil
}
func TestRequestPageAdmissionRecoveryAndPRG(t *testing.T) {
	data, e := os.ReadFile("../../plugin.json")
	if e != nil {
		t.Fatal(e)
	}
	p, _, e := manifest.Decode(strings.NewReader(string(data)))
	if e != nil {
		t.Fatal(e)
	}
	d := snapshot.Dependency{ID: strings.Repeat("a", 64), Manager: "kubernetes", PackageFile: "k8s/test.yaml", PackageName: "nginx", Assessment: "updates_found", CurrentValue: new("1.25.3"), Candidates: []snapshot.Candidate{{ID: strings.Repeat("b", 64), NewValue: new("1.26.3"), UpdateType: "minor"}}}
	for _, tc := range []struct {
		name, failure string
		want, calls   int
	}{
		{"get", "", 200, 0}, {"success", "", 303, 1}, {"repeat", "", 303, 2}, {"stale", "", 409, 0}, {"extra", "", 400, 0}, {"ungranted", "", 403, 0}, {"denied", "permission_denied", 403, 1}, {"changed", "conflict", 409, 1}, {"transport", "unavailable", 503, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &actionHost{failure: tc.failure, detail: advisory.Detail{Meta: advisory.Meta{SnapshotDigest: strings.Repeat("c", 64), ControllerAvailable: true, Controller: requests.Catalog{Enabled: true, Requests: []requests.Record{}}}, Dependency: advisory.Item{Dependency: d}}}
			r := ui.Request{Method: "POST", Mode: "page", ContributionType: "page", ContributionID: "request", ConnectionID: "00000000-0000-4000-8000-000000000001", ActionGrants: []string{"upgrades.request"}, PathParams: map[string]string{"dependency-id": d.ID, "candidate-id": d.Candidates[0].ID}, Query: url.Values{"search": {"nginx"}, "assessment": {"updates_found"}}, Form: url.Values{"approved": {"true"}, "snapshotDigest": {h.detail.Meta.SnapshotDigest}}}
			switch tc.name {
			case "get":
				r.Method = "GET"
			case "stale":
				r.Form.Set("snapshotDigest", strings.Repeat("e", 64))
			case "extra":
				r.Form.Set("repository", "foreign/repo")
			case "ungranted":
				r.ActionGrants = nil
			}
			response, e := (Handler{Manifest: p}).Handle(t.Context(), r, h)
			if e != nil || response.Status != tc.want {
				t.Fatalf("status=%d error=%v", response.Status, e)
			}
			if tc.name == "repeat" {
				response, e = (Handler{Manifest: p}).Handle(t.Context(), r, h)
				if e != nil || response.Status != 303 {
					t.Fatal("repeat failed")
				}
			}
			if h.actions != tc.calls {
				t.Fatalf("action calls=%d want=%d", h.actions, tc.calls)
			}
			if tc.want == 303 {
				if response.Redirect == nil || response.Redirect.Query.Get("search") != "nginx" || response.Redirect.Query.Get("request") != strings.Repeat("d", 64) {
					t.Fatal("receipt/filter identity lost")
				}
			}
			if tc.want >= 400 && (!strings.Contains(string(response.HTML), `role="alert"`) || !strings.Contains(string(response.HTML), "search=nginx")) {
				t.Fatal("error or recovery context missing")
			}
		})
	}
}
