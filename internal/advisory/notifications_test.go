package advisory

import (
	"context"
	"fmt"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
	"reflect"
	"strings"
	"testing"
	"time"
)

type publisher struct {
	messages []notifications.Message
	fail     bool
}

func (p *publisher) Publish(_ context.Context, m notifications.Message) (notifications.Receipt, error) {
	p.messages = append(p.messages, m)
	if p.fail {
		return notifications.Receipt{}, notifications.Fail("deadline_exceeded")
	}
	return notifications.Receipt{ID: "receipt", Created: true}, nil
}
func TestNotificationBaselineGapAndRetry(t *testing.T) {
	p := &publisher{}
	c := &connection{id: "00000000-0000-4000-8000-000000000002", config: Config{NotificationsEnabled: true, MaxSnapshotAgeSeconds: 43200, IntervalSeconds: 60}, state: "fresh", publisher: p, seen: map[string]bool{}, snapshot: testSnapshot(time.Now())}
	now := time.Now()
	c.success = &now
	c.admit()
	c.deliver(context.Background())
	if len(p.messages) != 0 {
		t.Fatal("startup notified")
	}
	u := c.snapshot.Dependencies[0].Candidates[0]
	u.ID = "new-candidate"
	c.snapshot.Dependencies[0].Candidates = append(c.snapshot.Dependencies[0].Candidates, u)
	c.admit()
	p.fail = true
	c.deliver(context.Background())
	p.fail = false
	c.deliver(context.Background())
	if len(p.messages) != 2 || !reflect.DeepEqual(p.messages[0], p.messages[1]) || len(c.pending) != 0 {
		t.Fatal("retry identity/content changed")
	}
	c.state = "stale"
	c.admit()
	c.state = "fresh"
	u.ID = "during-gap"
	c.snapshot.Dependencies[0].Candidates = append(c.snapshot.Dependencies[0].Candidates, u)
	c.admit()
	c.deliver(context.Background())
	if len(p.messages) != 2 {
		t.Fatal("recovery burst")
	}
	c.admit()
	if len(c.pending) != 0 {
		t.Fatal("unchanged poll notified")
	}
}

func TestPositiveNotificationsUnderPartialCoverageAndSecurityEscalation(t *testing.T) {
	now := time.Now()
	p := &publisher{}
	s := testSnapshot(now)
	s.Coverage.Status = "partial"
	s.Coverage.Reasons = []string{"source_problems"}
	c := &connection{id: "00000000-0000-4000-8000-000000000002", config: Config{NotificationsEnabled: true, MaxSnapshotAgeSeconds: 43200, IntervalSeconds: 60}, state: "partial", success: &now, publisher: p, seen: map[string]bool{}, snapshot: s}
	c.admit()
	c.deliver(t.Context())
	if len(p.messages) != 0 || !c.baseline {
		t.Fatal("partial startup did not establish silent baseline")
	}
	u := s.Dependencies[0].Candidates[0]
	u.NewValue = new("3")
	u.ID = snapshot.ID(s.Dependencies[0].ID, u.NewValue, u.NewDigest, u.UpdateType)
	s.Dependencies[0].Candidates = append(s.Dependencies[0].Candidates, u)
	c.config.ServiceMappings = []Mapping{{DependencyID: s.Dependencies[0].ID, ServiceID: strings.Repeat("b", 64)}}
	c.admit()
	c.deliver(t.Context())
	if len(p.messages) != 1 {
		t.Fatal("positive candidate suppressed by unrelated partial coverage")
	}
	regular := p.messages[0]
	if regular.Severity != notifications.SeverityInfo || regular.ServiceID != strings.Repeat("b", 64) || regular.Validate() != nil || !strings.Contains(regular.Body, "partial") || !strings.Contains(regular.Body, "candidate 3") || regular.Destination.Params["dependency-id"] != s.Dependencies[0].ID {
		t.Fatal("invalid ordinary notification", regular)
	}
	// Enrichment of the same candidate must not mutate an existing inbox event.
	s.Dependencies[0].Candidates[1].IsVulnerabilityAlert = true
	c.admit()
	c.deliver(t.Context())
	if len(p.messages) != 2 {
		t.Fatal("new security evidence did not notify")
	}
	security := p.messages[1]
	if !strings.HasPrefix(security.Title, "Renovate security flag:") || !strings.HasPrefix(security.Body, "Renovate flag only; CVE and severity are not established.") {
		t.Fatal("security boundary falls outside the inbox preview")
	}
	if security.Severity != notifications.SeverityWarning || security.EventID == regular.EventID || security.Validate() != nil || !strings.Contains(security.Body, "isVulnerabilityAlert=true") || !strings.Contains(security.Body, "not established") {
		t.Fatal("security classification overstated/missing", security)
	}
	if !reflect.DeepEqual(regular, p.messages[0]) {
		t.Fatal("ordinary notification mutated")
	}
	for range 3 {
		c.admit()
		c.deliver(t.Context())
	}
	s.Dependencies[0].Candidates[1].IsVulnerabilityAlert = false
	c.admit()
	s.Dependencies[0].Candidates[1].IsVulnerabilityAlert = true
	c.admit()
	c.deliver(t.Context())
	if len(p.messages) != 2 {
		t.Fatal("unchanged/reappearing security flag notified again")
	}
}
func TestSecurityNotificationAdmissionBoundsAndRetry(t *testing.T) {
	now := time.Now()
	p := &publisher{}
	c := &connection{id: "00000000-0000-4000-8000-000000000002", config: Config{NotificationsEnabled: true, MaxSnapshotAgeSeconds: 43200, IntervalSeconds: 60}, state: "fresh", success: &now, publisher: p, seen: map[string]bool{}, snapshot: testSnapshot(now)}
	c.admit()
	original := c.snapshot.Dependencies[0].Candidates[0]
	for i := range 65 {
		u := original
		u.NewValue = new(fmt.Sprint(i + 10))
		u.ID = snapshot.ID(c.snapshot.Dependencies[0].ID, u.NewValue, u.NewDigest, u.UpdateType)
		u.IsVulnerabilityAlert = i == 64
		c.snapshot.Dependencies[0].Candidates = append(c.snapshot.Dependencies[0].Candidates, u)
	}
	c.admit()
	if len(c.pending) != 64 || c.pending[0].Severity != notifications.SeverityWarning {
		t.Fatal("security candidate lost behind regular queue")
	}
	p.fail = true
	c.deliver(t.Context())
	frozen := p.messages[0]
	c.snapshot.Dependencies[0].PackageName = "Changed presentation"
	extra := original
	extra.NewValue = new("1000")
	extra.ID = snapshot.ID(c.snapshot.Dependencies[0].ID, extra.NewValue, extra.NewDigest, extra.UpdateType)
	extra.IsVulnerabilityAlert = true
	c.snapshot.Dependencies[0].Candidates = append(c.snapshot.Dependencies[0].Candidates, extra)
	c.admit()
	warnings := 0
	for _, message := range c.pending {
		if message.Severity == notifications.SeverityWarning {
			warnings++
		}
	}
	if len(c.pending) != 64 || warnings != 2 {
		t.Fatal("new security notice failed to displace an ordinary queued notice")
	}

	c.admit()
	p.fail = false
	c.deliver(t.Context())
	if !reflect.DeepEqual(frozen, p.messages[1]) {
		t.Fatal("security retry content changed")
	}
}
func TestNotificationEligibilityAndOptIn(t *testing.T) {
	now := time.Now()
	p := &publisher{}
	c := &connection{id: "00000000-0000-4000-8000-000000000002", config: Config{NotificationsEnabled: true}, state: "fresh", baseline: true, publisher: p, seen: map[string]bool{}, snapshot: testSnapshot(now)}
	for _, assessment := range []string{"unknown", "excluded", "no_candidate"} {
		c.snapshot.Dependencies[0].Assessment = assessment
		c.admit()
	}
	if len(c.pending) != 0 {
		t.Fatal("unassessed/excluded group notified")
	}
	c.snapshot.Dependencies[0].Assessment = "updates_found"
	c.config.NotificationsEnabled = false
	c.admit()
	c.config.NotificationsEnabled = true
	c.publisher = nil
	c.admit()
	if len(c.pending) != 0 {
		t.Fatal("disabled/ungranted notification admitted")
	}
}
