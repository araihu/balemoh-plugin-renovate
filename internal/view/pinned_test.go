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
		d.Candidates = []snapshot.Candidate{u}
		r := requests.Record{Dependency: d.Dependency, Candidate: u, State: "open"}
		for _, component := range []templ.Component{CandidateSummary(d, u), RequestStatus(Model{}, r), ExactReferences(d), Detail(Model{Detail: advisory.Detail{Dependency: d}})} {
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

func TestUnknownReferenceDoesNotInventAnUntaggedImage(t *testing.T) {
	var out bytes.Buffer
	if err := ReferenceValue(nil, nil, true).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Not recorded") || strings.Contains(out.String(), "Untagged image") {
		t.Fatal("unknown reference presented as confirmed untagged image")
	}
}

func TestRetainedPinnedStatusAnnouncesTrackingFailure(t *testing.T) {
	old, next := "sha256:"+strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64)
	r := requests.Record{State: "open", TrackingError: true, Dependency: snapshot.Dependency{CurrentDigest: &old}, Candidate: snapshot.Candidate{NewDigest: &next, UpdateType: "digest"}, PR: &requests.PullRequest{URL: "https://github.com/test/repo/pull/42", Checks: "unknown"}}
	m := Model{}
	m.Meta.ControllerAvailable = true
	var out bytes.Buffer
	if err := RequestStatus(m, r).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{"Request tracking unavailable", "Showing the last retained status", "Checks unknown or incomplete", old, next} {
		if !strings.Contains(body, want) {
			t.Fatal("missing retained-status evidence", want)
		}
	}
	if strings.Contains(body, "Reported checks passed") {
		t.Fatal("stale status claims passing checks")
	}
}
