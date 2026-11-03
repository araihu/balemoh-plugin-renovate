//go:build digestproof

package requests

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Opt-in GET-only tracking of the native fixture branches, without controller admission or execution.
func TestNativePinnedPRTracking(t *testing.T) {
	token, err := exec.Command("gh", "auth", "token", "--hostname", "github.com").Output()
	if err != nil {
		t.Fatal("authenticated GitHub CLI required")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	g := github{endpoint: "https://api.github.com", token: strings.TrimSpace(string(token)), author: "github-actions[bot]", client: client}
	_, cases := bindingProof(t)
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			r := bindingRecord(t, tc)
			r.Repository.Name = "guilycst/balemoh-renovate-engine-poc-20261101"
			r.Branch = tc.Branch
			pr, err := g.pull(t.Context(), r)
			if err != nil || pr == nil {
				t.Fatal("native fixture PR association failed")
			}
			if pr.Number != tc.PR || pr.HeadSHA != tc.Head || pr.State != "open" || pr.Checks != "unknown" {
				t.Fatal("native fixture head/state/check evidence differs")
			}
			t.Logf("verified fixture PR #%d, exact head %s, state %s, checks %s", pr.Number, pr.HeadSHA, pr.State, pr.Checks)
		})
	}
}
