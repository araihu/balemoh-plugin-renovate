package requests

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPRFingerprintAllowsOnlyApprovedImageAndChecksum(t *testing.T) {
	c, selection := setup(t)
	record, err := c.Submit(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	artifact := "docs/inventory.md"
	base := strings.Repeat("d", 40)
	head := strings.Repeat("c", 40)
	before := "# Inventory\n- Source digest: `" + strings.Repeat("a", 64) + "`\n"
	after := strings.ReplaceAll(before, strings.Repeat("a", 64), strings.Repeat("b", 64))
	for _, tc := range []struct {
		name, path, status, content, configured, base, mode string
		extra, changedImage, ok                             bool
	}{
		{name: "exact", path: artifact, status: "modified", content: after, configured: artifact, base: base, ok: true},
		{name: "disabled", path: artifact, status: "modified", content: after, base: base},
		{name: "wrong-file", path: "other.md", status: "modified", content: after, configured: artifact, base: base},
		{name: "renamed", path: artifact, status: "renamed", content: after, configured: artifact, base: base},
		{name: "deleted", path: artifact, status: "removed", content: after, configured: artifact, base: base},
		{name: "extra-text", path: artifact, status: "modified", content: after + "extra\n", configured: artifact, base: base},
		{name: "extra-file", path: artifact, status: "modified", content: after, configured: artifact, base: base, extra: true},
		{name: "extra-image-edit", path: artifact, status: "modified", content: after, configured: artifact, base: base, changedImage: true},
		{name: "executable", path: artifact, status: "modified", content: after, configured: artifact, base: base, mode: "100755"},
		{name: "symlink", path: artifact, status: "modified", content: after, configured: artifact, base: base, mode: "120000"},
		{name: "missing-base", path: artifact, status: "modified", content: after, configured: artifact},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Error("unexpected tracker write")
					w.WriteHeader(405)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/files") {
					files := []map[string]string{{"filename": record.Dependency.PackageFile}, {"filename": tc.path, "status": tc.status}}
					if tc.extra {
						files = append(files, map[string]string{"filename": "extra.yaml"})
					}
					_ = json.NewEncoder(w).Encode(files)
					return
				}
				if strings.Contains(r.URL.Path, "/git/trees/") {
					mode := tc.mode
					if mode == "" {
						mode = "100644"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"truncated": false, "tree": []map[string]string{{"path": artifact, "type": "blob", "mode": mode}}})
					return
				}
				data := "image: nginx:1.25.3\n"
				if strings.HasSuffix(r.URL.Path, "/"+artifact) {
					data = before
					if r.URL.Query().Get("ref") == head {
						data = tc.content
					} else if r.URL.Query().Get("ref") != base {
						t.Error("artifact compared to frozen image base")
					}
				} else if r.URL.Query().Get("ref") == head {
					data = "image: nginx:1.26.3\n"
					if tc.changedImage {
						data += "unapproved: edit\n"
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(data))})
			}))
			defer server.Close()
			g := github{endpoint: server.URL, client: server.Client(), token: "test", fingerprintFile: tc.configured}
			if err := g.verifyDiff(t.Context(), record, 42, head, tc.base); (err == nil) != tc.ok {
				t.Fatalf("acceptance=%v want %v", err, tc.ok)
			}
		})
	}
}

func TestFingerprintCatalogIsCompatibleWithBackup(t *testing.T) {
	raw, record := backupFixture(t)
	raw = []byte(strings.Replace(string(raw), `"enabled":false`, `"enabled":false,"generatedFingerprintFile":"docs/inventory.md"`, 1))
	catalog, err := DecodeCatalog(raw)
	if err != nil || catalog.GeneratedFingerprintFile != "docs/inventory.md" {
		t.Fatal("catalog rejected", err)
	}
	manifest, files, err := backupCatalog(raw, "test/repo")
	if err != nil || manifest.RecordCount != 1 || len(files[record.ID+".json"]) == 0 {
		t.Fatal("backup rejected metadata", err)
	}
	unsafe := []byte(strings.Replace(string(raw), "docs/inventory.md", "../escape.md", 1))
	if _, err := DecodeCatalog(unsafe); err == nil {
		t.Fatal("unsafe catalog path accepted")
	}
	if _, _, err := backupCatalog(unsafe, "test/repo"); err == nil {
		t.Fatal("unsafe backup path accepted")
	}
}
