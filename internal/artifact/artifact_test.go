package artifact

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyTokenAndETag(t *testing.T) {
	file := filepath.Join(t.TempDir(), "snapshot.json")
	_ = os.WriteFile(file, []byte(`{"snapshot":"example"}`), 0600)
	token := strings.Repeat("x", 40)
	handler := Handler(file, token)
	call := func(method, auth, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://source/snapshot.json", nil)
		r.Header.Set("Authorization", auth)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if call("GET", "", "").Code != 401 || call("PUT", "Bearer "+token, "").Code != 405 {
		t.Fatal("write/auth boundary")
	}
	first := call("GET", "Bearer "+token, "")
	if first.Code != 200 || first.Header().Get("ETag") == "" || call("GET", "Bearer "+token, first.Header().Get("ETag")).Code != http.StatusNotModified {
		t.Fatal("etag")
	}
	before, _ := os.ReadFile(file)
	if Publish(filepath.Dir(file), []byte(`{broken`)) == nil {
		t.Fatal("invalid artifact published")
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Fatal("last artifact lost")
	}
}
