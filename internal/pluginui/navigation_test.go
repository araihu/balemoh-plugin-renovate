package pluginui

import (
	"html"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func testHandler(t *testing.T) Handler {
	t.Helper()
	data, err := os.ReadFile("../../plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := manifest.Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	return Handler{Manifest: m}
}
func links(t *testing.T, data []byte) []*url.URL {
	t.Helper()
	out := []*url.URL{}
	for _, m := range regexp.MustCompile(`href="([^"]+)"`).FindAllSubmatch(data, -1) {
		value, err := url.Parse(html.UnescapeString(string(m[1])))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, value)
	}
	return out
}
func TestCanonicalNavigationPreservesFiltersAndPosition(t *testing.T) {
	handler := testHandler(t)
	q := url.Values{"search": {"nginx"}, "repository": {"example/read-only"}, "assessment": {"updates_found"}, "updateType": {"minor"}, "workflow": {"pr_open"}, "cursor": {"opaque-token"}, "revision": {"7"}}
	request := ui.Request{Method: "GET", Mode: "page", ContributionType: "page", ContributionID: "current", ConnectionID: "00000000-0000-4000-8000-000000000001", Query: q}
	response, err := handler.Handle(t.Context(), request, &host{})
	if err != nil || response.Status != 200 {
		t.Fatal(err)
	}
	coverage, detail, reset := false, false, false
	for _, u := range links(t, response.HTML) {
		if strings.HasSuffix(u.Path, "/coverage") || strings.HasSuffix(u.Path, "/"+strings.Repeat("a", 64)) {
			for key, values := range q {
				if u.Query().Get(key) != values[0] {
					t.Fatalf("lost %s in %s", key, u)
				}
			}
			coverage = coverage || strings.HasSuffix(u.Path, "/coverage")
			detail = detail || strings.HasSuffix(u.Path, "/"+strings.Repeat("a", 64))
		} else if strings.HasSuffix(u.Path, "/current") {
			reset = true
			if u.Query().Get("repository") != "example/read-only" || u.Query().Get("assessment") != "updates_found" || u.Query().Has("search") || u.Query().Has("cursor") || u.Query().Has("revision") {
				t.Fatalf("unsafe filter reset %s", u)
			}
		}
	}
	if !coverage || !detail || !reset {
		t.Fatal("missing contextual navigation")
	}
	if strings.Contains(string(response.HTML), "hx-get=") || !strings.Contains(string(response.HTML), `method="get"`) {
		t.Fatal("filters do not use canonical GET navigation")
	}
	for _, page := range []string{"detail", "coverage"} {
		request.ContributionID = page
		request.PathParams = map[string]string{"dependency-id": strings.Repeat("a", 64)}
		response, err = handler.Handle(t.Context(), request, &host{})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, u := range links(t, response.HTML) {
			if strings.HasSuffix(u.Path, "/current") {
				found = true
				for key, values := range q {
					if u.Query().Get(key) != values[0] {
						t.Fatalf("%s return lost %s", page, key)
					}
				}
			}
		}
		if !found {
			t.Fatalf("%s lacks contextual return", page)
		}
	}
}
func TestRecoveryClearsInvalidPositionAndInvalidFilters(t *testing.T) {
	handler := testHandler(t)
	for _, code := range []string{"conflict", "not_found", "invalid_argument"} {
		request := ui.Request{Method: "GET", Mode: "page", ContributionType: "page", ContributionID: "current", ConnectionID: "00000000-0000-4000-8000-000000000001", Query: url.Values{"search": {"nginx"}, "assessment": {"updates_found"}, "cursor": {"obsolete"}, "revision": {"5"}}}
		response, err := handler.Handle(t.Context(), request, &host{err: backend.Fail(code)})
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range links(t, response.HTML) {
			if u.Query().Has("cursor") || u.Query().Has("revision") {
				t.Fatalf("%s recovery retains invalid position", code)
			}
			if u.Query().Get("search") != "nginx" {
				t.Fatal("recovery lost valid filters")
			}

		}
	}
}
func TestHumanStatesAndResponsiveComparison(t *testing.T) {
	handler := testHandler(t)
	for _, page := range []string{"current", "detail", "coverage"} {
		request := ui.Request{Method: "GET", Mode: "page", ContributionType: "page", ContributionID: page, ConnectionID: "00000000-0000-4000-8000-000000000001", Query: url.Values{}}
		response, err := handler.Handle(t.Context(), request, &host{security: true, reasons: []string{"source_problems", "unassessed_dependencies"}})
		if err != nil {
			t.Fatal(err)
		}
		content := string(response.HTML)
		for _, token := range []string{">updates_found<", ">unknown · unknown<", ">source_problems<", ">unassessed_dependencies<"} {
			if strings.Contains(content, token) {
				t.Fatalf("%s exposes raw state %s", page, token)
			}
		}
		if !strings.Contains(content, "Observed at") || !strings.Contains(content, "Current observation") {
			t.Fatal("freshness/coverage lack explicit labels")
		}
		if page == "current" && (!strings.Contains(content, "image-upgrades-mobile") || !strings.Contains(content, "Reset to defaults")) {
			t.Fatal("missing responsive/reset composition")
		}
		if page == "current" && (!strings.Contains(content, `aria-label="Details for `) || !strings.Contains(content, `class="image-upgrades-proposals"`)) {
			t.Fatalf("missing contextual links=%t, workflow labels=%t or grouped candidates=%t", strings.Contains(content, `aria-label="Details for `), strings.Contains(content, "PR unknown"), strings.Contains(content, `class="image-upgrades-proposals"`))
		}
		if page == "detail" && (!strings.Contains(content, "Git reference") || !strings.Contains(content, "<dt>Candidate</dt>") || !strings.Contains(content, "Policy") || !strings.Contains(content, "Workflow")) {
			t.Fatal("missing labelled comparison")
		}
	}
}

func TestInvalidFilterWithoutPositionResetsQuery(t *testing.T) {
	request := ui.Request{Method: "GET", Mode: "page", ContributionType: "page", ContributionID: "current", ConnectionID: "00000000-0000-4000-8000-000000000001", Query: url.Values{"search": {"nginx"}, "assessment": {"invalid"}, "cursor": {"obsolete"}, "revision": {"5"}}}
	response, err := testHandler(t).Handle(t.Context(), request, &host{err: backend.Fail("invalid_argument")})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range links(t, response.HTML) {
		if u.Query().Has("search") || u.Query().Get("assessment") != "updates_found" {
			t.Fatal("invalid filters retained", u)
		}
	}
}
