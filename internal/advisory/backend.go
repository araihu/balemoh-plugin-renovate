package advisory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
)

type Backend struct {
	ctx         context.Context
	mu          sync.RWMutex
	connections map[string]*connection
}
type connection struct {
	mu               sync.Mutex
	id, revision     string
	fingerprint      [32]byte
	config           Config
	client           *http.Client
	token            string
	cancel           context.CancelFunc
	done             chan struct{}
	snapshot         *snapshot.Snapshot
	runs             map[string][32]byte
	digest           [32]byte
	etag             string
	observation      uint64
	state            string
	attempt, success *time.Time
	sourceError      *backend.Error
	publisher        notifications.Publisher
	baseline         bool
	seen             map[string]bool
	securitySeen     map[string]bool
	pending          []notifications.Message
}

func New(ctx context.Context) *Backend {
	return &Backend{ctx: ctx, connections: map[string]*connection{}}
}
func (b *Backend) Configure(ctx context.Context, r backend.Config) (backend.Status, error) {
	if ctx.Err() != nil {
		return backend.Status{}, ctx.Err()
	}
	if r.ConnectionID == "" || !backend.Revision(r.Revision) {
		return backend.Status{}, backend.Fail("invalid_argument")
	}
	cfg, client, err := parseConfig(r.Config, r.Secrets)
	if err != nil {
		return backend.Status{}, err
	}
	canonical, _ := json.Marshal(struct {
		Config  Config
		Secrets map[string]string
	}{cfg, r.Secrets})
	fingerprint := sha256.Sum256(canonical)
	adopted := false
	defer func() {
		if !adopted {
			client.CloseIdleConnections()
		}
	}()
	b.mu.Lock()
	defer b.mu.Unlock()
	old := b.connections[r.ConnectionID]
	if old != nil {
		next, _ := strconv.ParseUint(r.Revision, 10, 64)
		prev, _ := strconv.ParseUint(old.revision, 10, 64)
		if next < prev || next == prev && old.fingerprint != fingerprint {
			return backend.Status{}, backend.Fail("conflict")
		}
		if next == prev {
			return old.status(), nil
		}
	}
	var seed [8]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return backend.Status{}, backend.Fail("unavailable")
	}
	lifetime, cancel := context.WithCancel(b.ctx)
	c := &connection{id: r.ConnectionID, revision: r.Revision, config: cfg, fingerprint: fingerprint, client: client, token: r.Secrets["snapshot-token"], cancel: cancel, done: make(chan struct{}), state: "unavailable", observation: (binary.BigEndian.Uint64(seed[:]) & ((1 << 62) - 1)) + 1, publisher: r.Notifications, seen: map[string]bool{}}
	if ctx.Err() != nil {
		cancel()
		return backend.Status{}, ctx.Err()
	}
	b.connections[r.ConnectionID] = c
	adopted = true
	if old != nil {
		old.cancel()
	}
	go func() {
		if old != nil {
			<-old.done
		}
		c.collect(lifetime)
	}()
	return c.status(), nil
}
func (b *Backend) get(id string) (*connection, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	c := b.connections[id]
	if c == nil {
		return nil, backend.Fail("not_found")
	}
	return c, nil
}
func (b *Backend) Status(ctx context.Context, id string) (backend.Status, error) {
	if ctx.Err() != nil {
		return backend.Status{}, ctx.Err()
	}
	c, e := b.get(id)
	if e != nil {
		return backend.Status{}, e
	}
	return c.status(), nil
}
func (b *Backend) CloseConnection(ctx context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.connections[id]
	if c == nil {
		return nil
	}
	c.cancel()
	select {
	case <-c.done:
		delete(b.connections, id)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *connection) stateAt(now time.Time) string {
	if c.snapshot == nil {
		return "unavailable"
	}
	if c.sourceError != nil || now.Sub(c.snapshot.CompletedAt) > time.Duration(c.config.MaxSnapshotAgeSeconds)*time.Second || c.success == nil || now.Sub(*c.success) > time.Duration(2*c.config.IntervalSeconds+15)*time.Second {
		return "stale"
	}
	if c.snapshot.Coverage.Status == "partial" {
		return "partial"
	}
	return "fresh"
}
func (c *connection) refresh(now time.Time) {
	state := c.stateAt(now)
	if state != c.state {
		c.observation++
		c.state = state
		if !c.notificationObservation() {
			c.baseline = false
		}
	}
}
func (c *connection) status() backend.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refresh(time.Now())
	var observed *time.Time
	if c.snapshot != nil {
		t := c.snapshot.CompletedAt
		observed = &t
	}
	return backend.Status{ConnectionID: c.id, AppliedRevision: c.revision, State: c.state, ObservationRevision: strconv.FormatUint(c.observation, 10), ObservedAt: observed, LastAttemptAt: c.attempt, LastSuccessAt: c.success, SourceError: c.sourceError}
}
func (c *connection) collect(ctx context.Context) {
	defer close(c.done)
	defer c.client.CloseIdleConnections()
	failures := 0
	for {
		if c.observe(ctx) {
			failures = 0
		} else {
			failures = min(failures+1, 4)
		}
		if ctx.Err() != nil {
			return
		}
		c.deliver(ctx)
		var jitter [1]byte
		_, _ = rand.Read(jitter[:])
		delay := time.Duration(c.config.IntervalSeconds) * time.Second * time.Duration(1<<failures)
		delay = min(delay, 15*time.Minute)
		delay += time.Duration(jitter[0]) * delay / 2560
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
func (c *connection) observe(ctx context.Context) bool {
	request, err := http.NewRequestWithContext(ctx, "GET", c.config.SnapshotURL, nil)
	if err != nil {
		return false
	}
	c.mu.Lock()
	at := time.Now().UTC()
	c.attempt = &at
	etag := c.etag
	c.mu.Unlock()
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	response, err := c.client.Do(request)
	var next *snapshot.Snapshot
	var digest [32]byte
	var responseETag string
	unchanged := false
	if err == nil {
		defer response.Body.Close()
		if response.StatusCode == 304 {
			unchanged = true
		} else if response.StatusCode != 200 {
			err = backend.Fail("collection_failed")
		} else {
			var data []byte
			data, err = io.ReadAll(io.LimitReader(response.Body, snapshot.MaxBytes+1))
			if err == nil {
				next, err = snapshot.Decode(data, time.Now())
				canonical, _ := json.Marshal(next)
				digest = sha256.Sum256(canonical)
				responseETag = response.Header.Get("ETag")
				if len(responseETag) > 512 {
					responseETag = ""
				}
			}
		}
	}
	if ctx.Err() != nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil && unchanged && c.snapshot == nil {
		err = backend.Fail("invalid_response")
	}
	if err == nil && next != nil {
		if c.runs == nil {
			c.runs = map[string][32]byte{}
		}
		if previous, ok := c.runs[next.RunID]; ok && previous != digest || !ok && len(c.runs) >= 10000 {
			err = backend.Fail("invalid_response")
		}
		if "https://github.com/"+next.Repository.Name != c.config.BrowserRepositoryURL {
			err = backend.Fail("invalid_response")
		}
		if previous := c.snapshot; previous != nil && (next.CompletedAt.Before(previous.CompletedAt) || next.RunID == previous.RunID && digest != c.digest || next.CompletedAt.Equal(previous.CompletedAt) && next.RunID != previous.RunID) {
			err = backend.Fail("invalid_response")
		}
	}
	before := c.state
	oldError := c.sourceError
	oldDigest := c.digest
	if err != nil {
		c.sourceError = backend.Fail("collection_failed")
		c.baseline = false
	} else {
		if next != nil {
			c.snapshot = next
			c.runs[next.RunID] = digest
			c.digest = digest
			c.etag = responseETag
		}
		now := time.Now().UTC()
		c.success = &now
		c.sourceError = nil
	}
	c.state = c.stateAt(time.Now())
	if c.state != before || c.digest != oldDigest || (oldError == nil) != (c.sourceError == nil) {
		c.observation++
	}
	if err == nil {
		c.admit()
	}
	return err == nil
}
