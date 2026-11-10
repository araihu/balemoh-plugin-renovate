package view

import (
	"regexp"
	"strings"
	"testing"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
)

func TestIconActionsKeepNamesTooltipsAndResponsiveIDs(t *testing.T) {
	m := listActionModel()
	m.List.Items = []advisory.Item{m.Detail.Dependency}
	m.List.Total = 1
	body := rendered(t, Current(m))
	seen := map[string]bool{}
	for _, match := range regexp.MustCompile(`\sid="([^"]+)"`).FindAllStringSubmatch(body, -1) {
		if seen[match[1]] {
			t.Fatal("duplicate responsive ID", match[1])
		}
		seen[match[1]] = true
	}
	count := 0
	for _, anchor := range regexp.MustCompile(`(?s)<a\b([^>]*)>(.*?)</a>`).FindAllStringSubmatch(body, -1) {
		if !strings.Contains(anchor[1], "image-upgrades-icon-action") {
			continue
		}
		count++
		if !strings.Contains(anchor[1], "for nginx in a.yaml") || !strings.Contains(anchor[2], `<svg`) || !strings.Contains(anchor[2], `aria-hidden="true"`) {
			t.Fatal("icon action lost its accessible context or decorative SVG", anchor[0])
		}
		description := regexp.MustCompile(`aria-describedby="([^"]+)"`).FindStringSubmatch(anchor[1])
		if len(description) != 2 || !strings.Contains(body, `id="`+description[1]+`"`) || !seen[description[1]] {
			t.Fatal("action has no matching tooltip", anchor[0])
		}
		if text := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(anchor[2], ""); strings.TrimSpace(text) != "" {
			t.Fatal("icon action still has visible text", text)
		}
	}
	if count != 4 {
		t.Fatalf("got %d icon links; want review and details in both responsive layouts", count)
	}
}

func TestExternalIconActionKeepsSafeNavigationAndTabNotice(t *testing.T) {
	m := listActionModel()
	r := requests.Record{ID: "request-a", Dependency: m.Detail.Dependency.Dependency, State: "open", PR: &requests.PullRequest{Number: 15, URL: "https://github.com/test/repo/pull/15"}}
	m.Meta.Controller.Requests = []requests.Record{r}
	for _, body := range []string{rendered(t, ListActions(m, "table")), rendered(t, RequestStatus(m, r, "detail"))} {
		for _, want := range []string{`href="https://github.com/test/repo/pull/15"`, `target="_blank"`, `rel="noopener noreferrer"`, "View PR #15 for nginx in a.yaml · opens in a new tab", `role="tooltip"`} {
			if !strings.Contains(body, want) {
				t.Fatal("external action lost navigation or accessible help", want)
			}
		}
	}
}
