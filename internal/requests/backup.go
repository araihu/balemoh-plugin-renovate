package requests

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/credential"
)

const backupLimit = 2 << 20

var invalidBackup = errors.New("invalid request backup")

type BackupManifest struct {
	SchemaVersion           int               `json:"schemaVersion"`
	Repository              string            `json:"repository"`
	RecordCount             int               `json:"recordCount"`
	ControllerWriterEnabled bool              `json:"controllerWriterEnabled"`
	CatalogSHA256           string            `json:"catalogSHA256"`
	Files                   map[string]string `json:"files"`
}

func checksum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Strict decoding rejects duplicate keys as well as unknown record fields, including nested fields.
func backupJSON(data []byte, out any) error {
	// encoding/json accepts case-insensitive field aliases; the archive contract does not.
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return invalidBackup
	}
	typ := reflect.TypeOf(out).Elem()
	for key := range fields {
		found := false
		for i := 0; i < typ.NumField(); i++ {
			name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			if key == name {
				found = true
				break
			}
		}
		if !found {
			return invalidBackup
		}
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var walk func() error
	walk = func() error {
		token, e := d.Token()
		if e != nil {
			return e
		}
		if token == json.Delim('{') {
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return invalidBackup
				}
				seen[s] = true
				if e = walk(); e != nil {
					return e
				}
			}
			token, e = d.Token()
			if e != nil || token != json.Delim('}') {
				return invalidBackup
			}
		} else if token == json.Delim('[') {
			for d.More() {
				if e = walk(); e != nil {
					return e
				}
			}
			token, e = d.Token()
			if e != nil || token != json.Delim(']') {
				return invalidBackup
			}
		}
		return nil
	}
	if walk() != nil {
		return invalidBackup
	}
	if _, e := d.Token(); e != io.EOF {
		return invalidBackup
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		return invalidBackup
	}
	return nil
}
func backupRecord(data []byte, repository string) (string, error) {
	var r Record
	if backupJSON(data, &r) != nil || r.Repository.Name != repository || r.Validate() != nil {
		return "", invalidBackup
	}
	return r.ID + ".json", nil
}
func backupCatalog(raw []byte, repository string) (BackupManifest, map[string][]byte, error) {
	var wire struct {
		Enabled  *bool           `json:"enabled"`
		Requests json.RawMessage `json:"requests"`
	}
	if len(raw) > backupLimit || backupJSON(raw, &wire) != nil || wire.Enabled == nil || len(bytes.TrimSpace(wire.Requests)) == 0 || bytes.TrimSpace(wire.Requests)[0] != '[' {
		return BackupManifest{}, nil, invalidBackup
	}
	var rows []json.RawMessage
	if json.Unmarshal(wire.Requests, &rows) != nil || len(rows) > 512 {
		return BackupManifest{}, nil, invalidBackup
	}
	m := BackupManifest{SchemaVersion: 1, Repository: repository, RecordCount: len(rows), ControllerWriterEnabled: *wire.Enabled, CatalogSHA256: checksum(raw), Files: map[string]string{}}
	files := map[string][]byte{}
	for _, row := range rows {
		name, e := backupRecord(row, repository)
		if e != nil {
			return BackupManifest{}, nil, e
		}
		if _, ok := files[name]; ok {
			return BackupManifest{}, nil, invalidBackup
		}
		// Preserve each validated record's representation; hashes bind exact archive bytes.
		files[name] = bytes.Clone(row)
		m.Files[name] = checksum(row)
	}
	return m, files, nil
}
func ExportRequestBackup(ctx context.Context, endpoint, tokenFile, caFile, repository, archive string) (BackupManifest, error) {
	parsed, e := url.Parse(endpoint)
	if e != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "/v1/requests" || strings.TrimSpace(repository) == "" {
		return BackupManifest{}, invalidBackup
	}
	token, e := credential.ReadBearerFile(tokenFile)
	if e != nil {
		return BackupManifest{}, invalidBackup
	}
	ca, e := readBounded(caFile, 64<<10)
	if e != nil {
		return BackupManifest{}, invalidBackup
	}
	der, e := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(ca)))
	if e != nil {
		return BackupManifest{}, invalidBackup
	}
	cert, e := x509.ParseCertificate(der)
	if e != nil {
		return BackupManifest{}, invalidBackup
	}
	roots, e := x509.SystemCertPool()
	if e != nil {
		roots = x509.NewCertPool()
	}
	roots.AddCert(cert)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, e := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if e != nil {
		return BackupManifest{}, invalidBackup
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Accept", "application/json")
	response, e := client.Do(r)
	if e != nil {
		return BackupManifest{}, invalidBackup
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return BackupManifest{}, invalidBackup
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, backupLimit+1))
	if e != nil {
		return BackupManifest{}, invalidBackup
	}
	return writeRequestBackup(raw, repository, archive)
}
func writeRequestBackup(raw []byte, repository, path string) (BackupManifest, error) {
	m, files, e := backupCatalog(raw, repository)
	if e != nil {
		return BackupManifest{}, e
	}
	manifest, e := json.Marshal(m)
	if e != nil {
		return BackupManifest{}, e
	}
	files[".backup-manifest"] = manifest
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return BackupManifest{}, e
	}
	tw := tar.NewWriter(f)
	write := func() error {
		if e := tw.WriteHeader(&tar.Header{Name: "state", Typeflag: tar.TypeDir, Mode: 0700}); e != nil {
			return e
		}
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if e := tw.WriteHeader(&tar.Header{Name: "state/" + name, Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(files[name]))}); e != nil {
				return e
			}
			if _, e := tw.Write(files[name]); e != nil {
				return e
			}
		}
		return tw.Close()
	}
	e = write()
	if e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e != nil {
		return BackupManifest{}, e
	}
	return m, nil
}
func RestoreRequestBackup(archive, destination, repository string) (BackupManifest, error) {
	target := filepath.Join(destination, "state")
	if _, e := os.Lstat(target); !os.IsNotExist(e) {
		return BackupManifest{}, errors.New("refusing to overwrite retained state")
	}
	f, e := os.Open(archive)
	if e != nil {
		return BackupManifest{}, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.Size() > backupLimit*2 {
		return BackupManifest{}, invalidBackup
	}
	tr := tar.NewReader(io.LimitReader(f, backupLimit*2+1))
	files := map[string][]byte{}
	total := 0
	for {
		item, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return BackupManifest{}, invalidBackup
		}
		if (item.Name == "state" || item.Name == "state/") && item.Typeflag == tar.TypeDir && item.Size == 0 {
			continue
		}
		name := strings.TrimPrefix(item.Name, "state/")
		if (item.Typeflag != tar.TypeReg && item.Typeflag != tar.TypeRegA) || item.Size < 0 || item.Size > backupLimit || item.Name != "state/"+name || strings.Contains(name, "/") || name != ".backup-manifest" && (!strings.HasSuffix(name, ".json") || !identifier.MatchString(strings.TrimSuffix(name, ".json"))) {
			return BackupManifest{}, invalidBackup
		}
		if _, ok := files[name]; ok || len(files) >= 513 {
			return BackupManifest{}, invalidBackup
		}
		b, e := io.ReadAll(io.LimitReader(tr, backupLimit+1))
		total += len(b)
		if e != nil || int64(len(b)) != item.Size || total > backupLimit*2 {
			return BackupManifest{}, invalidBackup
		}
		files[name] = b
	}
	var m BackupManifest
	if backupJSON(files[".backup-manifest"], &m) != nil || m.SchemaVersion != 1 || m.Repository != repository || m.Files == nil || m.RecordCount != len(files)-1 || len(m.Files) != m.RecordCount {
		return BackupManifest{}, invalidBackup
	}
	for name, want := range m.Files {
		data, ok := files[name]
		if !ok || !identifier.MatchString(want) || checksum(data) != want {
			return BackupManifest{}, invalidBackup
		}
		actual, e := backupRecord(data, repository)
		if e != nil || actual != name {
			return BackupManifest{}, invalidBackup
		}
	}
	// Validate every member before creating state. A failed write leaves its partial target for diagnosis.
	if e = os.Mkdir(target, 0700); e != nil {
		return BackupManifest{}, e
	}
	root, e := os.OpenRoot(target)
	if e != nil {
		return BackupManifest{}, e
	}
	defer root.Close()
	for name, data := range files {
		file, e := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return BackupManifest{}, e
		}
		_, e = file.Write(data)
		if e == nil {
			e = file.Sync()
		}
		e = errors.Join(e, file.Close())
		if e != nil {
			return BackupManifest{}, e
		}
	}
	dir, e := root.Open(".")
	if e != nil {
		return BackupManifest{}, e
	}
	e = errors.Join(dir.Sync(), dir.Close())
	return m, e
}
