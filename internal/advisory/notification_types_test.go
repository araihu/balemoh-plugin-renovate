package advisory

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
)

func TestNotificationTypeSelectionAndSecurityBypass(t *testing.T) {
	for _, tc := range []struct {
		name    string
		types   []string
		enabled bool
		want    map[string]bool
	}{
		{"defaults", nil, true, map[string]bool{"major": true, "minor": true, "patch": true}},
		{"digest-opt-in", []string{"digest"}, true, map[string]bool{"digest": true}},
		{"pin-opt-in", []string{"pin", "pinDigest"}, true, map[string]bool{"pin": true, "pinDigest": true}},
		{"security-only", []string{}, true, map[string]bool{}},
		{"disabled", nil, false, map[string]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			p := &publisher{}
			c := &connection{id: "00000000-0000-4000-8000-000000000002", config: Config{NotificationsEnabled: tc.enabled, NotificationUpdateTypes: tc.types, MaxSnapshotAgeSeconds: 43200, IntervalSeconds: 60}, state: "fresh", success: &now, publisher: p, snapshot: testSnapshot(now)}
			c.snapshot.Dependencies[0].Candidates = nil
			c.admit()
			c.deliver(t.Context())
			if len(p.messages) != 0 {
				t.Fatal("baseline notified")
			}
			for _, kind := range []string{"major", "minor", "patch", "digest", "pinDigest", "pin"} {
				u := snapshot.Candidate{NewValue: new("2"), UpdateType: kind}
				u.ID = snapshot.ID(c.snapshot.Dependencies[0].ID, u.NewValue, u.NewDigest, u.UpdateType)
				c.snapshot.Dependencies[0].Candidates = append(c.snapshot.Dependencies[0].Candidates, u)
			}
			c.admit()
			c.deliver(t.Context())
			if len(p.messages) != len(tc.want) {
				t.Fatalf("ordinary events=%d, want %d", len(p.messages), len(tc.want))
			}
			for i, u := range c.snapshot.Dependencies[0].Candidates {
				found := false
				for _, m := range p.messages {
					if m.EventID == "image-upgrade:"+u.ID {
						found = true
					}
				}
				if found != tc.want[u.UpdateType] {
					t.Fatal("incorrect notification selection", u.UpdateType)
				}
				c.snapshot.Dependencies[0].Candidates[i].IsVulnerabilityAlert = true
			}
			c.admit()
			c.deliver(t.Context())
			want := len(tc.want)
			if tc.enabled {
				want += 6
			}
			if len(p.messages) != want {
				t.Fatal("security did not bypass filter, or master switch ignored")
			}
			for _, m := range p.messages[len(tc.want):] {
				if m.Severity != notifications.SeverityWarning {
					t.Fatal("security severity changed")
				}
			}
			c.admit()
			c.deliver(t.Context())
			if len(p.messages) != want {
				t.Fatal("repeat poll notified again")
			}
		})
	}
}

func TestNotificationTypeConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		ok          bool
	}{
		{"omitted", "", true}, {"empty", `,"notificationUpdateTypes":[]`, true},
		{"all", `,"notificationUpdateTypes":["major","minor","patch","digest","pinDigest","pin"]`, true},
		{"duplicate", `,"notificationUpdateTypes":["digest","digest"]`, false},
		{"unknown", `,"notificationUpdateTypes":["replacement"]`, false},
		{"wrong-case", `,"notificationUpdateTypes":["Major"]`, false},
		{"wrong-type", `,"notificationUpdateTypes":"major"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := json.RawMessage(`{"snapshotURL":"https://snapshot.example.test/snapshot.json","browserRepositoryURL":"https://github.com/example/read-only"` + tc.value + `}`)
			_, client, err := parseConfig(data, nil)
			if client != nil {
				client.CloseIdleConnections()
			}
			if (err == nil) != tc.ok {
				t.Fatal("incorrect config validation", err)
			}
		})
	}
}
