package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestRebaseCommitAndLeaseUseRealGit(t *testing.T) {
	for _, kind := range []string{"exact", "pinned", "wrong-target", "wrong-digest", "concurrent-head", "changed-source", "changed-mode", "extra-native-edit", "extra-native-file", "fingerprint-edit", "attributes-clean-crlf", "attributes-smudge-crlf", "attributes-noop"} {
		t.Run(kind, func(t *testing.T) {
			c, s := setup(t)
			r, _ := c.Submit(t.Context(), s)
			dir := t.TempDir()
			env := append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
			git := func(args ...string) string {
				t.Helper()
				b, e := rebaseGit(t.Context(), dir, env, args...)
				if e != nil {
					t.Fatal(args, e)
				}
				return strings.TrimSpace(string(b))
			}
			write := func(name, value string) {
				t.Helper()
				p := filepath.Join(dir, name)
				if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(p, []byte(value), 0600); e != nil {
					t.Fatal(e)
				}
				if e := os.Chmod(p, 0644); e != nil {
					t.Fatal(e)
				}
			}
			fingerprint := "docs/inventory.md"
			git("init")
			git("config", "user.name", "Native Renovate")
			git("config", "user.email", "native@example.test")
			before := "imageName: nginx:1.25.3\nimage: nginx:1.25.3\n"
			upgraded := "imageName: nginx:1.25.3\nimage: nginx:1.26.3\n"
			if kind == "pinned" || kind == "wrong-digest" {
				r.Dependency.CurrentDigest = new("sha256:" + strings.Repeat("a", 64))
				r.Candidate.NewDigest = new("sha256:" + strings.Repeat("b", 64))
				r.Candidate.ID = snapshot.ID(r.Dependency.ID, r.Candidate.NewValue, r.Candidate.NewDigest, r.Candidate.UpdateType)
				before = strings.ReplaceAll(before, "nginx:1.25.3", "nginx:1.25.3@"+*r.Dependency.CurrentDigest)
				upgraded = "imageName: nginx:1.25.3@" + *r.Dependency.CurrentDigest + "\nimage: nginx:1.26.3@" + *r.Candidate.NewDigest + "\n"
			}
			if kind == "attributes-clean-crlf" {
				before = strings.ReplaceAll(before, "\n", "\r\n")
				upgraded = strings.ReplaceAll(upgraded, "\n", "\r\n")
			}
			write(r.Dependency.PackageFile, before)
			write(fingerprint, "# Inventory\n- Source digest: `"+strings.Repeat("a", 64)+"`\nSource files: 2\n")
			git("add", ".")
			git("commit", "-m", "base")
			approved := git("rev-parse", "HEAD")
			r.Repository.Commit = approved
			r.ID = snapshot.ID(r.Repository, r.Dependency.ID, r.Candidate.ID)
			r.Branch = "balemoh-renovate/" + r.ID
			git("checkout", "-b", r.Branch)
			if kind == "wrong-target" {
				upgraded = strings.ReplaceAll(upgraded, "1.26.3", "1.27.0")
			}
			if kind == "wrong-digest" {
				upgraded = strings.ReplaceAll(upgraded, *r.Candidate.NewDigest, "sha256:"+strings.Repeat("c", 64))
			}
			if kind == "extra-native-edit" {
				upgraded += "extra: edit\n"
			}
			write(r.Dependency.PackageFile, upgraded)
			if e := refreshFingerprint(t.Context(), dir, fingerprint, env); e != nil {
				t.Fatal(e)
			}
			if kind == "extra-native-file" {
				write("extra.txt", "unapproved\n")
			}
			if kind == "fingerprint-edit" {
				write(fingerprint, "# changed\n- Source digest: `"+strings.Repeat("b", 64)+"`\n")
			}
			git("add", ".")
			git("commit", "-m", "native selected image")
			native := git("rev-parse", "HEAD")
			r.PR = &PullRequest{Number: 42, HeadSHA: native, State: "open"}
			git("checkout", "--detach", approved)
			write("notes.md", "new main content\n")
			write(fingerprint, "# Inventory\n- Source digest: `"+strings.Repeat("c", 64)+"`\nSource files: 3\n")
			if kind == "changed-source" {
				write(r.Dependency.PackageFile, before+"extra: new main\n")
			}
			if kind == "changed-mode" {
				if e := os.Chmod(filepath.Join(dir, r.Dependency.PackageFile), 0755); e != nil {
					t.Fatal(e)
				}
			}
			if strings.HasPrefix(kind, "attributes-") {
				ending := "lf"
				if kind == "attributes-smudge-crlf" {
					ending = "crlf"
				}
				write(".gitattributes", "*.yaml text eol="+ending+"\n")
				git("add", "--", "notes.md", fingerprint, ".gitattributes")
			} else {
				git("add", ".")
			}
			git("commit", "-m", "main advances")
			base := git("rev-parse", "HEAD")
			if strings.HasPrefix(kind, "attributes-") {
				blob, err := rebaseGit(t.Context(), dir, env, "show", base+":"+r.Dependency.PackageFile)
				if err != nil || !bytes.Equal(blob, []byte(before)) {
					t.Fatal("attribute-only base changed the approved blob", err)
				}
			}
			work := t.TempDir()
			head, err := rebaseCommit(t.Context(), r, base, approved, dir, work, fingerprint, env)
			positive := kind == "exact" || kind == "pinned" || kind == "concurrent-head" || kind == "attributes-noop"
			if (err == nil) != positive {
				t.Fatalf("acceptance=%v want positive=%t", err, positive)
			}
			if !positive {
				return
			}
			out := func(args ...string) string {
				t.Helper()
				b, e := rebaseGit(t.Context(), work, env, args...)
				if e != nil {
					t.Fatal(args, e)
				}
				return strings.TrimSpace(string(b))
			}
			if out("rev-parse", "HEAD^") != base || out("show", "HEAD:notes.md") != "new main content" {
				t.Fatal("new main content lost")
			}
			imageBlob, err := rebaseGit(t.Context(), work, env, "show", "HEAD:"+r.Dependency.PackageFile)
			if err != nil || !bytes.Equal(imageBlob, []byte(upgraded)) {
				t.Fatal("approved image bytes changed", err)
			}
			beforeHash := out("show", "HEAD:"+fingerprint)
			if e := refreshFingerprint(t.Context(), work, fingerprint, env); e != nil {
				t.Fatal(e)
			}
			afterHash, err := os.ReadFile(filepath.Join(work, fingerprint))
			if err != nil || !bytes.Equal([]byte(beforeHash), bytes.TrimSpace(afterHash)) {
				t.Fatal("fingerprint not reproducible")
			}
			if kind == "concurrent-head" {
				git("update-ref", "refs/heads/"+r.Branch, base, native)
			}
			_, err = rebaseGit(t.Context(), work, env, "push", "--force-with-lease=refs/heads/"+r.Branch+":"+native, "origin", "HEAD:refs/heads/"+r.Branch)
			if kind == "concurrent-head" {
				if err == nil || git("rev-parse", r.Branch) != base {
					t.Fatal("concurrent branch overwritten")
				}
				return
			}
			if err != nil || git("rev-parse", r.Branch) != head {
				t.Fatal("verified head not published", err)
			}
		})
	}
}

func TestRebaseMarkerSurvivesBackupValidation(t *testing.T) {
	c, s := setup(t)
	r, _ := c.Submit(t.Context(), s)
	r.State, r.Reason, r.RebaseHead = "needs_attention", "rebase_pending", strings.Repeat("e", 40)
	data, _ := json.Marshal(r)
	if id, err := backupRecord(data, r.Repository.Name); err != nil || id != r.ID+".json" {
		t.Fatal("pending rebase backup rejected", id, err)
	}
	r.RebaseHead = "invalid"
	if r.Validate() == nil {
		t.Fatal("bad recovery head accepted")
	}
	r.RebaseHead = strings.Repeat("e", 40)
	r.State = "queued"
	if r.Validate() == nil {
		t.Fatal("pending write queued for replay")
	}
}

func TestMissingPRRetainsPendingRebaseMarker(t *testing.T) {
	c, selection := setup(t)
	record, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	pr, present := githubFixture(t, c, record)
	*present = true
	c.cycle(t.Context())
	pending := c.store.list()[0]
	pending.State, pending.Reason, pending.RebaseHead = "needs_attention", "rebase_pending", strings.Repeat("e", 40)
	if err := c.store.save(pending); err != nil {
		t.Fatal(err)
	}
	writes := 0
	c.execute = func(context.Context, Record) error { writes++; return nil }
	c.rebase = func(context.Context, Record, string) error { writes++; return nil }
	*present = false
	c.cycle(t.Context())
	got := c.store.list()[0]
	if got.State != "needs_attention" || got.Reason != "rebase_pending" || got.RebaseHead != pending.RebaseHead || !got.TrackingError || got.PR.Checks != "unknown" || writes != 0 {
		t.Fatalf("pending publication lost or replayed: %+v writes=%d", got, writes)
	}
	data, _ := json.Marshal(got)
	if _, err := backupRecord(data, got.Repository.Name); err != nil {
		t.Fatal("pending marker no longer backup compatible", err)
	}
	*present = true
	pr.Head.SHA = pending.RebaseHead
	c.cycle(t.Context())
	confirmed := c.store.list()[0]
	if confirmed.State != "open" || confirmed.RebaseHead != "" || confirmed.Reason != "" || confirmed.TrackingError || confirmed.PR.Checks != "passing" || writes != 0 {
		t.Fatalf("expected head not confirmed read-only: %+v writes=%d", confirmed, writes)
	}
}
