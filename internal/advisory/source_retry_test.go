package advisory

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestRetrySurvivesSourceFailure(t *testing.T) {
	var encoded []byte
	failed := true
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if failed {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			_, _ = w.Write(encoded)
		}
	}))
	defer source.Close()
	now := time.Now().UTC()
	s := testSnapshot(now)
	c := testConnection(source)
	p := &publisher{fail: true}
	c.config.NotificationsEnabled = true
	c.publisher = p
	c.snapshot = s
	c.state = "fresh"
	c.success = &now
	c.seen = map[string]bool{}
	c.admit()
	newCandidate := s.Dependencies[0].Candidates[0]
	newCandidate.NewValue = new(fmt.Sprint(3))
	newCandidate.ID = snapshot.ID(s.Dependencies[0].ID, newCandidate.NewValue, newCandidate.NewDigest, newCandidate.UpdateType)
	s.Dependencies[0].Candidates = append(s.Dependencies[0].Candidates, newCandidate)
	c.admit()
	c.deliver(t.Context())
	if len(c.pending) != 1 {
		t.Fatalf("retryable publish did not leave one queued notice: %d", len(c.pending))
	}
	encoded, _ = json.Marshal(s)
	c.digest = sha256.Sum256(encoded)
	if c.observe(t.Context()) {
		t.Fatal("source failure accepted")
	}
	if len(c.pending) != 1 {
		t.Fatalf("retry queue lost after unrelated source failure: %d pending", len(c.pending))
	}
	c.refresh(now.Add(24 * time.Hour))
	c.admit()
	if len(c.pending) != 1 {
		t.Fatal("stale transition discarded admitted notice")
	}
	failed = false
	if !c.observe(t.Context()) {
		t.Fatal("recovery failed")
	}
	p.fail = false
	c.deliver(t.Context())
	if len(c.pending) != 0 || len(p.messages) != 2 || !reflect.DeepEqual(p.messages[0], p.messages[1]) {
		t.Fatal("recovery did not retry frozen content")
	}
}
