// dev-host runs an isolated real API/BFF with the controlled report fixture.
// It never reads a GitHub credential or contacts a registry.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	apiFlag := flag.String("api", ".local/host/balemoh", "Compatible real API executable")
	uiFlag := flag.String("ui", ".local/host/balemoh-ui", "Compatible real BFF executable")
	listen := flag.String("listen", "127.0.0.1:14080", "Local BFF address")
	reportFlag := flag.String("report", "internal/snapshot/testdata/report.json", "Controlled report fixture")
	diagnosticsFlag := flag.String("diagnostics", "", "Optional controlled native diagnostic fixture")
	notify := flag.Bool("notifications", false, "Demonstrate one synthetic security candidate notification in the isolated host")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	root, err := os.MkdirTemp(".local", "dev-host-*")
	if err != nil {
		return err
	}
	root, _ = filepath.Abs(root)
	write := func(name string, data []byte) string {
		p := filepath.Join(root, name)
		if os.WriteFile(p, data, 0600) != nil {
			panic("cannot write development fixture")
		}
		return p
	}
	secret := func() string { var b [32]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
	sourceToken, serviceToken, operatorToken := secret(), secret(), secret()
	data, err := os.ReadFile(*reportFlag)
	if err != nil {
		return err
	}
	if *notify {
		// Synthetic explicit update flag, never a real CVE claim.
		var raw map[string]any
		if json.Unmarshal(data, &raw) != nil {
			return errors.New("invalid controlled report")
		}
		repo := raw["repositories"].(map[string]any)["example/read-only"].(map[string]any)
		file := repo["packageFiles"].(map[string]any)["docker-compose"].([]any)[0].(map[string]any)
		dep := file["deps"].([]any)[0].(map[string]any)
		update := dep["updates"].([]any)[0].(map[string]any)
		update["isVulnerabilityAlert"] = true
		data, _ = json.Marshal(raw)
	}
	now := time.Now().UTC()
	commit := strings.Repeat("a", 40)
	m := snapshot.Metadata{RunID: "00000000-0000-4000-8000-000000000001", StartedAt: now.Add(-time.Minute), CompletedAt: now, Producer: snapshot.Producer{RenovateVersion: snapshot.RenovateVersion, RenovateImage: snapshot.RenovateImage, AdapterVersion: snapshot.AdapterVersion, PolicyHash: snapshot.ID("controlled-fixture")}, Repository: snapshot.Repository{Platform: "github", Name: "example/read-only", BaseBranch: "main", Commit: commit}, DryRun: "full", BeforeCommit: commit, AfterCommit: commit, CheckoutCommit: commit}
	var diagnosticReaders []io.Reader
	if *diagnosticsFlag != "" {
		f, e := os.Open(*diagnosticsFlag)
		if e != nil {
			return e
		}
		defer f.Close()
		diagnosticReaders = append(diagnosticReaders, f)
	}
	s, err := snapshot.Normalize(data, m, diagnosticReaders...)
	if err != nil {
		return err
	}
	s.Coverage.Scope = "Controlled fixture, simulated execution envelope. Not production inventory."
	// This ID was returned by the public API for the fixed manual fixture request.
	// Verify it again below. No production service ID algorithm is copied here.
	const fixtureServiceID = "25f23deeabd0b9093d18123ecbb7cfaeea0c174fe5e33297a422b02ee65111d0"
	var dependencyID string
	for _, d := range s.Dependencies {
		if d.Manager == "docker-compose" && d.PackageFile == "compose.yaml" && d.PackageName == "docker.io/library/nginx" {
			dependencyID = d.ID
		}
	}
	if dependencyID == "" {
		return errors.New("fixture dependency identity changed")
	}
	mappings, _ := json.Marshal([]advisory.Mapping{{DependencyID: dependencyID, ServiceID: fixtureServiceID}})
	var current atomic.Pointer[snapshot.Snapshot]
	initial := s
	if *notify {
		bytes, _ := json.Marshal(s)
		initial = new(snapshot.Snapshot)
		_ = json.Unmarshal(bytes, initial)
		for i, d := range initial.Dependencies {
			if d.ID == dependencyID {
				initial.Dependencies[i].Candidates = nil
				for _, u := range d.Candidates {
					if !u.IsVulnerabilityAlert {
						initial.Dependencies[i].Candidates = append(initial.Dependencies[i].Candidates, u)
					}
				}
			}
		}
	}
	current.Store(initial)
	var calls atomic.Int32
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+sourceToken {
			w.WriteHeader(401)
			return
		}
		_ = json.NewEncoder(w).Encode(current.Load())
	}))
	defer source.Close()
	ca := []byte(base64.StdEncoding.EncodeToString(source.Certificate().Raw))
	caFile := write("source-ca", ca)
	sourceFile := write("source-token", []byte(sourceToken))
	serviceFile := write("service-token", []byte(serviceToken))
	operatorFile := write("notification-operator-token", []byte(operatorToken))
	interval := 900
	capabilityGrants := []string{}
	if *notify {
		interval = 30
		capabilityGrants = append(capabilityGrants, "notifications.publish/v1")
	}
	pkg, err := manifest.Load("dist/package")
	if err != nil {
		return err
	}
	packagePath, _ := filepath.Abs("dist/package")
	digest, err := pkg.ContentDigest()
	if err != nil {
		return err
	}
	installation := map[string]any{"package": packagePath, "manifestDigest": pkg.Digest, "packageDigest": digest, "connections": []any{map[string]any{"key": "fixture", "name": "Renovate controlled fixture", "config": map[string]any{"snapshotURL": source.URL, "browserRepositoryURL": "https://github.com/example/read-only", "intervalSeconds": interval, "notificationsEnabled": *notify, "serviceMappingsJSON": string(mappings)}, "secrets": map[string]string{"snapshot-token": sourceFile, "snapshot-ca": caFile}, "grants": []string{"upgrades.list", "upgrades.get", "upgrades.summary"}, "capabilityGrants": capabilityGrants}}}
	apiConfig, _ := json.Marshal(map[string]any{"installations": []any{installation}})
	uiConfig, _ := json.Marshal(map[string]any{"installations": []any{map[string]any{"package": packagePath, "manifestDigest": pkg.Digest, "packageDigest": digest, "connectionKey": "fixture", "serviceIDs": []string{fixtureServiceID}, "previewWidgets": []any{map[string]any{"id": "00000000-0000-4000-8000-000000000010", "definition": "summary"}}}}})
	apiConfigFile := write("api.json", apiConfig)
	uiConfigFile := write("ui.json", uiConfig)
	start := func(binary string, environment []string) error {
		binary, _ = filepath.Abs(binary)
		log, err := os.OpenFile(filepath.Join(root, filepath.Base(binary)+".log"), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		command := exec.CommandContext(ctx, binary)
		command.Dir = root
		command.Env = append([]string{"HOME=" + root, "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}, environment...)
		command.Stdout = log
		command.Stderr = log
		if err = command.Start(); err != nil {
			log.Close()
			return err
		}
		go func() { _ = command.Wait(); _ = log.Close() }()
		return nil
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	apiAddress := l.Addr().String()
	_ = l.Close()
	apiURL := "http://" + apiAddress
	uiURL := "http://" + *listen
	if err = start(*apiFlag, []string{"BALEMOH_HTTP_ADDR=" + apiAddress, "BALEMOH_DATABASE_PATH=" + filepath.Join(root, "db.sqlite"), "BALEMOH_PLUGIN_CONFIG=" + apiConfigFile, "BALEMOH_PLUGIN_SERVICE_TOKEN_FILE=" + serviceFile, "BALEMOH_NOTIFICATION_OPERATOR_TOKEN_FILE=" + operatorFile}); err != nil {
		return err
	}
	client := &http.Client{Timeout: 4 * time.Second}
	get := func(base, path, token string) (int, []byte) {
		request, _ := http.NewRequest("GET", base+path, nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response, e := client.Do(request)
		if e != nil {
			return 0, nil
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		return response.StatusCode, body
	}
	wait := func(check func() bool) bool {
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				return true
			}
			select {
			case <-ctx.Done():
				return false
			case <-time.After(100 * time.Millisecond):
			}
		}
		return false
	}
	if !wait(func() bool { code, _ := get(apiURL, "/healthz", ""); return code == 200 }) {
		return errors.New("fixture API failed to start; inspect private development log")
	}
	var catalog struct {
		Plugins []struct {
			Connections []struct {
				ID          string     `json:"id"`
				LastSuccess *time.Time `json:"lastSuccessAt"`
			} `json:"connections"`
		} `json:"plugins"`
	}
	if !wait(func() bool {
		code, body := get(apiURL, "/api/v1/plugins", serviceToken)
		if code != 200 || json.Unmarshal(body, &catalog) != nil || len(catalog.Plugins) != 1 || len(catalog.Plugins[0].Connections) != 1 {
			return false
		}
		return catalog.Plugins[0].Connections[0].LastSuccess != nil
	}) {
		_, statusBody := get(apiURL, "/api/v1/plugins", serviceToken)
		return fmt.Errorf("fixture plugin failed to collect: %s", statusBody)
	}
	id := catalog.Plugins[0].Connections[0].ID
	// Create only in this process's isolated SQLite database, through the real API.
	serviceBody, _ := json.Marshal(map[string]string{"requestId": "00000000000040008000000000000011", "displayName": "Edge proxy", "description": "Controlled fixture service. Explicit image/file mapping.", "address": "https://edge.example.test", "icon": "goshtoso:ui-hi-16-solid-squares-2x2"})
	serviceRequest, _ := http.NewRequest("POST", apiURL+"/api/v1/staging/services", strings.NewReader(string(serviceBody)))
	serviceRequest.Header.Set("Authorization", "Bearer "+serviceToken)
	serviceRequest.Header.Set("Content-Type", "application/json")
	serviceResponse, e := client.Do(serviceRequest)
	if e != nil {
		return e
	}
	var created struct {
		ID string `json:"id"`
	}
	decodeErr := json.NewDecoder(serviceResponse.Body).Decode(&created)
	_ = serviceResponse.Body.Close()
	if serviceResponse.StatusCode != 201 || decodeErr != nil || created.ID != fixtureServiceID {
		return errors.New("fixture manual service identity mismatch")
	}
	placement, _ := json.Marshal(map[string]any{"pluginId": "araihu.image-upgrades", "connectionId": id, "contributionId": "summary", "options": map[string]any{"updateType": "pinDigest"}, "enabled": true})
	request, _ := http.NewRequest("PUT", apiURL+"/api/v1/plugin-widgets/00000000-0000-4000-8000-000000000010", strings.NewReader(string(placement)))
	request.Header.Set("Authorization", "Bearer "+serviceToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	_ = response.Body.Close()
	if response.StatusCode != 201 {
		return errors.New("fixture widget setup failed")
	}
	if err = start(*uiFlag, []string{"BALEMOH_UI_HTTP_ADDR=" + *listen, "BALEMOH_UI_API_BASE_URL=" + apiURL, "BALEMOH_UI_PLUGIN_CONFIG=" + uiConfigFile, "BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE=" + serviceFile, "BALEMOH_UI_ICON_CACHE_DIR=" + filepath.Join(root, "icons"), "BALEMOH_UI_ICON_STARTUP_TIMEOUT=10ms", "BALEMOH_UI_NOTIFICATION_OPERATOR_TOKEN_FILE=" + operatorFile}); err != nil {
		return err
	}
	page := "/plugins/araihu.image-upgrades/connections/" + id + "/pages/current"
	content := strings.Replace(page, "/pages/", "/page-content/", 1)
	if !wait(func() bool {
		code, body := get(uiURL, content, "")
		return code == 200 && strings.Contains(string(body), "nginx")
	}) {
		return errors.New("real BFF did not render fixture plugin")
	}
	before := calls.Load()
	for _, path := range []string{content, content + "?search=nginx", strings.Replace(content, "/current", "/coverage", 1), "/plugin-widgets/00000000-0000-4000-8000-000000000010", content + "/" + dependencyID} {
		code, body := get(uiURL, path, "")
		if (strings.HasPrefix(path, "/plugin-widgets/") || path == content+"/"+dependencyID) && !strings.Contains(string(body), "Edge proxy") {
			return errors.New("mapped service missing from widget/detail")
		}
		if code != 200 || strings.Contains(string(body), sourceToken) || strings.Contains(string(body), serviceToken) || strings.Contains(string(body), source.URL) || strings.Contains(string(body), operatorToken) {
			return errors.New("real host read-only boundary failed")
		}
	}
	// Each responsive composition contains only the explicitly mapped group.
	// CSS exposes exactly one composition at a time.
	_, mappedBody := get(uiURL, content+"?search=nginx", "")
	if strings.Count(string(mappedBody), "Edge proxy") != 2 || !strings.Contains(string(mappedBody), "squares-2x2") {
		write("mapped-page.html", mappedBody)
		return errors.New("exact service catalog presentation failed; inspect private mapped-page.html")
	}
	if before != calls.Load() {
		return errors.New("browser reads triggered source collection")
	}
	notified := false
	if *notify {
		var inbox struct {
			Notifications []struct{ EventID, Severity, ServiceID, Title string } `json:"notifications"`
		}
		code, body := get(apiURL, "/api/v1/notifications", operatorToken)
		if code != 200 || json.Unmarshal(body, &inbox) != nil || len(inbox.Notifications) != 0 {
			return errors.New("notification baseline was not silent")
		}
		// The next normal asynchronous poll discovers the previously absent candidate.
		s.RunID = "00000000-0000-4000-8000-000000000002"
		s.CompletedAt = time.Now().UTC()
		s.StartedAt = s.CompletedAt.Add(-time.Minute)
		current.Store(s)
		fmt.Println("Waiting for the normal collector poll to publish the synthetic security notification.")
		deadline := time.Now().Add(55 * time.Second)
		for time.Now().Before(deadline) {
			code, body = get(apiURL, "/api/v1/notifications", operatorToken)
			if code == 200 && json.Unmarshal(body, &inbox) == nil && len(inbox.Notifications) == 1 {
				message := inbox.Notifications[0]
				if message.Severity != "warning" || message.ServiceID != fixtureServiceID || !strings.HasPrefix(message.EventID, "image-upgrade-security:") {
					return errors.New("unexpected notification attribution")
				}
				notified = true
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
		if !notified {
			return errors.New("security notification was not received by the real host")
		}
	}
	receipt, _ := json.MarshalIndent(map[string]any{"fixture": true, "apiExecutable": *apiFlag, "uiExecutable": *uiFlag, "apiURL": apiURL, "sdk": "v0.0.1-alpha.4", "page": uiURL + page, "sourceCalls": calls.Load(), "queryReadsAddSourceCalls": false, "pageCoverageWidget": true, "mappedServicePresentation": true, "syntheticSecurityNotification": notified, "serviceID": fixtureServiceID, "mappedDependencyID": dependencyID, "manifestDigest": pkg.Digest, "packageDigest": digest}, "", "  ")
	write("receipt.json", receipt)
	fmt.Printf("Fixture preview: %s%s\nEvidence: %s/receipt.json\n", uiURL, page, root)
	<-ctx.Done()
	return nil
}
