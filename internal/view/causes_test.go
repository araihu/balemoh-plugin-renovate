package view

import (
	"bytes"
	"strings"
	"testing"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestCausePresentationAndLegacyFallback(t *testing.T) {
	for _, tc := range []struct{ cause, title, detail string }{
		{"cause_latest_tag_missing", "Default latest tag not found", "registry returned 404"},
		{"cause_registry_access_denied", "Registry access denied", "does not prove the package is private"},
		{"cause_registry_rate_limited", "Registry rate limit", "HTTP 429"},
		{"cause_internal_host_blocked", "Internal registry blocked", "network policy"},
		{"cause_empty_helm_tag", "Empty Helm image tag", "chart may supply it"},
		{"cause_not_recorded", "Cause not recorded in this snapshot", "new collection"},
	} {
		d := advisory.Item{Dependency: snapshot.Dependency{Assessment: "unknown", Reasons: []string{"lookup_unconfirmed"}}}
		if tc.cause != "cause_not_recorded" {
			d.Reasons = append(d.Reasons, tc.cause)
		}
		var out bytes.Buffer
		if err := Assessment(d).Render(t.Context(), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), tc.title) {
			t.Fatalf("missing cause %s", tc.title)
		}
		if tc.cause != "cause_not_recorded" && !strings.Contains(reasonLabel(tc.cause), tc.detail) {
			t.Fatal("missing explanation")
		}
		if len(displayReasons(d)) != 1 {
			t.Fatal("generic lookup warning duplicates the cause")
		}
	}
}
