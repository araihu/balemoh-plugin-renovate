package view

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func TestSourceIconsFollowManagerAndKeepAccessibleIDsUnique(t *testing.T) {
	for _, tc := range []struct{ manager, file, symbol string }{
		{"dockerfile", "Dockerfile", "docker-docker"},
		{"docker-compose", "compose.yaml", "docker-docker"},
		{"kubernetes", "compose.yaml", "kubernetes-kubernetes"},
		{"helm-values", "values.yaml", "kubernetes-kubernetes"},
		{"kustomize", "kustomization.yaml", "kubernetes-kubernetes"},
		{"regex", "Dockerfile", ""},
		{"regex", "k8s/deployment.yaml", ""},
		{"unknown", "compose.yaml", ""},
	} {
		t.Run(tc.manager+"/"+tc.file, func(t *testing.T) {
			d := advisory.Item{Dependency: snapshot.Dependency{ID: strings.Repeat("a", 64), Manager: tc.manager, PackageName: "nginx", PackageFile: tc.file}}
			m := Model{Request: ui.Request{ConnectionID: "00000000-0000-4000-8000-000000000001", ContributionID: "current"}, List: advisory.List{Items: []advisory.Item{d}, Total: 1}}
			m.Detail = advisory.Detail{Dependency: d}
			for _, page := range []struct {
				name      string
				count     int
				component templ.Component
			}{{"current", 2, Current(m)}, {"detail", 1, Detail(m)}} {
				var out bytes.Buffer
				if err := page.component.Render(t.Context(), &out); err != nil {
					t.Fatal(err)
				}
				body := out.String()
				if tc.symbol == "" {
					if strings.Contains(body, `aria-label="Image source:`) {
						t.Fatal("source inferred from filename instead of manager")
					}
				} else if strings.Count(body, "/ui/icons/sprite.svg#"+tc.symbol) != page.count || strings.Count(body, `aria-label="Image source: `+managerLabel(tc.manager)+`"`) != page.count {
					t.Fatalf("missing source icon or accessible name in %s", page.name)
				}
				seen := map[string]bool{}
				for _, match := range regexp.MustCompile(`\sid="([^"]+)"`).FindAllStringSubmatch(body, -1) {
					if seen[match[1]] {
						t.Fatal("duplicate responsive DOM ID", match[1])
					}
					seen[match[1]] = true
				}
			}
		})
	}
}
