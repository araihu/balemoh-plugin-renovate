package advisory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
)

func TestMultipleFilterSelections(t *testing.T) {
	s := testSnapshot(time.Now())
	d := &s.Dependencies[0]
	d.Candidates = []snapshot.Candidate{
		{UpdateType: "major", Workflow: "unknown"},
		{UpdateType: "minor", Workflow: "pr_open"},
		{UpdateType: "digest", Workflow: "unknown"},
	}
	other := *d
	other.ID = strings.Repeat("b", 64)
	other.Assessment = "unknown"
	other.Candidates = nil
	s.Dependencies = append(s.Dependencies, other)
	c := &connection{snapshot: s, observation: 1}
	for _, tc := range []struct {
		name               string
		filter             Filter
		groups, candidates int
	}{
		{"all", Filter{}, 2, 3},
		{"assessment union", Filter{Assessment: "updates_found,unknown"}, 2, 3},
		{"type union", Filter{UpdateType: "major,minor"}, 1, 2},
		{"workflow union", Filter{Workflow: "unknown,pr_open"}, 1, 3},
		{"intersection", Filter{Assessment: "updates_found,unknown", UpdateType: "major,minor", Workflow: "pr_open"}, 1, 1},
		{"same candidate intersection", Filter{UpdateType: "major,digest", Workflow: "pr_open"}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(tc.filter)
			got, err := c.list(raw)
			if err != nil {
				t.Fatal(err)
			}
			candidates := 0
			for _, item := range got.Items {
				candidates += len(item.Candidates)
			}
			if got.Total != tc.groups || candidates != tc.candidates {
				t.Fatalf("groups=%d candidates=%d", got.Total, candidates)
			}
		})
	}
}
func TestMultipleFilterValidationAndCursorBinding(t *testing.T) {
	for _, f := range []Filter{
		{Assessment: "updates_found,invalid"}, {Assessment: "unknown,unknown"},
		{UpdateType: "minor,"}, {UpdateType: ",major"}, {UpdateType: "Major,minor"},
		{Workflow: "pr_open,unknown,invalid"},
	} {
		if ValidFilter(f) {
			t.Fatalf("accepted invalid selection %+v", f)
		}
	}
	s := testSnapshot(time.Now())
	other := s.Dependencies[0]
	other.ID = strings.Repeat("b", 64)
	s.Dependencies = append(s.Dependencies, other)
	c := &connection{snapshot: s, observation: 1}
	first, err := c.list([]byte(`{"assessment":"updates_found,unknown","limit":1}`))
	if err != nil || first.NextCursor == "" {
		t.Fatalf("first page: %v", err)
	}
	raw, _ := json.Marshal(Filter{Assessment: "updates_found,unknown", Limit: 1, Cursor: first.NextCursor, Revision: first.Revision})
	second, err := c.list(raw)
	if err != nil || len(second.Items) != 1 || second.NextCursor != "" {
		t.Fatalf("second page: %v", err)
	}
	raw, _ = json.Marshal(Filter{Assessment: "updates_found,excluded", Limit: 1, Cursor: first.NextCursor, Revision: first.Revision})
	if _, err = c.list(raw); err == nil {
		t.Fatal("cursor reused with different multi-selection")
	}
}
