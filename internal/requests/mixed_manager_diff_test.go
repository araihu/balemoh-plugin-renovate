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

func TestMixedManagersRequireExactSelectedKubernetesReferences(t *testing.T) {
	c, selection := setup(t)
	r, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	_, bindings := bindingProof(t)
	records := []Record{r}
	for _, binding := range bindings {
		records = append(records, bindingRecord(t, binding))
	}
	for _, record := range records {
		label := "tag-only"
		if record.Dependency.CurrentDigest != nil {
			label = record.Candidate.UpdateType + "/" + record.ID[:8]
		}
		t.Run(label, func(t *testing.T) {
			var identity []string
			if json.Unmarshal([]byte(record.Dependency.DependencyKey), &identity) != nil {
				t.Fatal("bad fixture identity")
			}
			ref := func(tag, digest *string) string {
				v := identity[0]
				if tag != nil {
					v += ":" + *tag
				}
				if digest != nil {
					v += "@" + *digest
				}
				return v
			}
			old, next := ref(record.Dependency.CurrentValue, record.Dependency.CurrentDigest), ref(record.Candidate.NewValue, record.Candidate.NewDigest)
			foreign := "imageName: " + old + "\n"
			image := "image: " + old + "\n"
			changed := "image: " + next + "\n"
			for _, test := range []struct {
				name, before, after, manager string
				count                        int
				ok                           bool
			}{
				{"selected-only", foreign + image, foreign + changed, "kubernetes", 1, true},
				{"both-managers", foreign + image, strings.ReplaceAll(foreign, old, next) + changed, "kubernetes", 1, false},
				{"wrong-manager", foreign + image, strings.ReplaceAll(foreign, old, next) + image, "kubernetes", 1, false},
				{"extra-edit", foreign + image, foreign + changed + "extra: changed\n", "kubernetes", 1, false},
				{"two-selected", foreign + image + image, foreign + changed + changed, "kubernetes", 2, true},
				{"partial-selected", foreign + image + image, foreign + changed + image, "kubernetes", 2, false},
				{"wrong-count", foreign + image, foreign + changed, "kubernetes", 2, false},
				{"unrelated-text", foreign + "note: " + old + "\n" + image, foreign + "note: " + old + "\n" + changed, "kubernetes", 1, true},
				{"changed-comment", foreign + image + "# " + old + "\n", foreign + changed + "# " + next + "\n", "kubernetes", 1, false},
				{"quoted-crlf", foreign + "  - image:\t'" + old + "'\r\n", foreign + "  - image:\t'" + next + "'\r\n", "kubernetes", 1, true},
				{"embedded-manifest", foreign + "script: |\n  cat <<EOF\n    image: \"" + old + "\"\n  EOF\n", foreign + "script: |\n  cat <<EOF\n    image: \"" + next + "\"\n  EOF\n", "kubernetes", 1, true},
				{"wrong-target", foreign + image, foreign + "image: other/image:1\n", "kubernetes", 1, false},
				{"changed-whitespace", foreign + image, foreign + "image:  " + next + "\n", "kubernetes", 1, false},
				{"quoted-key-not-native", foreign + "'image': " + old + "\n", foreign + "'image': " + next + "\n", "kubernetes", 1, false},
				{"inline-map-not-native", foreign + "container: {image: " + old + "}\n", foreign + "container: {image: " + next + "}\n", "kubernetes", 1, false},
				{"other-manager-remains-strict", foreign + image, foreign + changed, "docker-compose", 1, false},
				{"image-volume-remains-strict", foreign + "reference: " + old + "\n", foreign + "reference: " + next + "\n", "kubernetes", 1, false},
				{"tag-prefix", foreign + "image: " + old + "0\n", foreign + "image: " + next + "0\n", "kubernetes", 1, false},
				{"repository-suffix", foreign + "image: other/" + old + "\n", foreign + "image: other/" + next + "\n", "kubernetes", 1, false},
			} {
				t.Run(test.name, func(t *testing.T) {
					record.Dependency.Manager = test.manager
					record.Dependency.SourceOccurrences = test.count
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
						if q.Method != "GET" {
							t.Error("tracker attempted write")
							w.WriteHeader(405)
							return
						}
						if strings.HasSuffix(q.URL.Path, "/files") {
							_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": record.Dependency.PackageFile}})
							return
						}
						content := test.after
						if q.URL.Query().Get("ref") == record.Repository.Commit {
							content = test.before
						}
						_ = json.NewEncoder(w).Encode(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))})
					}))
					defer server.Close()
					g := github{endpoint: server.URL, client: server.Client(), token: "test"}
					err := g.verifyDiff(t.Context(), record, 42, strings.Repeat("c", 40), record.Repository.Commit)
					if (err == nil) != test.ok {
						t.Fatalf("acceptance=%v, want %v", err, test.ok)
					}
				})
			}
		})
	}
}

func TestMixedManagerPRRecoversReadOnlyWithoutWriterReplay(t *testing.T) {
	c, selection := setup(t)
	record, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	record.State = "needs_attention"
	record.Reason = "pr_observation_unavailable"
	record.TrackingError = true
	before := "imageName: nginx:1.25.3\nimage: nginx:1.25.3\n"
	after := "imageName: nginx:1.25.3\nimage: nginx:1.26.3\n"
	head := strings.Repeat("c", 40)
	p := nativePR{Number: 42, URL: "https://github.com/test/repo/pull/42", State: "open"}
	p.User.Login = "bot"
	p.Head.Ref = record.Branch
	p.Head.SHA = head
	p.Head.Repo.FullName = record.Repository.Name
	p.Base.Ref = record.Repository.BaseBranch
	p.Base.Repo.FullName = record.Repository.Name
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.Method != "GET" {
			t.Error("recovery attempted write")
			w.WriteHeader(405)
			return
		}
		switch {
		case q.URL.Path == "/repos/test/repo/pulls":
			_ = json.NewEncoder(w).Encode([]nativePR{p})
		case strings.HasSuffix(q.URL.Path, "/files"):
			_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": record.Dependency.PackageFile}})
		case strings.Contains(q.URL.Path, "/contents/"):
			content := after
			if q.URL.Query().Get("ref") == record.Repository.Commit {
				content = before
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))})
		case strings.HasSuffix(q.URL.Path, "/check-runs"):
			_, _ = w.Write([]byte(`{"total_count":1,"check_runs":[{"status":"completed","conclusion":"success"}]}`))
		case strings.HasSuffix(q.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"total_count":0,"statuses":[]}`))
		case strings.HasSuffix(q.URL.Path, "/pulls/42"):
			_ = json.NewEncoder(w).Encode(p)
		default:
			t.Error("unexpected read", q.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c.github = github{endpoint: server.URL, client: server.Client(), token: "test", author: "bot"}
	c.execute = func(context.Context, Record) error { t.Fatal("recovery replayed writer"); return nil }
	c.store.mu.Lock()
	err = c.store.save(record)
	c.store.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	c.cycle(t.Context())
	got := c.store.list()[0]
	if got.State != "open" || got.TrackingError || got.Reason != "" || got.PR == nil || got.PR.Checks != "passing" || got.PR.HeadSHA != head || got.CreatedAt != record.CreatedAt {
		t.Fatal("mixed-manager request did not recover read-only", got.State, got.TrackingError)
	}
	after = strings.ReplaceAll(after, "imageName: nginx:1.25.3", "imageName: nginx:1.26.3")
	c.cycle(t.Context())
	got = c.store.list()[0]
	if !got.TrackingError || got.PR == nil || got.PR.Checks != "unknown" {
		t.Fatal("unapproved manager edit kept passing evidence")
	}
}
