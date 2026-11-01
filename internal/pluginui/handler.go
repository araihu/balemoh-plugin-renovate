// Package pluginui renders host-scoped, read-only pages and widgets.
package pluginui

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/a-h/templ"
	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/view"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/render"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

type Handler struct{ Manifest manifest.Manifest }

func (h Handler) Handle(ctx context.Context, r ui.Request, host ui.HostServices) (ui.Response, error) {
	if r.Method != "GET" {
		return ui.Response{}, ui.ErrUnavailable
	}
	if r.ContributionType == "widget" {
		if r.ContributionID != "summary" {
			return ui.Response{}, ui.ErrUnavailable
		}
	} else if _, ok := h.Manifest.Page(r.ContributionID); !ok {
		return ui.Response{}, ui.ErrUnavailable
	}
	current, _ := r.PageURL(h.Manifest, "current", nil)
	coverage, _ := r.PageURL(h.Manifest, "coverage", nil)
	f := advisory.Filter{Search: r.Query.Get("search"), Repository: r.Query.Get("repository"), Assessment: r.Query.Get("assessment"), UpdateType: r.Query.Get("updateType"), Workflow: r.Query.Get("workflow"), Cursor: r.Query.Get("cursor"), Revision: r.Query.Get("revision"), Limit: 50}
	if !r.Query.Has("assessment") {
		f.Assessment = "updates_found"
	}
	var result ui.QueryResult
	var component templ.Component
	var err error
	if r.ContributionType == "widget" {
		p, e := host.Placement(ctx)
		if e != nil {
			return ui.Response{}, e
		}
		var options struct {
			Repository string `json:"repository"`
			UpdateType string `json:"updateType"`
			MaxRows    int    `json:"maxRows"`
		}
		if json.Unmarshal(p.Options, &options) != nil {
			return ui.Response{}, ui.ErrUnavailable
		}
		f = advisory.Filter{Repository: options.Repository, UpdateType: options.UpdateType, Assessment: "updates_found", Limit: options.MaxRows}
		if f.Limit == 0 {
			f.Limit = 5
		}
		if f.Limit > 5 || f.Limit < 1 {
			return ui.Response{}, ui.ErrUnavailable
		}
	}
	q := navigationQuery(f)
	restart := appendQuery(current, filterQuery(f))
	resetFilter := advisory.Filter{Assessment: "updates_found"}
	if len(f.Repository) <= 256 {
		resetFilter.Repository = f.Repository
	}
	model := view.Model{Request: r, Filter: f, CurrentURL: current, BackURL: appendQuery(current, q), RestartURL: restart, ResetURL: appendQuery(current, filterQuery(resetFilter)), CoverageURL: appendQuery(coverage, q), Services: map[string]ui.ServiceIdentity{}, Links: map[string]string{}}

	if r.ContributionID == "detail" {
		input, _ := json.Marshal(map[string]string{"dependencyId": r.PathParams["dependency-id"]})
		result, err = host.Query(ctx, "upgrades.get", input)
		var data advisory.Detail
		if err == nil {
			err = json.Unmarshal(result.Data, &data)
			model.Detail = data
			model.Meta = data.Meta
			model.Result = result
			resolve(ctx, host, &model, []advisory.Item{data.Dependency})
			component = view.Detail(model)
		}
	} else if r.ContributionID == "coverage" {
		result, err = host.Query(ctx, "upgrades.summary", json.RawMessage(`{}`))
		var data advisory.Summary
		if err == nil {
			err = json.Unmarshal(result.Data, &data)
			model.Meta = data.Meta
			model.Result = result
			component = view.Coverage(model)
		}
	} else {
		input, _ := json.Marshal(f)
		result, err = host.Query(ctx, "upgrades.list", input)
		var data advisory.List
		if err == nil {
			err = json.Unmarshal(result.Data, &data)
			model.List = data
			model.Meta = data.Meta
			model.Result = result
			resolve(ctx, host, &model, data.Items)
			for _, item := range data.Items {
				link, _ := r.PageURL(h.Manifest, "detail", map[string]string{"dependency-id": item.ID})
				if q.Encode() != "" {
					link += "?" + q.Encode()
				}
				model.Links[item.ID] = link
			}
			next := q.Clone()
			next.Set("cursor", data.NextCursor)
			next.Set("revision", data.Revision)
			model.NextURL = current + "?" + next.Encode()

			if r.ContributionType == "widget" {
				component = view.Widget(model)
			} else if r.Mode == "fragment" && r.Region == "results" {
				component = view.Results(model)
			} else {
				component = view.Current(model)
			}
		}
	}
	status := 200
	if err != nil {
		code := backend.Code(err)
		status = 503
		switch code {
		case "not_found":
			status = 404
		case "conflict":
			status = 409
		case "invalid_argument":
			status = 400
		case "permission_denied":
			status = 403
		}
		if code == "conflict" || code == "not_found" {
			model.BackURL = model.RestartURL
		}
		if code == "invalid_argument" {
			hasPosition := f.Cursor != "" || f.Revision != ""
			f.Cursor, f.Revision = "", ""
			model.BackURL = model.ResetURL
			if hasPosition && advisory.ValidFilter(f) {
				model.BackURL = model.RestartURL
			}
		}
		component = view.Recovery(model, code)
	}
	if r.Mode == "fragment" && r.Region != "results" {
		return ui.Response{}, ui.ErrUnavailable
	}
	response, e := render.Component(ctx, "Image upgrades", component)
	response.Status = status
	response.AssetIDs = []string{"layout"}
	if r.Mode == "fragment" {
		response.Kind = "fragment"
	}
	return response, e
}
func filterQuery(f advisory.Filter) url.Values {
	q := url.Values{}
	q.Set("assessment", f.Assessment)
	for k, v := range map[string]string{"search": f.Search, "repository": f.Repository, "assessment": f.Assessment, "updateType": f.UpdateType, "workflow": f.Workflow} {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q
}
func appendQuery(path string, q url.Values) string {
	if q.Encode() == "" {
		return path
	}
	return path + "?" + q.Encode()
}
func navigationQuery(f advisory.Filter) url.Values {
	q := filterQuery(f)
	if f.Cursor != "" {
		q.Set("cursor", f.Cursor)
	}
	if f.Revision != "" {
		q.Set("revision", f.Revision)
	}
	return q
}
func resolve(ctx context.Context, host ui.HostServices, m *view.Model, items []advisory.Item) {
	catalog, ok := host.(ui.ServiceCatalog)
	if !ok {
		return
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, d := range items {
		if d.ServiceID != "" && !seen[d.ServiceID] {
			seen[d.ServiceID] = true
			ids = append(ids, d.ServiceID)
		}
	}
	if len(ids) == 0 {
		return
	}
	identities, e := catalog.ResolveServices(ctx, ids)
	if e != nil || ui.ValidateServiceIdentities(ids, identities) != nil {
		return
	}
	for _, identity := range identities {
		m.Services[identity.ID] = identity
	}
}
