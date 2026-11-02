package tracking

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixture(t *testing.T) (*App, *rsa.PrivateKey, *time.Time) {
	t.Helper()
	dir := t.TempDir()
	credentials := filepath.Join(dir, "credentials")
	out := filepath.Join(dir, "output")
	if e := os.Mkdir(credentials, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(out, 0700); e != nil {
		t.Fatal(e)
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	for name, value := range map[string][]byte{"app-id": []byte("123"), "installation-id": []byte("456"), "private-key.pem": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})} {
		if e := os.WriteFile(filepath.Join(credentials, name), value, 0600); e != nil {
			t.Fatal(e)
		}
	}
	c, e := Parse(map[string]string{"UPGRADES_REPOSITORY": "different/project", "UPGRADES_APP_SLUG": "example-reader", "UPGRADES_APP_OWNER": "app-owner", "UPGRADES_APP_CREDENTIALS_DIR": credentials, "UPGRADES_APP_TOKEN_DIR": out})
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Truncate(time.Second)
	a := New(c)
	a.now = func() time.Time { return now }
	return a, key, &now
}
func response(a *App, path string) map[string]any {
	switch {
	case path == "/app":
		return map[string]any{"id": 123, "slug": a.config.AppSlug, "owner": map[string]any{"login": a.config.AppOwner}, "permissions": maps.Clone(permissions), "events": []string{}}
	case path == "/repos/different/project/installation":
		return map[string]any{"id": 456, "app_id": 123, "account": map[string]any{"login": "different"}, "repository_selection": "selected", "permissions": maps.Clone(permissions)}
	case path == "/app/installations/456/access_tokens":
		return map[string]any{"token": strings.Repeat("t", 48), "expires_at": a.now().Add(time.Hour), "permissions": maps.Clone(permissions)}
	case path == "/installation/repositories":
		return map[string]any{"total_count": 1, "repositories": []map[string]any{{"full_name": a.config.Repository}}}
	case strings.Contains(path, "/pulls/") && !strings.HasSuffix(path, "/files"):
		return map[string]any{"head": map[string]any{"sha": strings.Repeat("a", 40), "repo": map[string]any{"full_name": a.config.Repository}}}
	case strings.HasSuffix(path, "/check-runs"):
		return map[string]any{"total_count": 2}
	case strings.HasSuffix(path, "/status"):
		return map[string]any{"total_count": 0}
	default:
		return map[string]any{}
	}
}
func fake(a *App, edit func(*http.Request, map[string]any)) {
	a.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		v := response(a, r.URL.Path)
		if edit != nil {
			edit(r, v)
		}
		b, _ := json.Marshal(v)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b))), Header: make(http.Header), Request: r}, nil
	})
}
func TestIssueScopesTokenAndSignsJWT(t *testing.T) {
	a, key, _ := fixture(t)
	mints := 0
	fake(a, func(r *http.Request, v map[string]any) {
		if r.URL.Host != "api.github.com" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatal("untrusted endpoint or authentication scheme")
		}
		if r.Method == "POST" {
			mints++
			var body struct {
				Repositories []string          `json:"repositories"`
				Permissions  map[string]string `json:"permissions"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Repositories) != 1 || body.Repositories[0] != "project" || !maps.Equal(body.Permissions, permissions) {
				t.Fatal("broad token request")
			}
		}
	})
	issued, e := a.Issue(t.Context())
	if e != nil || mints != 1 || len(issued.Token) != 48 {
		t.Fatal("valid App rejected", e)
	}
	id, jwt, e := a.jwt()
	if e != nil || id != "123" {
		t.Fatal(e)
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatal("invalid JWT")
	}
	signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature) != nil {
		t.Fatal("JWT signature invalid")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	_ = json.Unmarshal(raw, &claims)
	if claims["iss"] != "123" || int64(claims["exp"].(float64))-int64(claims["iat"].(float64)) != 600 {
		t.Fatal("unexpected JWT claims")
	}
}
func TestIssueRejectsBroadOrUnexpectedAuthority(t *testing.T) {
	tests := []struct {
		name, path string
		edit       func(map[string]any)
		beforeMint bool
	}{
		{"App owner", "/app", func(v map[string]any) { v["owner"] = map[string]string{"login": "other"} }, true},
		{"App permissions", "/app", func(v map[string]any) { v["permissions"] = map[string]string{"contents": "write"} }, true},
		{"App slug", "/app", func(v map[string]any) { v["slug"] = "other" }, true},
		{"App events", "/app", func(v map[string]any) { v["events"] = []string{"push"} }, true},
		{"all repositories", "/repos/different/project/installation", func(v map[string]any) { v["repository_selection"] = "all" }, true},
		{"installation identity", "/repos/different/project/installation", func(v map[string]any) { v["id"] = 999 }, true},
		{"installation permissions", "/repos/different/project/installation", func(v map[string]any) { v["permissions"] = map[string]string{"contents": "write"} }, true},
		{"suspended", "/repos/different/project/installation", func(v map[string]any) { v["suspended_at"] = time.Now() }, true},
		{"issued permissions", "/app/installations/456/access_tokens", func(v map[string]any) { v["permissions"] = map[string]string{"contents": "write"} }, false},
		{"expiry", "/app/installations/456/access_tokens", func(v map[string]any) { v["expires_at"] = time.Now().Add(2 * time.Hour) }, false},
		{"token injection", "/app/installations/456/access_tokens", func(v map[string]any) { v["token"] = "unsafe\ncredential" }, false},
		{"multiple repositories", "/installation/repositories", func(v map[string]any) { v["total_count"] = 2 }, false},
		{"wrong repository", "/installation/repositories", func(v map[string]any) { v["repositories"] = []map[string]string{{"full_name": "different/other"}} }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _, _ := fixture(t)
			mints := 0
			fake(a, func(r *http.Request, v map[string]any) {
				if r.Method == "POST" {
					mints++
				}
				if r.URL.Path == tt.path {
					tt.edit(v)
				}
			})
			_, e := a.Issue(t.Context())
			if e == nil || strings.Contains(e.Error(), "unsafe") {
				t.Fatal("unsafe authority accepted or leaked")
			}
			if tt.beforeMint && mints != 0 {
				t.Fatal("mint occurred before authority validation")
			}
		})
	}
}
func TestQualificationAndAPIWriteBoundary(t *testing.T) {
	a, _, _ := fixture(t)
	calls := 0
	fake(a, func(r *http.Request, _ map[string]any) { calls++ })
	if a.api(t.Context(), "private-token", "POST", "/repos/different/project/issues", nil, new(any)) == nil || calls != 0 {
		t.Fatal("repository write accepted")
	}
	r, e := a.Qualify(t.Context(), 42)
	if e != nil || r["checkRuns"] != 2 || r["commitStatuses"] != 0 {
		t.Fatal("read qualification failed", e)
	}
	encoded, _ := json.Marshal(r)
	if strings.Contains(string(encoded), strings.Repeat("t", 48)) {
		t.Fatal("credential in receipt")
	}
	count := 0
	fake(a, func(r *http.Request, v map[string]any) {
		if r.URL.Path == "/repos/different/project/pulls/42" {
			count++
			if count == 2 {
				v["head"] = map[string]any{"sha": strings.Repeat("b", 40), "repo": map[string]string{"full_name": a.config.Repository}}
			}
		}
	})
	if _, e = a.Qualify(t.Context(), 42); e == nil {
		t.Fatal("moving PR head accepted")
	}
	a.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("private-token")), Header: make(http.Header)}, nil
	})
	e = a.api(t.Context(), "private-token", "GET", "/app", nil, new(any))
	if e == nil || strings.Contains(e.Error(), "private-token") {
		t.Fatal("upstream credential error leaked")
	}
}
func TestRefreshScheduleExpiryRestartAndFileModes(t *testing.T) {
	a, _, now := fixture(t)
	generation := 0
	fake(a, func(r *http.Request, v map[string]any) {
		if r.Method == "POST" {
			generation++
			v["token"] = strings.Repeat(string(rune('a'+generation)), 48)
		}
	})
	if changed, e := a.cycle(t.Context(), true); e != nil || !changed {
		t.Fatal("initial issuance", e)
	}
	old, e := os.Open(filepath.Join(a.config.OutputDir, "token"))
	if e != nil {
		t.Fatal(e)
	}
	defer old.Close()
	if changed, e := a.cycle(t.Context(), false); e != nil || changed || generation != 1 {
		t.Fatal("early renewal", e)
	}
	*now = now.Add(2400 * time.Second)
	if changed, e := a.cycle(t.Context(), false); e != nil || !changed || generation != 2 {
		t.Fatal("normal renewal", e)
	}
	oldBytes, _ := io.ReadAll(old)
	newBytes, _ := os.ReadFile(filepath.Join(a.config.OutputDir, "token"))
	if string(oldBytes) == string(newBytes) {
		t.Fatal("open reader was overwritten")
	}
	if _, e := State(a.config.OutputDir, *now); e != nil {
		t.Fatal("renewed file not private/ready", e)
	}
	if changed, e := a.cycle(t.Context(), true); e != nil || !changed || generation != 3 {
		t.Fatal("restart did not remint", e)
	}
	*now = now.Add(2401 * time.Second)
	a.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return nil, unavailable })
	if _, e := a.cycle(t.Context(), false); e == nil {
		t.Fatal("failure not reported")
	}
	if _, e := State(a.config.OutputDir, *now); e != nil {
		t.Fatal("valid previous token lost")
	}
	*now = now.Add(1140 * time.Second)
	if _, e := a.cycle(t.Context(), false); e == nil {
		t.Fatal("expiry failure not reported")
	}
	if _, e := os.Stat(filepath.Join(a.config.OutputDir, "token")); !os.IsNotExist(e) {
		t.Fatal("expired token retained")
	}
}
func TestRefresherLockAndGracefulStop(t *testing.T) {
	a, _, _ := fixture(t)
	fake(a, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- a.Run(ctx, func(string) {
			select {
			case started <- struct{}{}:
			default:
			}
		})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("refresher not ready")
	}
	if e := a.Run(t.Context(), func(string) {}); e == nil {
		t.Fatal("second refresher acquired directory")
	}
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("refresher did not stop")
	}
}
func TestEnvironmentContract(t *testing.T) {
	base := map[string]string{"UPGRADES_REPOSITORY": "owner/repo", "UPGRADES_APP_OWNER": "owner", "UPGRADES_APP_SLUG": "reader"}
	c, e := Parse(base)
	if e != nil || c.RefreshSeconds != 2400 {
		t.Fatal("default environment rejected", e)
	}
	for key, value := range map[string]string{"UPGRADES_REPOSITORY": "../..", "UPGRADES_APP_OWNER": " ", "UPGRADES_APP_SLUG": "", "UPGRADES_APP_CREDENTIALS_DIR": "relative", "UPGRADES_APP_TOKEN_DIR": "/credentials/app", "UPGRADES_APP_REFRESH_SECONDS": "29"} {
		t.Run(key, func(t *testing.T) {
			v := maps.Clone(base)
			v[key] = value
			if _, e := Parse(v); e == nil {
				t.Fatal("invalid environment accepted")
			}
		})
	}
	delete(base, "UPGRADES_APP_OWNER")
	if _, e := Parse(base); e == nil {
		t.Fatal("missing required environment accepted")
	}
}
