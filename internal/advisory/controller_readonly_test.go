package advisory

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/araihu/balemoh/plugin-sdk/backend"
)

func TestControllerReadOnlyCredentialTracksWithoutAllowingActions(t *testing.T) {
	reader := strings.Repeat("r", 40)
	var reads, writes atomic.Int32
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes.Add(1)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path != "/v1/requests" || r.Header.Get("Authorization") != "Bearer "+reader {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		reads.Add(1)
		_, _ = w.Write([]byte(`{"enabled":true,"requests":[]}`))
	}))
	defer source.Close()
	data, err := json.Marshal(Config{SnapshotURL: source.URL + "/snapshot.json", ControllerURL: source.URL, BrowserRepositoryURL: "https://github.com/example/read-only"})
	if err != nil {
		t.Fatal(err)
	}
	secrets := map[string]string{"controller-read-token": reader, "controller-ca": base64.StdEncoding.EncodeToString(source.Certificate().Raw)}
	cfg, client, err := parseConfig(data, secrets)
	if err != nil {
		t.Fatal("read-only controller configuration rejected", err)
	}
	c := &connection{id: "read-only", config: cfg, client: client, controllerReadToken: reader}
	c.observeRequests(t.Context())
	if !c.controllerAvailable || reads.Load() != 1 {
		t.Fatal("read-only tracking unavailable")
	}
	b := New(t.Context())
	b.connections[c.id] = c
	if _, err := b.Action(t.Context(), backend.Request{ConnectionID: c.id, OperationID: "upgrades.request", Input: json.RawMessage(`{}`)}); backend.Code(err) != "permission_denied" || writes.Load() != 0 {
		t.Fatal("missing action credential allowed a write", err)
	}
	secrets["controller-action-token"] = reader
	if _, _, err := parseConfig(data, secrets); backend.Code(err) != "invalid_argument" {
		t.Fatal("shared read/write credential accepted", err)
	}
	secrets["controller-action-token"] = strings.Repeat("w", 40)
	if _, _, err := parseConfig(data, secrets); err != nil {
		t.Fatal("distinct action credential rejected", err)
	}
	delete(secrets, "controller-read-token")
	if _, _, err := parseConfig(data, secrets); backend.Code(err) != "invalid_argument" {
		t.Fatal("missing read credential accepted", err)
	}
}
