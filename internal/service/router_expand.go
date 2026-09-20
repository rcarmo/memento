package service

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var searchRequestPrefix = regexp.MustCompile(`(?i)^(?:(?:please|kindly)\s+)?(?:find|search(?:\s+for)?|show|fetch|locate|get)\s+`)
var statusProjections = map[string]string{"service_version": "service_version", "schema_version": "schema_version", "repo_revision": "repo_revision", "index_revision": "index_revision", "index_stale": "index_stale", "principal": "principal", "visible_concepts": "visible_concepts", "proposal_backlog": "proposal_backlog", "limits": "limits", "roles": "roles", "features": "features", "readiness": "readiness", "semantic_search_ready": "readiness.semantic_search.ready", "semantic_search_model_id": "readiness.semantic_search.model_id", "semantic_search_dimensions": "readiness.semantic_search.dimensions", "semantic_search_embedding_revision": "readiness.semantic_search.embedding_revision", "semantic_search_sqlite_vector_enabled": "readiness.semantic_search.sqlite_vector_enabled"}
var readProjections = map[string]string{"path": "path", "frontmatter": "frontmatter", "body": "body", "title": "frontmatter.title", "type": "frontmatter.type", "status": "frontmatter.status", "tags": "frontmatter.tags", "aliases": "frontmatter.aliases"}

func derivedSearchQuery(request string) string {
	derived := strings.TrimSpace(searchRequestPrefix.ReplaceAllString(request, ""))
	if derived == "" {
		return request
	}
	return derived
}
func ExpandRouterAction(action RouterAction, request string) map[string]any {
	query := derivedSearchQuery(request)
	switch action.Action {
	case "search_then_read":
		search := map[string]any{"query": query, "limit": json.Number("1"), "query_syntax": "plain", "search_mode": nil, "concept_type": nil, "cursor": nil}
		if action.SearchMode != "" {
			search["search_mode"] = action.SearchMode
		}
		plan := map[string]any{"operations": []any{map[string]any{"op": "search", "args": search, "save_as": "hits"}, map[string]any{"op": "read", "args": map[string]any{"id_or_path": "$hits.results.0.path"}, "save_as": "doc"}}, "returns": []any{map[string]any{"name": "document", "ref": "$doc", "fields": []any{}, "limit": nil}}, "stop_on_error": true}
		return map[string]any{"kind": "execute", "tool": "memory_execute", "args": map[string]any{"plan": plan}}
	case "search_paths":
		return map[string]any{"kind": "direct", "tool": "memory_search", "args": map[string]any{"query": query, "limit": action.Limit, "search_mode": action.SearchMode}, "projection": map[string]any{"ref": "results", "fields": []any{"path"}, "limit": action.Limit}}
	case "status_field":
		return map[string]any{"kind": "direct", "tool": "memory_status", "args": map[string]any{}, "projection": map[string]any{"ref": statusProjections[action.Field], "fields": []any{}, "limit": nil}}
	case "search_then_graph":
		plan := map[string]any{"operations": []any{map[string]any{"op": "search", "args": map[string]any{"query": query, "limit": json.Number("1"), "search_mode": action.SearchMode, "query_syntax": "plain", "concept_type": nil, "cursor": nil}, "save_as": "hits"}, map[string]any{"op": "graph", "args": map[string]any{"id_or_path": "$hits.results.0.path", "depth": json.Number(strconv.Itoa(action.Depth))}, "save_as": "graph"}}, "returns": []any{map[string]any{"name": "graph", "ref": "$graph", "fields": []any{}, "limit": nil}}, "stop_on_error": true}
		return map[string]any{"kind": "execute", "tool": "memory_execute", "args": map[string]any{"plan": plan}}
	case "read_field":
		if !strings.Contains(request, action.IDOrPath) {
			return nil
		}
		return map[string]any{"kind": "direct", "tool": "memory_read", "args": map[string]any{"id_or_path": action.IDOrPath}, "projection": map[string]any{"ref": readProjections[action.Field], "fields": []any{}, "limit": nil}}
	default:
		return nil
	}
}
