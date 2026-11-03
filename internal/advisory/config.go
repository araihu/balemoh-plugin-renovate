// Package advisory implements asynchronous collection and read-only SDK queries.
package advisory

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/credential"
)

type Mapping struct {
	DependencyID string `json:"dependencyId"`
	ServiceID    string `json:"serviceId"`
}
type Config struct {
	ControllerURL           string    `json:"controllerURL"`
	SnapshotURL             string    `json:"snapshotURL"`
	BrowserRepositoryURL    string    `json:"browserRepositoryURL"`
	IntervalSeconds         int       `json:"intervalSeconds"`
	MaxSnapshotAgeSeconds   int       `json:"maxSnapshotAgeSeconds"`
	NotificationsEnabled    bool      `json:"notificationsEnabled"`
	NotificationUpdateTypes []string  `json:"notificationUpdateTypes"`
	ServiceMappingsJSON     string    `json:"serviceMappingsJSON"`
	ServiceMappings         []Mapping `json:"-"`
}

func decode(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return backend.Fail("invalid_argument")
	}
	return nil
}
func parseConfig(data []byte, secrets map[string]string) (Config, *http.Client, error) {
	var c Config
	if !backend.Object(data, backend.MaxInput) || decode(data, &c) != nil {
		return c, nil, backend.Fail("invalid_argument")
	}
	if c.IntervalSeconds == 0 {
		c.IntervalSeconds = 60
	}
	if c.MaxSnapshotAgeSeconds == 0 {
		c.MaxSnapshotAgeSeconds = 43200
	}
	if !snapshot.SafeURL(c.SnapshotURL) || len(c.SnapshotURL) > 2048 || !strings.HasPrefix(c.BrowserRepositoryURL, "https://github.com/") || !snapshot.SafeURL(c.BrowserRepositoryURL) || strings.Count(strings.TrimPrefix(c.BrowserRepositoryURL, "https://github.com/"), "/") != 1 || c.IntervalSeconds < 30 || c.IntervalSeconds > 900 || c.MaxSnapshotAgeSeconds < 3600 || c.MaxSnapshotAgeSeconds > 604800 || len(c.ServiceMappingsJSON) > 50000 {
		return c, nil, backend.Fail("invalid_argument")
	}
	if c.ControllerURL != "" && (!snapshot.SafeURL(c.ControllerURL) || len(c.ControllerURL) > 2048 || strings.HasSuffix(c.ControllerURL, "/")) {
		return c, nil, backend.Fail("invalid_argument")
	}
	if len(c.NotificationUpdateTypes) > 6 {
		return c, nil, backend.Fail("invalid_argument")
	}
	for i, kind := range c.NotificationUpdateTypes {
		if !slices.Contains([]string{"major", "minor", "patch", "digest", "pinDigest", "pin"}, kind) || slices.Contains(c.NotificationUpdateTypes[:i], kind) {
			return c, nil, backend.Fail("invalid_argument")
		}
	}
	if c.ServiceMappingsJSON != "" {
		if !strings.HasPrefix(strings.TrimSpace(c.ServiceMappingsJSON), "[") || decode([]byte(c.ServiceMappingsJSON), &c.ServiceMappings) != nil || len(c.ServiceMappings) > 256 {
			return c, nil, backend.Fail("invalid_argument")
		}
	}
	seen := map[string]bool{}
	services := map[string]bool{}
	for _, m := range c.ServiceMappings {
		if !validID(m.DependencyID) || !validID(m.ServiceID) || seen[m.DependencyID] {
			return c, nil, backend.Fail("invalid_argument")
		}
		seen[m.DependencyID] = true
		services[m.ServiceID] = true
	}
	if len(services) > 64 {
		return c, nil, backend.Fail("invalid_argument")
	}
	for key, value := range secrets {
		if key != "snapshot-token" && key != "snapshot-ca" && key != "controller-read-token" && key != "controller-action-token" && key != "controller-ca" || (key == "snapshot-token" || key == "controller-read-token" || key == "controller-action-token") && !credential.ValidBearer(value) {
			return c, nil, backend.Fail("invalid_argument")
		}
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	for _, encoded := range []string{secrets["snapshot-ca"], secrets["controller-ca"]} {
		if encoded == "" {
			continue
		}
		der, e := base64.StdEncoding.DecodeString(encoded)
		if e != nil {
			return c, nil, backend.Fail("invalid_argument")
		}
		cert, e := x509.ParseCertificate(der)
		if e != nil {
			return c, nil, backend.Fail("invalid_argument")
		}
		roots.AddCert(cert)
	}
	if c.ControllerURL != "" && (secrets["controller-read-token"] == "" || secrets["controller-action-token"] == "" || secrets["controller-read-token"] == secrets["controller-action-token"]) {
		return c, nil, backend.Fail("invalid_argument")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return c, client, nil
}

func (c Config) notificationTypeAllowed(kind string) bool {
	if c.NotificationUpdateTypes == nil {
		return kind == "major" || kind == "minor" || kind == "patch"
	}
	return slices.Contains(c.NotificationUpdateTypes, kind)
}
func validID(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
