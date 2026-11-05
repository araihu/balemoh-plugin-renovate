package pluginui

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/araihu/balemoh-plugin-renovate/internal/advisory"
	"github.com/araihu/balemoh-plugin-renovate/internal/requests"
	"github.com/araihu/balemoh-plugin-renovate/internal/snapshot"
	"github.com/araihu/balemoh-plugin-renovate/internal/view"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/render"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func (h Handler) request(ctx context.Context, r ui.Request, host ui.HostServices) (ui.Response, error) {
	if r.ContributionType != "page" || r.Mode != "page" || (r.Method != "GET" && r.Method != "POST") {
		return ui.Response{}, ui.ErrUnavailable
	}
	if _, ok := h.Manifest.Page("request"); !ok {
		return ui.Response{}, ui.ErrUnavailable
	}
	current, _ := r.PageURL(h.Manifest, "current", nil)
	back, e := r.PageURL(h.Manifest, "detail", map[string]string{"dependency-id": r.PathParams["dependency-id"]})
	if e != nil {
		return ui.Response{}, ui.ErrUnavailable
	}
	filter := advisory.Filter{Search: r.Query.Get("search"), Assessment: querySelection(r.Query, "assessment"), UpdateType: querySelection(r.Query, "updateType"), Workflow: querySelection(r.Query, "workflow")}
	if !r.Query.Has("assessment") {
		filter.Assessment = "updates_found"
	}
	q := filterQuery(filter)
	back = appendQuery(back, q)
	action, _ := r.PageURL(h.Manifest, "request", r.PathParams)
	action = appendQuery(action, q)
	m := view.Model{Request: r, CurrentURL: current, BackURL: back, RestartURL: back, ResetURL: back, ActionURL: action, StatusURL: back, RequestLinks: map[string]string{}}
	raw, _ := json.Marshal(map[string]string{"dependencyId": r.PathParams["dependency-id"]})
	result, err := host.Query(ctx, "upgrades.get", raw)
	if err != nil {
		response, e := render.Component(ctx, "Upgrade unavailable", view.Recovery(m, backend.Code(err)))
		response.Status = errorStatus(err)
		response.AssetIDs = []string{"layout"}
		return response, e
	}
	if json.Unmarshal(result.Data, &m.Detail) != nil {
		return ui.Response{}, backend.Fail("invalid_response")
	}
	m.Meta, m.Result = m.Detail.Meta, result
	var candidate *snapshot.Candidate
	for _, u := range m.Detail.Dependency.Candidates {
		if u.ID == r.PathParams["candidate-id"] {
			value := u
			candidate = &value
		}
	}
	if candidate == nil {
		response, e := render.Component(ctx, "Upgrade unavailable", view.Recovery(m, "not_found"))
		response.Status = 404
		response.AssetIDs = []string{"layout"}
		return response, e
	}
	m.Selected = *candidate
	status := 200
	if r.Method == "POST" {
		// Repository, version, branch and URLs come from retained server data, never form fields.
		valid := len(r.Form) == 2 && len(r.Form["snapshotDigest"]) == 1 && len(r.Form["approved"]) == 1 && r.Form.Get("approved") == "true"
		if !valid {
			err = backend.Fail("invalid_argument")
		} else if r.Form.Get("snapshotDigest") != m.Meta.SnapshotDigest {
			err = backend.Fail("conflict")
		} else if !m.CanRequest(*candidate) {
			err = backend.Fail("permission_denied")
		} else if actions, ok := host.(ui.ActionServices); ok {
			input, _ := json.Marshal(requests.Selection{DependencyID: m.Detail.Dependency.ID, CandidateID: candidate.ID, SnapshotDigest: r.Form.Get("snapshotDigest")})
			var receipt backend.ActionResult
			receipt, err = actions.Action(ctx, "upgrades.request", input)
			if err == nil {
				var value struct {
					ID string `json:"requestId"`
				}
				decodeErr := json.Unmarshal(receipt.Data, &value)
				_, hexErr := hex.DecodeString(value.ID)
				if decodeErr != nil || len(value.ID) != 64 || hexErr != nil {
					err = backend.Fail("invalid_response")
				} else {
					query := q.Clone()
					query.Set("request", value.ID)
					return ui.Response{Kind: "redirect", Status: 303, Redirect: &ui.Redirect{Page: "detail", Params: map[string]string{"dependency-id": m.Detail.Dependency.ID}, Query: query}}, nil
				}
			}
		} else {
			err = backend.Fail("permission_denied")
		}
		if err != nil {
			status = 503
			switch backend.Code(err) {
			case "conflict":
				status = 409
				m.ActionError = "The observation changed. Review the current selection before requesting again."
			case "invalid_argument":
				status = 400
				m.ActionError = "This selection cannot be requested. Return to its details and review the available candidate."
			case "permission_denied":
				status = 403
				m.ActionError = "PR requests are not enabled for this connection. Writer migration or an explicit action grant is still required."
			default:
				m.ActionError = "The request could not be confirmed. Check the image details for an existing request before retrying."
			}
		}
	}
	response, e := render.Component(ctx, "Request image upgrade PR", view.RequestUpgrade(m))
	response.Description = "Review the selected image upgrade. Renovate creates the PR; merging remains manual."
	response.Status = status
	response.AssetIDs = []string{"layout"}
	return response, e
}
