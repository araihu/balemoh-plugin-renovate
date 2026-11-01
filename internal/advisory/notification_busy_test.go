package advisory

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
)

type busyNotificationPublisher struct {
	calls    int
	messages []notifications.Message
	accept   bool
}

func (p *busyNotificationPublisher) Publish(_ context.Context, m notifications.Message) (notifications.Receipt, error) {
	p.calls++
	p.messages = append(p.messages, m)
	if p.accept {
		return notifications.Receipt{ID: "retained", Created: true}, nil
	}
	return notifications.Receipt{}, notifications.Fail("resource_exhausted")
}

func TestBusyNotificationIsRetriedWithFrozenContent(t *testing.T) {
	now := time.Now().UTC()
	publisher := &busyNotificationPublisher{}
	c := &connection{
		id:        "00000000-0000-4000-8000-000000000002",
		config:    Config{NotificationsEnabled: true, MaxSnapshotAgeSeconds: 43200, IntervalSeconds: 60},
		state:     "fresh",
		success:   &now,
		publisher: publisher,
		seen:      map[string]bool{},
		snapshot:  testSnapshot(now),
	}
	c.admit()

	newCandidate := c.snapshot.Dependencies[0].Candidates[0]
	newCandidate.NewValue = new("3")
	newCandidate.ID = snapshot.ID(c.snapshot.Dependencies[0].ID, newCandidate.NewValue, newCandidate.NewDigest, newCandidate.UpdateType)
	c.snapshot.Dependencies[0].Candidates = append(c.snapshot.Dependencies[0].Candidates, newCandidate)
	c.admit()
	if len(c.pending) != 1 {
		t.Fatalf("repro setup expected one pending notification, got %d", len(c.pending))
	}

	c.deliver(t.Context())
	if publisher.calls != 1 {
		t.Fatalf("expected one publication attempt, got %d", publisher.calls)
	}
	if len(c.pending) != 1 {
		t.Fatalf("retryable resource_exhausted removed pending notification; got %d pending", len(c.pending))
	}
	publisher.accept = true
	c.deliver(t.Context())
	if len(c.pending) != 0 || publisher.calls != 2 || !reflect.DeepEqual(publisher.messages[0], publisher.messages[1]) {
		t.Fatal("busy retry did not preserve frozen message and clear accepted event")
	}
}
