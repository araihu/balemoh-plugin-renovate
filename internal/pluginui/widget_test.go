package pluginui

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

type overviewHost struct {
	host
	input     advisory.Filter
	operation string
}

func (h *overviewHost) Placement(context.Context) (ui.Placement, error) {
	return ui.Placement{Options: json.RawMessage(`{"repository":"example/read-only","updateType":"minor","maxRows":1}`)}, nil
}
func (h *overviewHost) Query(ctx context.Context, id string, input json.RawMessage) (ui.QueryResult, error) {
	h.operation = id
	_ = json.Unmarshal(input, &h.input)
	return h.host.Query(ctx, id, input)
}
func TestWidgetUsesUnpaginatedScopedSummaryAndUnavailableIsNotZero(t *testing.T) {
	raw, err := os.ReadFile("../../plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := manifest.Decode(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	h := &overviewHost{}
	r := ui.Request{Method: "GET", Mode: "page", ContributionType: "widget", ContributionID: "summary", ConnectionID: "00000000-0000-4000-8000-000000000001", Query: url.Values{}}
	out, err := (Handler{Manifest: p}).Handle(t.Context(), r, h)
	if err != nil || out.Status != 200 || h.operation != "upgrades.summary" || h.calls != 1 || h.input.Repository != "example/read-only" || h.input.UpdateType != "minor" || h.input.Limit != 0 {
		t.Fatal("paginated or unscoped overview", err, h)
	}
	if !strings.Contains(string(out.HTML), "<article") {
		t.Fatal("missing card")
	}
	h.err = backend.Fail("unavailable")
	out, err = (Handler{Manifest: p}).Handle(t.Context(), r, h)
	if err != nil || out.Status != 503 || strings.Contains(string(out.HTML), "No upgrade candidates reported") {
		t.Fatal("unavailable became empty", err)
	}
}
