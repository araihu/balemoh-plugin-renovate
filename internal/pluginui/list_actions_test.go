package pluginui

import (
	"context"
	"encoding/json"
	"html"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

type listActionHost struct {
	actions, queries int
	items            []advisory.Item
}

func (h *listActionHost) Placement(context.Context) (ui.Placement, error) { return ui.Placement{}, nil }
func (h *listActionHost) Query(context.Context, string, json.RawMessage) (ui.QueryResult, error) {
	h.queries++
	meta := advisory.Meta{ControllerAvailable: true, Controller: requests.Catalog{Enabled: true}}
	b, _ := json.Marshal(advisory.List{Meta: meta, Items: h.items, Total: len(h.items)})
	return ui.QueryResult{Data: b, State: "fresh"}, nil
}
func (h *listActionHost) Action(context.Context, string, json.RawMessage) (backend.ActionResult, error) {
	h.actions++
	return backend.ActionResult{}, nil
}

func TestListLinksBindEachCandidateAndPreserveFiltersWithoutMutation(t *testing.T) {
	raw, e := os.ReadFile("../../plugin.json")
	if e != nil {
		t.Fatal(e)
	}
	p, _, e := manifest.Decode(strings.NewReader(string(raw)))
	if e != nil {
		t.Fatal(e)
	}
	r := ui.Request{Method: "GET", Mode: "page", ContributionType: "page", ContributionID: "current", ConnectionID: "00000000-0000-4000-8000-000000000001", ActionGrants: []string{"upgrades.request"}, Query: url.Values{"search": {"nginx"}, "repository": {"example/repo"}, "updateType": {"minor", "major"}}}
	h := &listActionHost{}
	for _, id := range []string{"a", "b"} {
		h.items = append(h.items, advisory.Item{Dependency: snapshot.Dependency{ID: strings.Repeat(id, 64), PackageName: "nginx", PackageFile: id + ".yaml", Manager: "kubernetes", CurrentValue: new("1.0.0"), Assessment: "updates_found", Candidates: []snapshot.Candidate{{ID: strings.Repeat(id, 64), NewValue: new("1.1.0"), UpdateType: "minor"}}}})
	}
	response, e := (Handler{Manifest: p}).Handle(t.Context(), r, h)
	if e != nil || response.Status != 200 || h.actions != 0 || h.queries != 1 {
		t.Fatal("list must only query", e, h)
	}
	if response.Description == "" {
		t.Fatal("route metadata missing")
	}
	counts := map[string]int{}
	for _, href := range regexp.MustCompile(`href="([^"]+)"`).FindAllSubmatch(response.HTML, -1) {
		u, e := url.Parse(html.UnescapeString(string(href[1])))
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(u.Path, "/request/") {
			continue
		}
		for _, id := range []string{"a", "b"} {
			want := "/current/" + strings.Repeat(id, 64) + "/request/" + strings.Repeat(id, 64)
			if strings.HasSuffix(u.Path, want) {
				counts[id]++
				q := u.Query()
				if q.Get("search") != "nginx" || q.Get("repository") != "example/repo" || strings.Join(q["updateType"], ",") != "minor,major" {
					t.Fatal("lost filters", u)
				}
			}
		}
	}
	if counts["a"] != 2 || counts["b"] != 2 {
		t.Fatal("desktop/mobile actions lost their candidate identity", counts)
	}
	if !strings.Contains(string(response.HTML), "PR status") {
		t.Fatal("missing status column")
	}
}
