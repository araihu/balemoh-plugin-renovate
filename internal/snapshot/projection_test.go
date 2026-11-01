package snapshot

import "testing"

func TestActualRenovateBranchProjection(t *testing.T) {
	report := []byte(`{"repositories":{"example/read-only":{"status":"activated","packageFiles":{"kubernetes":[{"packageFile":"a.yaml","deps":[{"datasource":"docker","depName":"nginx","depType":"container","currentValue":"1","updates":[{"newValue":"2","updateType":"major","branchName":"renovate/nginx"}]}]}]},"branches":[{"branchName":"renovate/nginx","result":"needs-approval","prNo":42,"upgrades":[{"datasource":"docker","depName":"nginx","currentValue":"1","newValue":"2","packageFile":"a.yaml","updateType":"major","packageName":"nginx"}]}]}}}`)
	s, err := Normalize(report, metadata())
	if err != nil {
		t.Fatal(err)
	}
	candidate := s.Dependencies[0].Candidates[0]
	if candidate.PR == nil || candidate.Policy != "approval_required" || candidate.Workflow != "pr_open" {
		t.Fatalf("actual Renovate branch projection lost association: policy=%q workflow=%q PR=%+v", candidate.Policy, candidate.Workflow, candidate.PR)
	}
}

func TestNativePRLimitPolicy(t *testing.T) {
	report := []byte(`{"repositories":{"example/read-only":{"status":"activated","packageFiles":{"kubernetes":[{"packageFile":"a.yaml","deps":[{"datasource":"docker","depName":"nginx","currentValue":"1","updates":[{"newValue":"2","updateType":"major","branchName":"renovate/nginx"}]}]}]},"branches":[{"branchName":"renovate/nginx","result":"pr-limit-reached","upgrades":[{"depName":"nginx","currentValue":"1","newValue":"2","packageFile":"a.yaml","updateType":"major","packageName":"nginx"}]}]}}}`)
	s, err := Normalize(report, metadata())
	if err != nil {
		t.Fatal(err)
	}
	if s.Dependencies[0].Candidates[0].Policy != "pr_limit" {
		t.Fatal("PR limit evidence lost")
	}
}
