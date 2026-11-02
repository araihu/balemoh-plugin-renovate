package tracking

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var permissions = map[string]string{"checks": "read", "contents": "read", "metadata": "read", "pull_requests": "read", "statuses": "read"}
var numeric = regexp.MustCompile(`^[0-9]+$`)
var bearer = regexp.MustCompile(`^[A-Za-z0-9._~+/-]+=*$`)
var mintPath = regexp.MustCompile(`^/app/installations/[0-9]+/access_tokens$`)
var unavailable = errors.New("App authentication unavailable")

type Issued struct {
	Token       string            `json:"token"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Permissions map[string]string `json:"permissions"`
}
type App struct {
	config Config
	client *http.Client
	now    func() time.Time
}

func New(c Config) *App {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &App{config: c, now: time.Now, client: &http.Client{Transport: transport, Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func boundedFile(path string, max int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, unavailable
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, max+1))
	if e != nil || int64(len(b)) > max {
		return nil, unavailable
	}
	return b, nil
}
func (a *App) jwt() (string, string, error) {
	id, e := boundedFile(filepath.Join(a.config.CredentialsDir, "app-id"), 64)
	if e != nil {
		return "", "", e
	}
	appID := strings.TrimSpace(string(id))
	if !numeric.MatchString(appID) {
		return "", "", unavailable
	}
	data, e := boundedFile(filepath.Join(a.config.CredentialsDir, "private-key.pem"), 64<<10)
	if e != nil {
		return "", "", e
	}
	block, rest := pem.Decode(data)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return "", "", unavailable
	}
	var key *rsa.PrivateKey
	if block.Type == "RSA PRIVATE KEY" {
		key, e = x509.ParsePKCS1PrivateKey(block.Bytes)
	} else if block.Type == "PRIVATE KEY" {
		var v any
		v, e = x509.ParsePKCS8PrivateKey(block.Bytes)
		key, _ = v.(*rsa.PrivateKey)
	} else {
		return "", "", unavailable
	}
	if e != nil || key == nil || key.N.BitLen() < 2048 || key.Validate() != nil {
		return "", "", unavailable
	}
	now := a.now().Unix()
	payload, _ := json.Marshal(map[string]any{"iat": now - 60, "exp": now + 540, "iss": appID})
	encoded := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString(payload)
	sum := sha256.Sum256([]byte(encoded))
	signed, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if e != nil {
		return "", "", unavailable
	}
	return appID, encoded + "." + base64.RawURLEncoding.EncodeToString(signed), nil
}
func (a *App) api(ctx context.Context, token, method, path string, body, out any) error {
	if method != http.MethodGet && !(method == http.MethodPost && mintPath.MatchString(path)) || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\r\n#") {
		return unavailable
	}
	var data []byte
	if body != nil {
		var e error
		data, e = json.Marshal(body)
		if e != nil {
			return unavailable
		}
	}
	r, e := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, bytes.NewReader(data))
	if e != nil {
		return unavailable
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Accept", "application/vnd.github+json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, e := a.client.Do(r)
	if e != nil {
		return unavailable
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return unavailable
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if e != nil || len(raw) > 2<<20 {
		return unavailable
	}
	if json.Unmarshal(raw, out) != nil {
		return unavailable
	}
	return nil
}
func (a *App) Issue(ctx context.Context) (Issued, error) {
	id, jwt, e := a.jwt()
	if e != nil {
		return Issued{}, e
	}
	raw, e := boundedFile(filepath.Join(a.config.CredentialsDir, "installation-id"), 64)
	if e != nil {
		return Issued{}, e
	}
	installationID := strings.TrimSpace(string(raw))
	if !numeric.MatchString(installationID) {
		return Issued{}, unavailable
	}
	var app struct {
		ID    int64  `json:"id"`
		Slug  string `json:"slug"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Permissions map[string]string `json:"permissions"`
		Events      []string          `json:"events"`
	}
	if a.api(ctx, jwt, "GET", "/app", nil, &app) != nil || strconv.FormatInt(app.ID, 10) != id || app.Slug != a.config.AppSlug || app.Owner.Login != a.config.AppOwner || !maps.Equal(app.Permissions, permissions) || len(app.Events) != 0 {
		return Issued{}, unavailable
	}
	parts := strings.SplitN(a.config.Repository, "/", 2)
	var installation struct {
		ID      int64 `json:"id"`
		AppID   int64 `json:"app_id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
		Selection   string            `json:"repository_selection"`
		Permissions map[string]string `json:"permissions"`
		SuspendedAt *time.Time        `json:"suspended_at"`
	}
	if a.api(ctx, jwt, "GET", "/repos/"+a.config.Repository+"/installation", nil, &installation) != nil || strconv.FormatInt(installation.ID, 10) != installationID || installation.AppID != app.ID || installation.Account.Login != parts[0] || installation.Selection != "selected" || !maps.Equal(installation.Permissions, permissions) || installation.SuspendedAt != nil {
		return Issued{}, unavailable
	}
	var issued Issued
	if a.api(ctx, jwt, "POST", "/app/installations/"+installationID+"/access_tokens", map[string]any{"repositories": []string{parts[1]}, "permissions": permissions}, &issued) != nil || !maps.Equal(issued.Permissions, permissions) || issued.ExpiresAt.Sub(a.now()) <= 10*time.Minute || issued.ExpiresAt.Sub(a.now()) > 3700*time.Second || len(issued.Token) > 4096 || len(issued.Token) < 32 || !bearer.MatchString(issued.Token) {
		return Issued{}, unavailable
	}
	var repositories struct {
		Total        int `json:"total_count"`
		Repositories []struct {
			FullName string `json:"full_name"`
		} `json:"repositories"`
	}
	if a.api(ctx, issued.Token, "GET", "/installation/repositories?per_page=100", nil, &repositories) != nil || repositories.Total != 1 || len(repositories.Repositories) != 1 || repositories.Repositories[0].FullName != a.config.Repository {
		return Issued{}, unavailable
	}
	return issued, nil
}

// Qualify proves read access against one unchanged PR head without writing repository resources.
func (a *App) Qualify(ctx context.Context, number int) (map[string]any, error) {
	if number < 1 {
		return nil, unavailable
	}
	issued, e := a.Issue(ctx)
	if e != nil {
		return nil, e
	}
	prefix := "/repos/" + a.config.Repository
	path := prefix + "/pulls/" + strconv.Itoa(number)
	var pull struct {
		Head struct {
			SHA  string `json:"sha"`
			Repo struct {
				FullName string `json:"full_name"`
			} `json:"repo"`
		} `json:"head"`
	}
	if a.api(ctx, issued.Token, "GET", path, nil, &pull) != nil || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(pull.Head.SHA) || pull.Head.Repo.FullName != a.config.Repository {
		return nil, unavailable
	}
	var checks struct {
		Total int `json:"total_count"`
	}
	var statuses struct {
		Total int `json:"total_count"`
	}
	if a.api(ctx, issued.Token, "GET", prefix+"/commits/"+pull.Head.SHA+"/check-runs?per_page=100", nil, &checks) != nil || a.api(ctx, issued.Token, "GET", prefix+"/commits/"+pull.Head.SHA+"/status?per_page=100", nil, &statuses) != nil {
		return nil, unavailable
	}
	for _, p := range []string{prefix + "/git/ref/heads/main", prefix + "/contents/renovate.json?ref=" + pull.Head.SHA, path + "/files?per_page=100"} {
		var v any
		if a.api(ctx, issued.Token, "GET", p, nil, &v) != nil {
			return nil, unavailable
		}
	}
	head := pull.Head.SHA
	if a.api(ctx, issued.Token, "GET", path, nil, &pull) != nil || pull.Head.SHA != head {
		return nil, unavailable
	}
	return map[string]any{"qualifiedAt": a.now().UTC(), "repository": a.config.Repository, "headSHA": head, "pullRequest": number, "checksHTTPStatus": 200, "checkRuns": checks.Total, "commitStatusHTTPStatus": 200, "commitStatuses": statuses.Total, "repositoryMutations": 0, "appSlug": a.config.AppSlug}, nil
}
