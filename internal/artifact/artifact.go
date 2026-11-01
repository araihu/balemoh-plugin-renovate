// Package artifact publishes one atomic snapshot and serves only authenticated reads.
package artifact

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func Publish(directory string, data []byte) error {
	if _, err := snapshot.Decode(data, time.Now()); err != nil {
		return err
	}
	f, err := os.CreateTemp(directory, ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0640); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), filepath.Join(directory, "snapshot.json"))
}
func Handler(path, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path == "/healthz" && (r.Method == "GET" || r.Method == "HEAD") {
			w.WriteHeader(204)
			return
		}
		if r.URL.Path != "/snapshot.json" || r.URL.RawQuery != "" {
			http.NotFound(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(405)
			return
		}
		expected := sha256.Sum256([]byte("Bearer " + token))
		actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if len(token) < 32 || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			w.WriteHeader(401)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			w.WriteHeader(503)
			return
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, snapshot.MaxBytes+1))
		if err != nil || len(data) > snapshot.MaxBytes {
			w.WriteHeader(503)
			return
		}
		digest := sha256.Sum256(data)
		etag := fmt.Sprintf(`"%x"`, digest)
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(304)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method == "GET" {
			_, _ = w.Write(data)
		}
	})
}
