package view

import (
	"net/url"
	"strings"
)

type widgetCount struct {
	Label, Href string
	Count       int
}

func (m Model) widgetTypes() []widgetCount {
	rows := []widgetCount{}
	for _, kind := range []string{"major", "minor", "patch", "digest", "pinDigest", "pin", "unknown"} {
		n := m.Summary.UpdateTypes[kind]
		if m.Filter.UpdateType != "" && !strings.Contains(","+m.Filter.UpdateType+",", ","+kind+",") {
			continue
		}
		if n == 0 && kind != "major" && kind != "minor" && kind != "patch" {
			continue
		}
		href, _ := url.Parse(m.BackURL)
		q := href.Query()
		q.Set("updateType", kind)
		href.RawQuery = q.Encode()
		rows = append(rows, widgetCount{Label: label(kind), Count: n, Href: href.String()})
	}
	return rows
}
func (m Model) widgetRequests() []widgetCount {
	rows := []widgetCount{}
	for _, group := range []struct {
		label  string
		states []string
	}{
		{"Open PRs", []string{"open"}}, {"Draft PRs", []string{"draft"}},
		{"In progress", []string{"queued", "running", "awaiting_pr"}},
		{"Needs review", []string{"blocked", "needs_attention"}},
		{"Merged", []string{"merged"}}, {"Closed", []string{"closed"}},
	} {
		n := 0
		for _, state := range group.states {
			n += m.Summary.RequestStates[state]
		}
		if n > 0 {
			rows = append(rows, widgetCount{Label: group.label, Count: n})
		}
	}
	return rows
}
func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
