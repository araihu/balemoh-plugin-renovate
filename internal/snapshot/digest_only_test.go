package snapshot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDigestOnlyReferenceRetainsNativeBranchAssociation(t *testing.T) {
	empty := ""
	currentDigest := "sha256:" + strings.Repeat("a", 64)
	newDigest := "sha256:" + strings.Repeat("b", 64)
	report := rawReport{Repositories: map[string]rawRepo{"example/read-only": {
		Status: "activated", Files: map[string][]rawFile{"kubernetes": {{File: "a.yaml", Deps: []rawDependency{{Datasource: "docker", Name: "nginx", Current: &empty, Digest: &currentDigest, Updates: []rawUpdate{{Digest: &newDigest, Type: "digest", Branch: "renovate/nginx"}}}}}}},
		Branches: []rawBranch{{Name: "renovate/nginx", PR: 42, Result: "needs-approval", Upgrades: []rawUpgrade{{File: "a.yaml", Package: "nginx", Name: "nginx", Current: &empty, CurrentDigest: &currentDigest, Digest: &newDigest, Type: "digest"}}}},
	}}}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Normalize(data, metadata())
	if err != nil {
		t.Fatal(err)
	}
	d := s.Dependencies[0]
	c := d.Candidates[0]
	if d.CurrentValue != nil || d.CurrentDigest == nil || *d.CurrentDigest != currentDigest || c.NewValue != nil || c.PR == nil || c.PR.Number != 42 || c.Policy != "approval_required" {
		t.Fatalf("digest-only reference or native association lost: %+v", d)
	}
}
