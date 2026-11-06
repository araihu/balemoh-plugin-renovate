package requests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHeadChangeFailsClosedUntilCurrentDiffIsVerified(t *testing.T) {
	_, cases := bindingProof(t)
	cases = append([]bindingCase{{Case: "tag-only"}}, cases...)
	for _, tc := range cases {
		for _, retained := range []bool{false, true} {
			name := tc.Case + "/new"
			if retained {
				name = tc.Case + "/retained"
			}
			t.Run(name, func(t *testing.T) {
				c, selection := setup(t)
				var r Record
				if tc.Case == "tag-only" {
					var err error
					r, err = c.Submit(t.Context(), selection)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					r = bindingRecord(t, tc)
				}
				r.State = "needs_attention"
				r.Reason = "execution_interrupted"
				h1, h2 := strings.Repeat("1", 40), strings.Repeat("2", 40)
				url := "https://github.com/test/repo/pull/42"
				if retained {
					r.PR = &PullRequest{Number: 42, URL: url, HeadSHA: h1, State: "open", Checks: "passing"}
					r.State = "open"
					r.Reason = ""
				}
				reference := func(tag, digest *string) string {
					v := "127.0.0.1:5500/fixture/image"
					if tc.Case == "tag-only" {
						v = "nginx"
					}
					if tag != nil {
						v += ":" + *tag
					}
					if digest != nil {
						v += "@" + *digest
					}
					return "image: " + v + "\n"
				}
				before, after := reference(r.Dependency.CurrentValue, r.Dependency.CurrentDigest), reference(r.Candidate.NewValue, r.Candidate.NewDigest)
				listedHead, latestHead, approvedH2 := h1, h2, false
				p := nativePR{Number: 42, URL: url, State: "open"}
				p.User.Login = "bot"
				p.Head.Ref = r.Branch
				p.Head.Repo.FullName = r.Repository.Name
				p.Base.Ref = r.Repository.BaseBranch
				p.Base.Repo.FullName = r.Repository.Name
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
					if q.Method != "GET" {
						t.Error("tracker attempted write")
						w.WriteHeader(405)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					switch {
					case q.URL.Path == "/repos/test/repo/pulls":
						row := p
						row.Head.SHA = listedHead
						_ = json.NewEncoder(w).Encode([]nativePR{row})
					case strings.HasSuffix(q.URL.Path, "/pulls/42/files"):
						_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": r.Dependency.PackageFile}})
					case strings.Contains(q.URL.Path, "/contents/"):
						content := after
						if q.URL.Query().Get("ref") == r.Repository.Commit {
							content = before
						} else if q.URL.Query().Get("ref") == h2 && !approvedH2 {
							content += "unapproved: edit\n"
						}
						_ = json.NewEncoder(w).Encode(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))})
					case strings.HasSuffix(q.URL.Path, "/check-runs"):
						_, _ = w.Write([]byte(`{"total_count":1,"check_runs":[{"status":"completed","conclusion":"success"}]}`))
					case strings.HasSuffix(q.URL.Path, "/status"):
						_, _ = w.Write([]byte(`{"total_count":0,"statuses":[]}`))
					case strings.HasSuffix(q.URL.Path, "/pulls/42"):
						row := p
						row.Head.SHA = latestHead
						_ = json.NewEncoder(w).Encode(row)
					default:
						t.Error("unexpected tracking request", q.URL.Path)
						w.WriteHeader(404)
					}
				}))
				defer server.Close()
				c.github = github{endpoint: server.URL, client: server.Client(), token: "test", author: "bot"}
				c.execute = func(_ctx context.Context, _r Record) error { t.Fatal("tracking failure replayed writer"); return nil }
				c.store.mu.Lock()
				err := c.store.save(r)
				c.store.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				for _, phase := range []string{"changed-head", "unapproved-current-diff"} {
					c.cycle(t.Context())
					got := c.store.list()[0]
					if !got.TrackingError || got.State != r.State || got.Reason != r.Reason || got.TrackedAt == nil {
						t.Fatalf("%s: false fresh association: state=%s trackingError=%t reason=%q", phase, got.State, got.TrackingError, got.Reason)
					}
					if !retained && got.PR != nil {
						t.Fatal("unverified current PR newly associated")
					}
					if retained && (got.PR == nil || got.PR.HeadSHA != h1 || got.PR.Checks != "unknown") {
						t.Fatal("retained evidence lost or stale passing checks kept")
					}
					if retained && r.PR.Checks != "passing" {
						t.Fatal("observation mutated original receipt")
					}
					listedHead = h2
				}
				approvedH2 = true
				c.cycle(t.Context())
				got := c.store.list()[0]
				if got.TrackingError || got.PR == nil || got.PR.HeadSHA != h2 || got.PR.Checks != "passing" || got.State != "open" || got.Reason != "" {
					t.Fatalf("stable head not reconciled: state=%s trackingError=%t reason=%q", got.State, got.TrackingError, got.Reason)
				}
			})
		}
	}
}
