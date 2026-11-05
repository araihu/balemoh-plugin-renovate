package pluginui

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func TestCheckboxFiltersPreserveMultipleSelections(t *testing.T) {
	q := url.Values{"assessment": {"", "updates_found", "unknown"}, "updateType": {"", "minor", "patch"}, "workflow": {"", "pr_open", "unknown"}}
	f := advisory.Filter{Assessment: querySelection(q, "assessment"), UpdateType: querySelection(q, "updateType"), Workflow: querySelection(q, "workflow")}
	if !advisory.ValidFilter(f) {
		t.Fatal("invalid parsed selection")
	}
	canonical := filterQuery(f)
	for _, key := range []string{"assessment", "updateType", "workflow"} {
		if !reflect.DeepEqual(canonical[key], q[key][1:]) {
			t.Fatalf("lost selection %s", key)
		}
	}
	h := testHandler(t)
	for _, page := range []string{"current", "detail", "coverage"} {
		r := ui.Request{Method: "GET", Mode: "page", ContributionType: "page", ContributionID: page, ConnectionID: "00000000-0000-4000-8000-000000000001", Query: q, PathParams: map[string]string{"dependency-id": strings.Repeat("a", 64)}}
		response, err := h.Handle(t.Context(), r, &host{})
		if err != nil || response.Status != 200 {
			t.Fatalf("%s: %v", page, err)
		}
		found := false
		for _, u := range links(t, response.HTML) {
			if strings.HasSuffix(u.Path, "/coverage") || strings.HasSuffix(u.Path, "/"+strings.Repeat("a", 64)) || page != "current" && strings.HasSuffix(u.Path, "/current") {
				found = true
				for key, values := range canonical {
					if !reflect.DeepEqual(u.Query()[key], values) {
						t.Fatalf("%s lost %s in %s", page, key, u)
					}
				}
			}
		}
		if !found {
			t.Fatalf("%s missing navigation", page)
		}
		if page == "current" {
			content := string(response.HTML)
			if strings.Count(content, `data-combobox-multi="true"`) != 3 || strings.Count(content, `data-combobox-persist="false"`) != 3 {
				t.Fatal("expected three URL-owned multiple comboboxes")
			}
			if !strings.Contains(content, `type="checkbox"`) {
				t.Fatal("missing checkbox options")
			}
			if strings.Count(content, `value=""`) < 3 {
				t.Fatal("empty selections must submit explicitly")
			}
		}
	}
}
func TestClearedAssessmentSurvivesQueryRoundTrip(t *testing.T) {
	q := url.Values{"assessment": {""}, "updateType": {"", "digest"}}
	f := advisory.Filter{Assessment: querySelection(q, "assessment"), UpdateType: querySelection(q, "updateType")}
	roundTrip := filterQuery(f)
	if !roundTrip.Has("assessment") || roundTrip.Get("assessment") != "" || roundTrip.Get("updateType") != "digest" {
		t.Fatal("clear restored default or lost other filters")
	}
}
