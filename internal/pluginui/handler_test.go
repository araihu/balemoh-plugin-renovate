package pluginui

import (
	"context"
	"encoding/json"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh-plugin-renovate/internal/view"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type host struct {
	calls     int
	err       error
	serviceID string
	security  bool
	reasons   []string
}

func (h *host) Placement(context.Context) (ui.Placement, error) {
	return ui.Placement{Options: json.RawMessage(`{}`)}, nil
}
func (h *host) Query(_ context.Context, id string, input json.RawMessage) (ui.QueryResult, error) {
	h.calls++
	if h.err != nil {
		return ui.QueryResult{}, h.err
	}
	now := time.Now()
	meta := advisory.Meta{Repository: snapshot.Repository{Name: "example/read-only"}, Coverage: snapshot.Coverage{Status: "partial", Unknown: 1, Reasons: h.reasons}, CompletedAt: now}
	d := snapshot.Dependency{ID: strings.Repeat("a", 64), PackageName: "<unsafe-image>", PackageFile: "k8s/test.yaml", Assessment: "unknown", Reasons: []string{}, Candidates: []snapshot.Candidate{}}
	if h.security {
		d.Candidates = []snapshot.Candidate{{IsVulnerabilityAlert: true, NewValue: new("2"), UpdateType: "major"}}
	}
	value := any(advisory.List{Meta: meta, Items: []advisory.Item{{Dependency: d, ServiceID: h.serviceID}}, Total: 1, Revision: "1"})
	if id == "upgrades.get" {
		value = advisory.Detail{Meta: meta, Dependency: advisory.Item{Dependency: d, ServiceID: h.serviceID}}
	}
	if id == "upgrades.summary" {
		value = advisory.Summary{Meta: meta, Assessments: map[string]int{"updates_found": 1}, UpdateTypes: map[string]int{"major": 1}, Candidates: 1}
		if h.security {
			v := value.(advisory.Summary)
			v.SecurityCandidates = 1
			value = v
		}
	}
	data, _ := json.Marshal(value)
	return ui.QueryResult{Data: data, State: "partial", ObservationRevision: "1", ObservedAt: &now, LastSuccessAt: &now}, nil
}
func TestReadOnlyPagesAndRecovery(t *testing.T) {
	data, e := os.ReadFile("../../plugin.json")
	if e != nil {
		t.Fatal(e)
	}
	p, _, e := manifest.Decode(strings.NewReader(string(data)))
	if e != nil {
		t.Fatal(e)
	}
	handler := Handler{Manifest: p}
	for _, page := range []string{"current", "detail", "coverage"} {
		r := ui.Request{Method: "GET", Mode: "page", ContributionType: "page", ContributionID: page, ConnectionID: "00000000-0000-4000-8000-000000000001", PathParams: map[string]string{"dependency-id": strings.Repeat("a", 64)}, Query: url.Values{}}
		h := &host{}
		response, e := handler.Handle(context.Background(), r, h)
		if e != nil || response.Status != 200 || h.calls != 1 || strings.Contains(string(response.HTML), "<unsafe-image>") {
			t.Fatalf("unsafe/failed page %s: %v", page, e)
		}
		h.err = backend.Fail("not_found")
		response, e = handler.Handle(context.Background(), r, h)
		if e != nil || response.Status != 404 || !strings.Contains(string(response.HTML), "no longer") {
			t.Fatal("missing detail recovery")
		}
		r.Method = "POST"
		before := h.calls
		if _, e := handler.Handle(context.Background(), r, h); e == nil || before != h.calls {
			t.Fatal("UI write method accepted")
		}
	}
}

// The optional catalog only resolves IDs supplied by an exact mapping. It never
// searches for a service by image name, and failed presentation keeps image data.
type catalogHost struct {
	host
	identities []ui.ServiceIdentity
	catalogErr error
	requested  []string
}

func (h *catalogHost) ResolveServices(_ context.Context, ids []string) ([]ui.ServiceIdentity, error) {
	h.requested = append(h.requested, ids...)
	return h.identities, h.catalogErr
}
func TestMappedServicePresentation(t *testing.T) {
	data, err := os.ReadFile("../../plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := manifest.Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("b", 64)
	for _, tc := range []struct {
		name       string
		identities []ui.ServiceIdentity
		err        error
		want       string
	}{
		{name: "image", identities: []ui.ServiceIdentity{{ID: id, Name: "Edge <proxy>", IconURL: "/icons/upload:test/image"}}, want: `src="/icons/upload:test/image"`},
		{name: "sprite", identities: []ui.ServiceIdentity{{ID: id, Name: "Edge <proxy>", IconSpriteURL: "/assets/icons.svg", IconSymbol: "nginx"}}, want: `/assets/icons.svg#nginx`},
		{name: "name only", identities: []ui.ServiceIdentity{{ID: id, Name: "Edge <proxy>"}}, want: "Edge &lt;proxy&gt;"},
		{name: "missing"},
		{name: "denied", err: backend.Fail("permission_denied")},
		{name: "unavailable", err: backend.Fail("unavailable")},
		{name: "foreign identity", identities: []ui.ServiceIdentity{{ID: strings.Repeat("c", 64), Name: "Unexpected service"}}},
		{name: "remote icon", identities: []ui.ServiceIdentity{{ID: id, Name: "Unexpected service", IconURL: "https://example.test/icon.svg"}}},
		{name: "incomplete sprite", identities: []ui.ServiceIdentity{{ID: id, Name: "Unexpected service", IconSymbol: "nginx"}}},
		{name: "duplicate identity", identities: []ui.ServiceIdentity{{ID: id, Name: "Unexpected service"}, {ID: id, Name: "Unexpected service"}}},
	} {
		for _, page := range []string{"current", "detail"} {
			t.Run(tc.name+"/"+page, func(t *testing.T) {
				h := &catalogHost{host: host{serviceID: id}, identities: tc.identities, catalogErr: tc.err}
				kind := "page"
				if page == "summary" {
					kind = "widget"
				}
				r := ui.Request{Method: "GET", Mode: "page", ContributionType: kind, ContributionID: page, ConnectionID: "00000000-0000-4000-8000-000000000001", PathParams: map[string]string{"dependency-id": strings.Repeat("a", 64)}, Query: url.Values{}}
				response, err := (Handler{Manifest: m}).Handle(t.Context(), r, h)
				html := string(response.HTML)
				if err != nil || response.Status != 200 || !strings.Contains(html, "&lt;unsafe-image&gt;") || h.calls != 1 || len(h.requested) != 1 || h.requested[0] != id {
					t.Fatalf("lost image data or invalid resolution: %v", err)
				}
				if tc.want != "" {
					if !strings.Contains(html, tc.want) || !strings.Contains(html, "Edge &lt;proxy&gt;") || strings.Contains(html, "Edge <proxy>") {
						t.Fatal("missing/unsafe service presentation")
					}
				} else if strings.Contains(html, "image-upgrades-service") || strings.Contains(html, "Unexpected service") {
					t.Fatal("unresolved service rendered")
				}
			})
		}
	}
	h := &catalogHost{}
	model := view.Model{Services: map[string]ui.ServiceIdentity{}}
	resolve(t.Context(), h, &model, []advisory.Item{{Dependency: snapshot.Dependency{PackageName: "nginx"}}})
	if len(h.requested) != 0 {
		t.Fatal("unmapped image triggered service lookup")
	}
	resolve(t.Context(), h, &model, []advisory.Item{{ServiceID: id}, {ServiceID: id}})
	if len(h.requested) != 1 {
		t.Fatal("duplicate service resolution")
	}
}

func TestSecurityFlagPresentation(t *testing.T) {
	data, err := os.ReadFile("../../plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := manifest.Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"current", "detail", "summary"} {
		kind := "page"
		if page == "summary" {
			kind = "widget"
		}
		r := ui.Request{Method: "GET", Mode: "page", ContributionType: kind, ContributionID: page, ConnectionID: "00000000-0000-4000-8000-000000000001", Query: url.Values{}}
		response, err := (Handler{Manifest: m}).Handle(t.Context(), r, &host{security: true})
		if err != nil || !strings.Contains(string(response.HTML), "Renovate security flag") {
			t.Fatalf("missing security badge on %s: %v", page, err)
		}
		response, err = (Handler{Manifest: m}).Handle(t.Context(), r, &host{})
		if err != nil || strings.Contains(string(response.HTML), "Renovate security flag") {
			t.Fatalf("invented security badge on %s", page)
		}
	}
}
