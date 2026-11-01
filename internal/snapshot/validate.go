package snapshot

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed snapshot.schema.json
var schemaBytes []byte
var compiled = sync.OnceValue(func() *jsonschema.Schema {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	var v any
	if json.Unmarshal(schemaBytes, &v) != nil {
		panic("invalid embedded schema")
	}
	if err := c.AddResource("urn:balemoh:image-upgrades:snapshot:1", v); err != nil {
		panic(err)
	}
	s, err := c.Compile("urn:balemoh:image-upgrades:snapshot:1")
	if err != nil {
		panic(err)
	}
	return s
})
var ErrInvalid = errors.New("invalid advisory snapshot")

// ID uses canonical JSON arrays. Strings are UTF-8 with no HTML escaping.
func ID(values ...any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(values)
	h := sha256.Sum256(bytes.TrimSuffix(b.Bytes(), []byte("\n")))
	return hex.EncodeToString(h[:])
}
func Decode(data []byte, now time.Time) (*Snapshot, error) {
	if len(data) > MaxBytes {
		return nil, ErrInvalid
	}
	var v any
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF || compiled().Validate(v) != nil {
		return nil, ErrInvalid
	}
	var s Snapshot
	if json.Unmarshal(data, &s) != nil || s.Validate(now) != nil {
		return nil, ErrInvalid
	}
	return &s, nil
}
func (s Snapshot) Validate(now time.Time) error {
	if s.Producer.RenovateVersion != RenovateVersion || s.Producer.RenovateImage != RenovateImage || s.Producer.AdapterVersion != AdapterVersion || s.StartedAt.After(s.CompletedAt) || s.CompletedAt.After(now.Add(5*time.Minute)) {
		return ErrInvalid
	}
	seen := map[string]bool{}
	assessed, excluded, unknown, occurrences := 0, 0, 0, 0
	for _, d := range s.Dependencies {
		if !imageManager(d.Manager) || d.ID != ID(s.Repository.Platform, s.Repository.Name, s.Repository.BaseBranch, d.Manager, d.PackageFile, d.DependencyKey, d.PackageName) || seen[d.ID] || !safePath(d.PackageFile) || !safePackage(d.PackageName) {
			return ErrInvalid
		}
		seen[d.ID] = true
		occurrences += max(d.SourceOccurrences, 1)
		if d.RegistryURL != "" && !SafeURL(d.RegistryURL) {
			return ErrInvalid
		}
		switch d.Assessment {
		case "updates_found", "no_candidate":
			assessed++
		case "excluded":
			excluded++
		case "unknown":
			unknown++
		default:
			return ErrInvalid
		}
		for _, c := range d.Candidates {
			if c.ID != ID(d.ID, c.NewValue, c.NewDigest, c.UpdateType) || seen[c.ID] {
				return ErrInvalid
			}
			seen[c.ID] = true
			if c.PR != nil && c.PR.URL != fmt.Sprintf("https://github.com/%s/pull/%d", s.Repository.Name, c.PR.Number) {
				return ErrInvalid
			}
		}
	}
	c := s.Coverage
	if c.Extracted != len(s.Dependencies) || c.Assessed != assessed || c.Excluded != excluded || c.Unknown != unknown || c.SourceRecords < occurrences || c.Status == "complete" && (unknown > 0 || len(c.Reasons) > 0 || c.SourceRecords != occurrences) {
		return ErrInvalid
	}
	return nil
}
func SafeURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Opaque == "" && !strings.ContainsAny(s, "\r\n\t\\")
}
func safePackage(s string) bool {
	return s != "" && !strings.Contains(s, "://") && !strings.ContainsAny(s, "@?#\r\n\t ")
}
func safePath(s string) bool {
	return s != "" && !strings.HasPrefix(s, "/") && !strings.ContainsAny(s, "\\\r\n\x00") && !strings.Contains("/"+s+"/", "/../")
}
func imageManager(m string) bool {
	switch m {
	case "kubernetes", "helm-values", "docker-compose", "dockerfile", "kustomize", "regex":
		return true
	}
	return false
}
