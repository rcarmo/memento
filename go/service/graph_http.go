package service

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/graphdebug"
	"github.com/rcarmo/memento/go/umcp"
)

type GraphHTTPConfig struct {
	Enabled                    bool
	ExportNodeLimit            int
	RoutePrefix                string
	Overview                   graphdebug.OverviewOptions
	Neighbourhood              graphdebug.NeighbourhoodOptions
	Cluster                    graphdebug.ClusterOptions
	PreviewChars, SummaryLimit int
}
type GraphSnapshots interface {
	ExportSelection(context.Context, []string, int, int, *access.EffectivePolicy) ([]graphdebug.Node, []graphdebug.Edge, graphdebug.Revisions, error)
	Overview(context.Context, *access.EffectivePolicy, graphdebug.OverviewOptions) (graphdebug.Overview, error)
	Search(context.Context, string, *access.EffectivePolicy) (graphdebug.SearchResults, error)
	ExpandCluster(context.Context, string, *access.EffectivePolicy, graphdebug.ClusterOptions) (graphdebug.ClusterExpansion, error)
	Detail(context.Context, string, *access.EffectivePolicy, int, int, int) (graphdebug.Detail, error)
	Neighbourhood(context.Context, string, *access.EffectivePolicy, graphdebug.NeighbourhoodOptions) (graphdebug.Neighbourhood, error)
}

type GraphHTTP struct {
	Config    GraphHTTPConfig
	Snapshots GraphSnapshots
	Refresh   *graphdebug.RefreshCoordinator
	Policies  *GraphPolicyDirectory
	Policy    func(context.Context, map[string]string) (*access.EffectivePolicy, error)
}

var graphHeaders = [][2]string{{"Cache-Control", "no-store"}, {"X-Content-Type-Options", "nosniff"}}

func graphJSON(payload any, status int) (*umcp.HTTPResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	mime := "application/json; charset=utf-8"
	return &umcp.HTTPResponse{Status: status, Body: body, ContentType: &mime, Headers: graphHeaders}, nil
}
func graphError(message string, status int) (*umcp.HTTPResponse, error) {
	return graphJSON(map[string]any{"error": message}, status)
}
func graphNotFound() *umcp.HTTPResponse {
	return &umcp.HTTPResponse{Status: 404, Headers: graphHeaders}
}
func (h GraphHTTP) Handle(ctx context.Context, method, path string, headers map[string]string, body []byte, _ string) (*umcp.HTTPResponse, error) {
	prefix := h.Config.RoutePrefix
	if path != prefix && !strings.HasPrefix(path, prefix+"/") {
		return nil, nil
	}
	if !h.Config.Enabled {
		return graphNotFound(), nil
	}
	var policy *access.EffectivePolicy
	var err error
	if h.Policies != nil {
		policy, err = h.Policies.Resolve(ctx, headers)
	} else if h.Policy != nil {
		policy, err = h.Policy(ctx, headers)
	}
	if err != nil {
		return graphError(err.Error(), 400)
	}
	postAllowed := path == prefix+"/api/v1/search" || path == prefix+"/api/v1/embeddings/refresh" || path == prefix+"/api/v1/export/json" || path == prefix+"/api/v1/export/svg"
	if method != "GET" && !(method == "POST" && postAllowed) {
		response := graphNotFound()
		response.Status = 405
		response.Headers = append(response.Headers, [2]string{"Allow", "GET"})
		return response, nil
	}
	if method == "GET" && len(body) > 0 {
		return &umcp.HTTPResponse{Status: 400, Headers: graphHeaders}, nil
	}
	switch {
	case method == "GET" && path == prefix+"/api/v1/principals":
		principals := []GraphPrincipal{}
		protected := []string{}
		if h.Policies != nil {
			principals, err = h.Policies.List(ctx)
			if err != nil {
				return nil, err
			}
			protected = append(protected, h.Policies.Static.ProtectedReadPrefixes...)
		}
		return graphJSON(map[string]any{"schema_version": 1, "protected_read_prefixes": protected, "principals": principals}, 200)
	case method == "GET" && (path == prefix || path == prefix+"/"):
		return graphNotFound(), nil
	case method == "GET" && path == prefix+"/api/v1/status":
		return graphJSON(map[string]any{"schema_version": 1, "enabled": true, "warning": "Unauthenticated visual debugger; trusted networks only.", "route_prefix": prefix}, 200)
	case method == "GET" && path == prefix+"/api/v1/embeddings/status":
		if h.Refresh == nil {
			return graphJSON(map[string]any{"available": false}, 200)
		}
		return graphJSON(h.Refresh.State(), 200)
	case method == "GET" && path == prefix+"/api/v1/overview":
		if h.Snapshots == nil {
			return graphError("graph snapshot unavailable", 503)
		}
		options := h.Config.Overview
		options.IncludeTrash = headers["x-memento-include-trash"] == "true"
		value, callErr := h.Snapshots.Overview(ctx, policy, options)
		if callErr != nil {
			return graphError(callErr.Error(), 404)
		}
		return graphJSON(value, 200)
	case method == "POST" && (path == prefix+"/api/v1/export/json" || path == prefix+"/api/v1/export/svg"):
		if h.Snapshots == nil {
			return graphError("graph snapshot unavailable", 503)
		}
		var payload any
		if json.Unmarshal(body, &payload) != nil {
			return graphError("invalid JSON", 400)
		}
		object, ok := payload.(map[string]any)
		if !ok {
			return graphError("export body must be an object", 400)
		}
		rawIDs, ok := object["concept_ids"].([]any)
		if !ok {
			return graphError("concept_ids must be an array of strings", 400)
		}
		ids := make([]string, 0, len(rawIDs))
		for _, raw := range rawIDs {
			id, ok := raw.(string)
			if !ok {
				return graphError("concept_ids must be an array of strings", 400)
			}
			ids = append(ids, id)
		}
		nodes, edges, revisions, callErr := h.Snapshots.ExportSelection(ctx, ids, h.Config.ExportNodeLimit, h.Config.Overview.EdgeLimit, policy)
		if callErr != nil {
			return graphError(callErr.Error(), 400)
		}
		if strings.HasSuffix(path, "/json") {
			settings, _ := object["settings"].(map[string]any)
			output, formatErr := graphdebug.ExportJSON(nodes, edges, revisions, settings)
			if formatErr != nil {
				return graphError(formatErr.Error(), 400)
			}
			mime := "application/json; charset=utf-8"
			return &umcp.HTTPResponse{Status: 200, Body: output, ContentType: &mime, Headers: graphHeaders}, nil
		}
		output := graphdebug.ExportSVG(nodes, edges, 1600, 1000)
		mime := "image/svg+xml"
		return &umcp.HTTPResponse{Status: 200, Body: output, ContentType: &mime, Headers: graphHeaders}, nil
	case method == "POST" && path == prefix+"/api/v1/embeddings/refresh":
		if policy != nil {
			return graphError("embedding refresh is disabled while simulating", 400)
		}
		if h.Refresh == nil {
			return graphError("semantic embedding refresh is unavailable", 503)
		}
		var payload map[string]any
		if json.Unmarshal(body, &payload) != nil {
			return graphError("invalid JSON", 400)
		}
		ids := []string{}
		if raw, exists := payload["concept_ids"]; exists {
			values, ok := raw.([]any)
			if !ok {
				return graphError("concept_ids must be an array of strings", 400)
			}
			for _, value := range values {
				id, ok := value.(string)
				if !ok {
					return graphError("concept_ids must be an array of strings", 400)
				}
				ids = append(ids, id)
			}
		}
		scope, _ := payload["scope"].(string)
		_, callErr := h.Refresh.Enqueue(ctx, scope, ids, payload["confirm_full"] == true)
		if callErr != nil {
			return graphError(callErr.Error(), 400)
		}
		return graphJSON(h.Refresh.State(), 202)
	case method == "POST" && path == prefix+"/api/v1/search":
		if h.Snapshots == nil {
			return graphError("graph snapshot unavailable", 503)
		}
		var payload map[string]any
		if json.Unmarshal(body, &payload) != nil {
			return graphError("invalid JSON", 400)
		}
		query, ok := payload["query"].(string)
		if !ok {
			return graphError("search query must be a string", 400)
		}
		value, callErr := h.Snapshots.Search(ctx, query, policy)
		if callErr != nil {
			return graphError(callErr.Error(), 400)
		}
		return graphJSON(value, 200)
	}
	for _, route := range []struct{ prefix, kind string }{{prefix + "/api/v1/clusters/", "cluster"}, {prefix + "/api/v1/memories/", "detail"}, {prefix + "/api/v1/neighbourhood/", "neighbourhood"}} {
		if method == "GET" && strings.HasPrefix(path, route.prefix) {
			id, decodeErr := url.PathUnescape(strings.TrimPrefix(path, route.prefix))
			if decodeErr != nil {
				return graphError("unknown memory", 404)
			}
			if h.Snapshots == nil {
				return graphError("graph snapshot unavailable", 503)
			}
			var value any
			var callErr error
			switch route.kind {
			case "cluster":
				value, callErr = h.Snapshots.ExpandCluster(ctx, id, policy, h.Config.Cluster)
			case "detail":
				value, callErr = h.Snapshots.Detail(ctx, id, policy, h.Config.PreviewChars, h.Config.Overview.EdgeLimit, h.Config.SummaryLimit)
			default:
				value, callErr = h.Snapshots.Neighbourhood(ctx, id, policy, h.Config.Neighbourhood)
			}
			if callErr != nil {
				return graphError(callErr.Error(), 404)
			}
			return graphJSON(value, 200)
		}
	}
	return graphNotFound(), nil
}
