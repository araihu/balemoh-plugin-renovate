package view

import (
	"strings"
	"testing"
)

func TestRequestReviewDisclosesGeneratedFingerprint(t *testing.T) {
	m := listActionModel()
	m.Selected = m.Detail.Dependency.Candidates[0]
	if body := rendered(t, RequestReview(m)); strings.Contains(body, "will also be refreshed") {
		t.Fatal("disabled artifact disclosed")
	}
	m.Meta.Controller.GeneratedFingerprintFile = "docs/inventory.md"
	body := rendered(t, RequestReview(m))
	if !strings.Contains(body, "docs/inventory.md") || !strings.Contains(body, "will also be refreshed") || strings.Contains(body, "Other files and images are outside") {
		t.Fatal("request approval omits ancillary artifact")
	}
}
