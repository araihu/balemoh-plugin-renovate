package requests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinnedDiffUsesCompleteReferences(t *testing.T) {
	_, cases := bindingProof(t)
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			r := bindingRecord(t, tc)
			ref := func(tag, digest *string) string {
				v := "127.0.0.1:5500/fixture/image"
				if tag != nil {
					v += ":" + *tag
				}
				return v + "@" + *digest
			}
			old, next := ref(r.Dependency.CurrentValue, r.Dependency.CurrentDigest), ref(r.Candidate.NewValue, r.Candidate.NewDigest)
			original := "image: " + old + "\n"
			for _, test := range []struct {
				name, before, after string
				count               int
				ok                  bool
			}{
				{"exact", original, "image: " + next + "\n", 1, true},
				{"quoted", "image: '" + old + "'\n", "image: '" + next + "'\n", 1, true},
				{"repeated", original + original, strings.ReplaceAll(original+original, old, next), 2, true},
				{"extra-edit", original, "image: " + next + "\nextra: changed\n", 1, false},
				{"digest-prefix", "image: " + old + "0\n", "image: " + next + "0\n", 1, false},
				{"repository-suffix", "image: other/" + old + "\n", "image: other/" + next + "\n", 1, false},
				{"mixed-digest", original + strings.ReplaceAll(original, *r.Dependency.CurrentDigest, "sha256:"+strings.Repeat("b", 64)), strings.ReplaceAll(original, old, next) + strings.ReplaceAll(original, *r.Dependency.CurrentDigest, "sha256:"+strings.Repeat("b", 64)), 2, false},
				{"partial-occurrences", original + original, strings.Replace(original+original, old, next, 1), 2, false},
				{"wrong-target", original, "image: " + strings.ReplaceAll(next, *r.Candidate.NewDigest, "sha256:"+strings.Repeat("b", 64)) + "\n", 1, false},
			} {
				t.Run(test.name, func(t *testing.T) {
					r.Dependency.SourceOccurrences = test.count
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
						if q.Method != "GET" {
							t.Error("tracker attempted write")
							w.WriteHeader(405)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						if strings.HasSuffix(q.URL.Path, "/files") {
							_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": r.Dependency.PackageFile}})
							return
						}
						data := test.before
						if q.URL.Query().Get("ref") != r.Repository.Commit {
							data = test.after
						}
						_ = json.NewEncoder(w).Encode(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(data))})
					}))
					defer server.Close()
					g := github{endpoint: server.URL, client: server.Client(), token: "test"}
					if err := g.verifyDiff(t.Context(), r, 42, strings.Repeat("c", 40), r.Repository.Commit); (err == nil) != test.ok {
						t.Fatalf("diff acceptance=%v, want %v", err, test.ok)
					}
				})
			}
		})
	}
}

func TestPinnedAndOldTagReceiptsSurviveCopiedStoreWithoutReplay(t *testing.T) {
	c, selection := setup(t)
	old, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	_, cases := bindingProof(t)
	for _, tc := range cases {
		r := bindingRecord(t, tc)
		c.store.mu.Lock()
		err = c.store.save(r)
		c.store.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}
	source := c.config.DataDir
	c.Close()
	restored := t.TempDir()
	files, err := filepath.Glob(filepath.Join(source, "*.json"))
	if err != nil || len(files) != 4 {
		t.Fatal("missing retained receipts", err)
	}
	for _, file := range files {
		data, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(restored, filepath.Base(file)), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	cfg := c.config
	cfg.DataDir = restored
	cfg.WriterEnabled = false
	cfg.GitHubWriteTokenFile = ""
	next, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	next.execute = func(context.Context, Record) error { t.Fatal("restored write replayed"); return nil }
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	next.Run(ctx)
	got := next.store.list()
	if len(got) != 4 {
		t.Fatal("restore dropped receipts")
	}
	for _, r := range got {
		want := "needs_attention"
		if r.ID == old.ID {
			want = "queued"
		}
		if r.State != want || r.Validate() != nil {
			t.Fatal("restore changed identity or recovery state", r.State)
		}
	}
}
