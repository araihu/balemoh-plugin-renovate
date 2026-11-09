package requests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func recoveryResponse(body any) *http.Response {
	data, _ := json.Marshal(body)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}
}

func TestBaseChangedRecoveryPreservesApproval(t *testing.T) {
	for _, kind := range []string{"queued", "retained-blocked", "changed-image", "changed-mode", "symlink", "unavailable", "writer-disabled"} {
		t.Run(kind, func(t *testing.T) {
			c, s := setup(t)
			r, err := c.Submit(t.Context(), s)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "retained-blocked" {
				r.State, r.Reason = "blocked", "base_changed"
				if err = c.store.save(r); err != nil {
					t.Fatal(err)
				}
			}
			_, present := githubFixture(t, c, r)
			original := c.github.client.Transport
			base := strings.Repeat("d", 40)
			c.github.client.Transport = responseTransport(func(req *http.Request) (*http.Response, error) {
				switch {
				case strings.HasSuffix(req.URL.Path, "/git/ref/heads/main"):
					return recoveryResponse(map[string]any{"object": map[string]string{"sha": base}}), nil
				case strings.Contains(req.URL.Path, "/contents/") && req.URL.Query().Get("ref") == base:
					if kind == "unavailable" {
						return nil, errors.New("unavailable")
					}
					value := "image: nginx:1.25.3\n"
					if kind == "changed-image" {
						value += "unapproved: edit\n"
					}
					return recoveryResponse(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(value))}), nil
				case strings.Contains(req.URL.Path, "/git/trees/"):
					mode := "100644"
					if strings.HasSuffix(req.URL.Path, "/"+base) {
						if kind == "changed-mode" {
							mode = "100755"
						}
						if kind == "symlink" {
							mode = "120000"
						}
					}
					return recoveryResponse(map[string]any{"tree": []map[string]string{{"path": r.Dependency.PackageFile, "type": "blob", "mode": mode}}}), nil
				}
				return original.RoundTrip(req)
			})
			calls := 0
			c.execute = func(_ context.Context, approved Record) error {
				calls++
				if approved.ID != r.ID || approved.Repository != r.Repository || approved.Candidate.ID != r.Candidate.ID || approved.SnapshotDigest != r.SnapshotDigest {
					t.Fatal("approval rewritten")
				}
				*present = true
				return nil
			}
			if kind == "writer-disabled" {
				c.config.WriterEnabled = false
			}
			c.cycle(t.Context())
			got := c.store.list()[0]
			want := kind == "queued" || kind == "retained-blocked"
			if (calls == 1) != want || want && (got.State != "open" || got.ID != r.ID || got.Repository != r.Repository) {
				t.Fatalf("calls=%d state=%s", calls, got.State)
			}
			if !want && kind != "writer-disabled" && (got.State != "blocked" || got.Reason != "base_changed") {
				t.Fatalf("unexpected rejection: %+v", got)
			}
			c.cycle(t.Context())
			if calls > 1 || len(c.store.list()) != 1 {
				t.Fatal("recovery duplicated execution or request")
			}
		})
	}
}

func TestRebaseObservationAndRestartNeverReplayUnknownPush(t *testing.T) {
	for _, outcome := range []string{"confirmed", "ambiguous", "prepare-failed", "disabled"} {
		t.Run(outcome, func(t *testing.T) {
			c, s := setup(t)
			r, err := c.Submit(t.Context(), s)
			if err != nil {
				t.Fatal(err)
			}
			p, present := githubFixture(t, c, r)
			*present = true
			c.config.GeneratedFingerprintFile = "docs/inventory.md"
			base, target := strings.Repeat("d", 40), strings.Repeat("e", 40)
			original := c.github.client.Transport
			c.github.client.Transport = responseTransport(func(req *http.Request) (*http.Response, error) {
				if strings.HasSuffix(req.URL.Path, "/git/ref/heads/main") {
					return recoveryResponse(map[string]any{"object": map[string]string{"sha": base}}), nil
				}
				if strings.Contains(req.URL.Path, "/compare/") {
					behind := 1
					if p.Head.SHA == target {
						behind = 0
					}
					return recoveryResponse(map[string]any{"behind_by": behind, "merge_base_commit": map[string]string{"sha": r.Repository.Commit}}), nil
				}
				return original.RoundTrip(req)
			})
			calls := 0
			c.execute = func(context.Context, Record) error { t.Fatal("native writer replayed"); return nil }
			c.rebase = func(_ context.Context, attempt Record, _ string) error {
				calls++
				attempt.PR.Checks = "unknown"
				if c.store.list()[0].PR.Checks != "passing" {
					t.Fatal("maintenance changed retained checks before persisting its marker")
				}
				if outcome == "prepare-failed" {
					return errors.New("prepare rejected")
				}
				attempt.State, attempt.Reason, attempt.RebaseHead = "needs_attention", "rebase_pending", target
				if err := c.store.save(attempt); err != nil {
					t.Fatal(err)
				}
				if outcome == "confirmed" {
					p.Head.SHA = target
					return nil
				}
				return errors.New("unknown push result")
			}
			if outcome == "disabled" {
				c.config.WriterEnabled = false
			}
			c.cycle(t.Context())
			c.cycle(t.Context())
			got := c.store.list()[0]
			if outcome == "disabled" {
				if calls != 0 || got.State != "open" {
					t.Fatal("disabled writer maintained branch")
				}
				return
			}
			if calls != 1 {
				t.Fatal("rebase replayed", calls)
			}
			if outcome == "confirmed" {
				if got.State != "open" || got.RebaseHead != "" || got.PR.Checks != "passing" {
					t.Fatal("new head not reconciled", got)
				}
				return
			}
			if got.State != "needs_attention" || got.PR.Checks != "unknown" {
				t.Fatal("uncertain update not retained", got)
			}
			c.Close()
			next, err := New(c.config)
			if err != nil {
				t.Fatal(err)
			}
			defer next.Close()
			next.github = c.github
			next.rebase = func(context.Context, Record, string) error { t.Fatal("restart replayed rebase"); return nil }
			next.execute = func(context.Context, Record) error { t.Fatal("restart replayed writer"); return nil }
			next.cycle(t.Context())
			if next.store.list()[0].State != "needs_attention" {
				t.Fatal("restart discarded pending maintenance")
			}
			if outcome == "ambiguous" {
				p.Head.SHA = target
				next.cycle(t.Context())
				if next.store.list()[0].State != "open" {
					t.Fatal("late push not recovered read-only")
				}
			}
		})
	}
}

func TestRebaseGuardRejectsChangedOwnershipHeadBaseAndAutomerge(t *testing.T) {
	for _, kind := range []string{"same", "head", "author", "automerge", "closed", "base", "repository"} {
		t.Run(kind, func(t *testing.T) {
			c, s := setup(t)
			r, _ := c.Submit(t.Context(), s)
			p, _ := githubFixture(t, c, r)
			r.PR = &PullRequest{Number: p.Number, HeadSHA: p.Head.SHA}
			switch kind {
			case "head":
				p.Head.SHA = strings.Repeat("e", 40)
			case "author":
				p.User.Login = "other"
			case "automerge":
				p.AutoMerge = json.RawMessage(`{}`)
			case "closed":
				p.State = "closed"
			case "repository":
				p.Head.Repo.FullName = "other/repo"
			}
			base := r.Repository.Commit
			if kind == "base" {
				base = strings.Repeat("d", 40)
			}
			if err := c.rebaseGuard(t.Context(), r, base); (err == nil) != (kind == "same") {
				t.Fatal("guard acceptance", err)
			}
		})
	}
}
