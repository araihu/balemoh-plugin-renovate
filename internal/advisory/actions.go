package advisory

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh/plugin-sdk/backend"
)

func (c *connection) observeRequests(ctx context.Context) {
	if c.config.ControllerURL == "" {
		return
	}
	r, _ := http.NewRequestWithContext(ctx, "GET", c.config.ControllerURL+"/v1/requests", nil)
	r.Header.Set("Authorization", "Bearer "+c.controllerReadToken)
	response, err := c.client.Do(r)
	var catalog requests.Catalog
	if err == nil {
		defer response.Body.Close()
		if response.StatusCode != 200 {
			err = backend.Fail("unavailable")
		} else {
			data, e := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
			if e != nil {
				err = e
			} else {
				catalog, err = requests.DecodeCatalog(data)
			}
		}
	}
	if err == nil {
		for _, r := range catalog.Requests {
			if r.Validate() != nil || r.Repository.Name != strings.TrimPrefix(c.config.BrowserRepositoryURL, "https://github.com/") {
				err = backend.Fail("invalid_response")
				break
			}
			if r.PR != nil && r.PR.URL != "https://github.com/"+r.Repository.Name+"/pull/"+strconv.Itoa(r.PR.Number) {
				err = backend.Fail("invalid_response")
				break
			}
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	previous, _ := json.Marshal(c.controller)
	if err != nil {
		if c.controllerAvailable {
			c.observation++
		}
		c.controllerAvailable = false
		return
	}
	next, _ := json.Marshal(catalog)
	if !c.controllerAvailable || string(next) != string(previous) {
		c.observation++
	}
	c.controller = catalog
	c.controllerAvailable = true
}

func (b *Backend) Action(ctx context.Context, r backend.Request) (backend.ActionResult, error) {
	if r.OperationID != "upgrades.request" || !backend.Object(r.Input, backend.MaxInput) {
		return backend.ActionResult{}, backend.Fail("invalid_argument")
	}
	var selection requests.Selection
	if decode(r.Input, &selection) != nil {
		return backend.ActionResult{}, backend.Fail("invalid_argument")
	}
	c, err := b.get(r.ConnectionID)
	if err != nil {
		return backend.ActionResult{}, err
	}
	c.mu.Lock()
	if c.config.ControllerURL == "" || !c.controllerAvailable || !c.controller.Enabled || c.controllerActionToken == "" {
		c.mu.Unlock()
		return backend.ActionResult{}, backend.Fail("permission_denied")
	}
	if c.snapshot == nil || c.stateAt(time.Now()) == "stale" || selection.SnapshotDigest != requests.Digest(c.snapshot) {
		c.mu.Unlock()
		return backend.ActionResult{}, backend.Fail("conflict")
	}
	allowed := false
	for _, d := range c.snapshot.Dependencies {
		if d.ID == selection.DependencyID {
			for _, u := range d.Candidates {
				if u.ID == selection.CandidateID && u.PR == nil && requests.Supported(d, u) {
					allowed = true
				}
			}
		}
	}
	if !allowed {
		c.mu.Unlock()
		return backend.ActionResult{}, backend.Fail("invalid_argument")
	}
	endpoint, token, client := c.config.ControllerURL, c.controllerActionToken, c.client
	c.mu.Unlock()
	request, _ := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/requests", strings.NewReader(string(r.Input)))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return backend.ActionResult{}, backend.Fail("unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		code := map[int]string{400: "invalid_argument", 401: "permission_denied", 403: "permission_denied", 409: "conflict"}[response.StatusCode]
		return backend.ActionResult{}, backend.Fail(code)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	var record requests.Record
	if err != nil || len(data) > 1<<20 || decode(data, &record) != nil || record.Validate() != nil || record.Dependency.ID != selection.DependencyID || record.Candidate.ID != selection.CandidateID || record.Repository.Name != strings.TrimPrefix(c.config.BrowserRepositoryURL, "https://github.com/") {
		return backend.ActionResult{}, backend.Fail("invalid_response")
	}
	c.mu.Lock()
	found := false
	for i, r := range c.controller.Requests {
		if r.ID == record.ID {
			c.controller.Requests[i] = record
			found = true
		}
	}
	if !found {
		c.controller.Requests = append(c.controller.Requests, record)
	}
	c.observation++
	c.mu.Unlock()
	output, _ := json.Marshal(map[string]string{"requestId": record.ID})
	return backend.ActionResult{Data: output}, nil
}
