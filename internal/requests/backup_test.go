package requests

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func backupFixture(t *testing.T) ([]byte, Record) {
	t.Helper()
	c, selection := setup(t)
	record, e := c.Submit(t.Context(), selection)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(Catalog{Enabled: false, Requests: []Record{record}})
	if e != nil {
		t.Fatal(e)
	}
	return raw, record
}
func archiveFixture(t *testing.T, entries []struct {
	name string
	data []byte
	kind byte
}) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.tar")
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	tw := tar.NewWriter(f)
	for _, entry := range entries {
		h := &tar.Header{Name: entry.name, Size: int64(len(entry.data)), Mode: 0600, Typeflag: entry.kind}
		if entry.kind == tar.TypeSymlink || entry.kind == tar.TypeLink {
			h.Linkname = "/tmp/escape"
			h.Size = 0
		}
		if e := tw.WriteHeader(h); e != nil {
			t.Fatal(e)
		}
		if h.Size > 0 {
			if _, e := tw.Write(entry.data); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := tw.Close(); e != nil {
		t.Fatal(e)
	}
	if e := f.Close(); e != nil {
		t.Fatal(e)
	}
	return path
}
func TestRequestBackupRoundtripAndRetainedTargets(t *testing.T) {
	raw, record := backupFixture(t)
	archive := filepath.Join(t.TempDir(), "requests.tar")
	m, e := writeRequestBackup(raw, "test/repo", archive)
	if e != nil || m.RecordCount != 1 || m.ControllerWriterEnabled {
		t.Fatal("valid backup rejected", e)
	}
	destination := t.TempDir()
	restored, e := RestoreRequestBackup(archive, destination, "test/repo")
	if e != nil || restored.Files[record.ID+".json"] != m.Files[record.ID+".json"] {
		t.Fatal("restore failed", e)
	}
	path := filepath.Join(destination, "state", record.ID+".json")
	data, e := os.ReadFile(path)
	if e != nil || checksum(data) != m.Files[record.ID+".json"] {
		t.Fatal("record bytes changed", e)
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unsafe record mode")
	}
	info, e = os.Stat(filepath.Dir(path))
	if e != nil || info.Mode().Perm() != 0700 {
		t.Fatal("unsafe directory mode")
	}
	if _, e := RestoreRequestBackup(archive, destination, "test/repo"); e == nil {
		t.Fatal("retained target overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(data, after) {
		t.Fatal("retained bytes changed")
	}
	if _, e := writeRequestBackup(raw, "test/repo", archive); e == nil {
		t.Fatal("archive overwritten")
	}
}
func TestBackupRejectsUntrustedCatalogBeforeWriting(t *testing.T) {
	raw, _ := backupFixture(t)
	tests := [][]byte{
		[]byte(`{"enabled":false,"requests":null}`), []byte(`{"enabled":false,"enabled":true,"requests":[]}`), []byte(`{"requests":[]}`),
		bytes.Replace(raw, []byte(`"trackingError":false`), []byte(`"trackingError":false,"Reason":"unexpected alias"`), 1),
		append(bytes.Clone(raw), []byte(` {}`)...),
		bytes.Replace(raw, []byte(`"trackingError":false`), []byte(`"trackingError":false,"password":"private-secret"`), 1),
		bytes.Replace(raw, []byte(`"name":"test/repo"`), []byte(`"name":"test/repo","name":"other/repo"`), 1),
		bytes.Replace(raw, []byte(`"name":"test/repo"`), []byte(`"name":"other/repo"`), 1),
	}
	for i, test := range tests {
		path := filepath.Join(t.TempDir(), "requests.tar")
		if _, e := writeRequestBackup(test, "test/repo", path); e == nil {
			t.Fatalf("unsafe catalog accepted: %d", i)
		}
		if _, e := os.Stat(path); !os.IsNotExist(e) {
			t.Fatal("invalid catalog wrote archive")
		}
	}
}
func TestBackupArchiveCorruptionTraversalLinksAndUnknownFields(t *testing.T) {
	raw, r := backupFixture(t)
	m, files, e := backupCatalog(raw, "test/repo")
	if e != nil {
		t.Fatal(e)
	}
	manifest, _ := json.Marshal(m)
	record := files[r.ID+".json"]
	for _, mode := range []string{"corruption", "traversal", "absolute", "nested", "symlink", "hardlink", "duplicate", "unknown", "wrong repo"} {
		t.Run(mode, func(t *testing.T) {
			dest := t.TempDir()
			outside := filepath.Join(t.TempDir(), "sentinel.json")
			if err := os.WriteFile(outside, []byte("retained"), 0600); err != nil {
				t.Fatal(err)
			}
			entries := []struct {
				name string
				data []byte
				kind byte
			}{{"state/" + r.ID + ".json", record, tar.TypeReg}, {"state/.backup-manifest", manifest, tar.TypeReg}}
			repo := "test/repo"
			switch mode {
			case "corruption":
				entries[0].data = append(bytes.Clone(record), ' ')
			case "traversal":
				entries[0].name = "state/../escape.json"
			case "absolute":
				entries[0].name = outside
			case "nested":
				entries[0].name = "state/nested/" + r.ID + ".json"
			case "symlink":
				entries[0].kind = tar.TypeSymlink
			case "hardlink":
				entries[0].kind = tar.TypeLink
			case "duplicate":
				entries = append(entries, entries[0])
			case "unknown":
				entries = append(entries, struct {
					name string
					data []byte
					kind byte
				}{"state/credential", []byte("private-secret"), tar.TypeReg})
			case "wrong repo":
				repo = "other/repo"
			}
			if _, e := RestoreRequestBackup(archiveFixture(t, entries), dest, repo); e == nil {
				t.Fatal("invalid archive accepted")
			}
			left, _ := os.ReadDir(dest)
			if len(left) != 0 {
				t.Fatal("invalid archive wrote partial state")
			}
			if data, err := os.ReadFile(outside); err != nil || string(data) != "retained" {
				t.Fatal("invalid archive changed retained external file", err)
			}
		})
	}
}
func TestLegacyPythonV1EmptyArchive(t *testing.T) {
	// Version 1 used Python tarfile with a private state directory and JSON manifest.
	raw := []byte(`{"enabled":false,"requests":[]}`)
	manifest := []byte(`{"schemaVersion":1,"repository":"test/repo","recordCount":0,"controllerWriterEnabled":false,"catalogSHA256":"` + checksum(raw) + `","files":{}}`)
	path := archiveFixture(t, []struct {
		name string
		data []byte
		kind byte
	}{{"state/", nil, tar.TypeDir}, {"state/.backup-manifest", manifest, tar.TypeReg}})
	m, e := RestoreRequestBackup(path, t.TempDir(), "test/repo")
	if e != nil || m.RecordCount != 0 {
		t.Fatal("legacy empty archive incompatible", e)
	}
}
func TestExportUsesReaderTLSAndRefusesRedirect(t *testing.T) {
	raw, _ := backupFixture(t)
	calls := 0
	redirect := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/requests" || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("r", 32) {
			t.Fatal("wrong reader request")
		}
		if redirect {
			w.Header().Set("Location", "https://example.invalid/requests")
			w.WriteHeader(302)
			return
		}
		_, _ = w.Write(raw)
	}))
	defer server.Close()
	dir := t.TempDir()
	token := filepath.Join(dir, "reader")
	ca := filepath.Join(dir, "ca")
	if e := os.WriteFile(token, []byte(strings.Repeat("r", 32)), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(ca, []byte(base64.StdEncoding.EncodeToString(server.TLS.Certificates[0].Certificate[0])), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := ExportRequestBackup(t.Context(), server.URL+"/v1/requests", token, ca, "test/repo", filepath.Join(dir, "requests.tar")); e != nil {
		t.Fatal("TLS export failed", e)
	}
	redirect = true
	if _, e := ExportRequestBackup(t.Context(), server.URL+"/v1/requests", token, ca, "test/repo", filepath.Join(dir, "redirect.tar")); e == nil || calls != 2 {
		t.Fatal("redirect followed or accepted")
	}
	if _, e := ExportRequestBackup(t.Context(), strings.Replace(server.URL, "https://", "http://", 1)+"/v1/requests", token, ca, "test/repo", filepath.Join(dir, "http.tar")); e == nil || calls != 2 {
		t.Fatal("insecure export attempted")
	}
	f, e := os.Open(filepath.Join(dir, "requests.tar"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if _, e := io.Copy(io.Discard, f); e != nil {
		t.Fatal(e)
	}
}
