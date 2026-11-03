package view

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestPinnedComparisonAndRetainedStatusShowExactDigests(t *testing.T) {
	old, next := "sha256:"+strings.Repeat("a", 63)+"1", "sha256:"+strings.Repeat("a", 63)+"2"
	for _, tag := range []*string{nil, new("1.0.0")} {
		d := advisory.Item{Dependency: snapshot.Dependency{CurrentValue: tag, CurrentDigest: &old}}
		u := snapshot.Candidate{NewValue: tag, NewDigest: &next, UpdateType: "digest"}
		r := requests.Record{Dependency: d.Dependency, Candidate: u, State: "open"}
		for _, component := range []templ.Component{CandidateSummary(d, u), RequestStatus(Model{}, r)} {
			var out bytes.Buffer
			if err := component.Render(t.Context(), &out); err != nil {
				t.Fatal(err)
			}
			body := out.String()
			if !strings.Contains(body, old) || !strings.Contains(body, next) || strings.Contains(body, "Not recorded") {
				t.Fatal("comparison omitted full approved digests")
			}
			if tag == nil && !strings.Contains(body, "Untagged image") {
				t.Fatal("absent tag shown as missing evidence")
			}
		}
		if tag == nil && candidateLabel(u) != "Digest update" {
			t.Fatal("digest-only candidate mislabeled")
		}
	}
}
