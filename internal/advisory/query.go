package advisory

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh/plugin-sdk/backend"
)

type Filter struct {
	Search     string `json:"search,omitempty"`
	Repository string `json:"repository,omitempty"`
	Assessment string `json:"assessment,omitempty"`
	UpdateType string `json:"updateType,omitempty"`
	Workflow   string `json:"workflow,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}
type Meta struct {
	SnapshotDigest      string              `json:"snapshotDigest"`
	Controller          requests.Catalog    `json:"controller"`
	ControllerAvailable bool                `json:"controllerAvailable"`
	Repository          snapshot.Repository `json:"repository"`
	Producer            snapshot.Producer   `json:"producer"`
	Coverage            snapshot.Coverage   `json:"coverage"`
	CompletedAt         time.Time           `json:"completedAt"`
}
type Item struct {
	snapshot.Dependency
	ServiceID string `json:"serviceId,omitempty"`
}
type List struct {
	Meta
	Items      []Item `json:"items"`
	Total      int    `json:"total"`
	Revision   string `json:"revision"`
	NextCursor string `json:"nextCursor"`
}
type Summary struct {
	Meta
	Assessments map[string]int `json:"assessments"`
	UpdateTypes map[string]int `json:"updateTypes"`
	Candidates  int            `json:"candidates"`
}
type Detail struct {
	Meta
	Dependency Item `json:"dependency"`
}

func (c *connection) meta() Meta {
	s := c.snapshot
	catalog := c.controller
	if catalog.Requests == nil {
		catalog.Requests = []requests.Record{}
	}
	return Meta{Repository: s.Repository, Producer: s.Producer, Coverage: s.Coverage, CompletedAt: s.CompletedAt, SnapshotDigest: requests.Digest(s), Controller: catalog, ControllerAvailable: c.controllerAvailable}
}
func (c *connection) item(d snapshot.Dependency) Item {
	item := Item{Dependency: d}
	for _, m := range c.config.ServiceMappings {
		if m.DependencyID == d.ID {
			item.ServiceID = m.ServiceID
		}
	}
	return item
}
func (b *Backend) Query(ctx context.Context, r backend.Request) (backend.Result, error) {
	if ctx.Err() != nil {
		return backend.Result{}, ctx.Err()
	}
	c, err := b.get(r.ConnectionID)
	if err != nil {
		return backend.Result{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refresh(time.Now())
	if c.snapshot == nil {
		return backend.Result{}, backend.Fail("unavailable")
	}
	if !backend.Object(r.Input, backend.MaxInput) {
		return backend.Result{}, backend.Fail("invalid_argument")
	}
	var value any
	switch r.OperationID {
	case "upgrades.list":
		value, err = c.list(r.Input)
	case "upgrades.summary":
		var f Filter
		if decode(r.Input, &f) != nil || f.Cursor != "" || f.Revision != "" || f.Limit != 0 || !ValidFilter(f) {
			return backend.Result{}, backend.Fail("invalid_argument")
		}
		s := Summary{Meta: c.meta(), Assessments: map[string]int{}, UpdateTypes: map[string]int{}}
		for _, d := range c.snapshot.Dependencies {
			if !c.matches(d, f) {
				continue
			}
			s.Assessments[d.Assessment]++
			for _, u := range d.Candidates {
				if candidateMatches(u, f) {
					s.UpdateTypes[u.UpdateType]++
					s.Candidates++
				}
			}
		}
		value = s
	case "upgrades.get":
		var input struct {
			ID string `json:"dependencyId"`
		}
		if decode(r.Input, &input) != nil || !validID(input.ID) {
			return backend.Result{}, backend.Fail("invalid_argument")
		}
		err = backend.Fail("not_found")
		for _, d := range c.snapshot.Dependencies {
			if d.ID == input.ID {
				value = Detail{c.meta(), c.item(d)}
				err = nil
				break
			}
		}
	default:
		return backend.Result{}, backend.Fail("not_found")
	}
	if err != nil {
		return backend.Result{}, err
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) > backend.MaxResult {
		return backend.Result{}, backend.Fail("resource_exhausted")
	}
	observed := c.snapshot.CompletedAt
	return backend.Result{Data: data, State: c.state, ObservationRevision: strconv.FormatUint(c.observation, 10), ObservedAt: &observed, LastSuccessAt: c.success, SourceError: c.sourceError}, nil
}

// ValidFilter validates shared list/summary filter syntax, including cursor size.
func ValidFilter(f Filter) bool {
	return len(f.Search) <= 256 && len(f.Repository) <= 256 && len(f.Cursor) <= 1024 && (f.Assessment == "" || slices.Contains([]string{"updates_found", "no_candidate", "excluded", "unknown"}, f.Assessment)) && (f.UpdateType == "" || slices.Contains([]string{"major", "minor", "patch", "digest", "pinDigest", "pin", "unknown"}, f.UpdateType)) && (f.Workflow == "" || slices.Contains([]string{"pr_open", "branch_only", "none_observed", "unknown"}, f.Workflow))
}
func candidateMatches(u snapshot.Candidate, f Filter) bool {
	return (f.UpdateType == "" || u.UpdateType == f.UpdateType) && (f.Workflow == "" || u.Workflow == f.Workflow)
}
func (c *connection) matches(d snapshot.Dependency, f Filter) bool {
	if f.Repository != "" && f.Repository != c.snapshot.Repository.Name || f.Assessment != "" && f.Assessment != d.Assessment {
		return false
	}
	if f.UpdateType != "" || f.Workflow != "" {
		if !slices.ContainsFunc(d.Candidates, func(u snapshot.Candidate) bool { return candidateMatches(u, f) }) {
			return false
		}
	}
	text := strings.ToLower(d.PackageName + " " + d.PackageFile + " " + d.Manager)
	for _, term := range strings.Fields(strings.ToLower(f.Search)) {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}
func (c *connection) list(data []byte) (List, error) {
	var f Filter
	if decode(data, &f) != nil || !ValidFilter(f) {
		return List{}, backend.Fail("invalid_argument")
	}
	if f.Limit == 0 {
		f.Limit = 50
	}
	if f.Limit < 1 || f.Limit > 100 {
		return List{}, backend.Fail("invalid_argument")
	}
	revision := strconv.FormatUint(c.observation, 10)
	if f.Revision != "" && f.Revision != revision {
		return List{}, backend.Fail("conflict")
	}
	offset := 0
	filter := f
	filter.Cursor = ""
	filter.Revision = ""
	signature := snapshot.ID(c.id, c.revision, filter)
	if f.Cursor != "" {
		var token struct {
			Revision string `json:"revision"`
			Filter   string `json:"filter"`
			Offset   int    `json:"offset"`
		}
		raw, e := base64.RawURLEncoding.DecodeString(f.Cursor)
		if e != nil || decode(raw, &token) != nil || token.Offset < 1 || token.Offset > 10000 {
			return List{}, backend.Fail("invalid_argument")
		}
		if token.Revision != revision || token.Filter != signature {
			return List{}, backend.Fail("conflict")
		}
		offset = token.Offset
	}
	items := []Item{}
	for _, d := range c.snapshot.Dependencies {
		if c.matches(d, f) {
			item := c.item(d)
			if f.UpdateType != "" || f.Workflow != "" {
				item.Candidates = []snapshot.Candidate{}
				for _, candidate := range d.Candidates {
					if candidateMatches(candidate, f) {
						item.Candidates = append(item.Candidates, candidate)
					}
				}
			}
			items = append(items, item)
		}
	}
	if offset > len(items) {
		return List{}, backend.Fail("invalid_argument")
	}
	end := min(offset+f.Limit, len(items))
	next := ""
	if end < len(items) {
		raw, _ := json.Marshal(map[string]any{"revision": revision, "filter": signature, "offset": end})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return List{c.meta(), items[offset:end], len(items), revision, next}, nil
}
