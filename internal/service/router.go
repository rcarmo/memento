package service

import (
	"bytes"
	"encoding/json"
	"errors"
)

type RouterAction struct {
	Action, Query, SearchMode, Field, IDOrPath string
	Limit, Depth                               int
}

var statusAliases = map[string]string{"indexed": "index_revision", "index": "index_revision", "repository": "repo_revision", "revision": "repo_revision", "stale": "index_stale", "status": "readiness", "semantic_ready": "semantic_search_ready"}
var readAliases = map[string]string{"name": "title", "contents": "body", "content": "body"}
var statusFields = map[string]bool{"service_version": true, "schema_version": true, "repo_revision": true, "index_revision": true, "index_stale": true, "principal": true, "visible_concepts": true, "proposal_backlog": true, "limits": true, "roles": true, "features": true, "readiness": true, "semantic_search_ready": true, "semantic_search_model_id": true, "semantic_search_dimensions": true, "semantic_search_embedding_revision": true, "semantic_search_sqlite_vector_enabled": true}
var readFields = map[string]bool{"path": true, "frontmatter": true, "body": true, "title": true, "type": true, "status": true, "tags": true, "aliases": true}

func ParseNeedleRouterOutput(payload string) (RouterAction, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(payload))
	decoder.UseNumber()
	var calls []map[string]json.RawMessage
	if err := decoder.Decode(&calls); err != nil {
		return RouterAction{}, err
	}
	if len(calls) != 1 {
		return RouterAction{}, errors.New("needle router output must be a single-call JSON array")
	}
	call := calls[0]
	for key := range call {
		if key != "name" && key != "arguments" {
			return RouterAction{}, errors.New("needle router output call has extra fields")
		}
	}
	var name string
	if json.Unmarshal(call["name"], &name) != nil || name == "" {
		return RouterAction{}, errors.New("needle router output call name must be a non-empty string")
	}
	arguments := map[string]json.RawMessage{}
	if raw := call["arguments"]; len(raw) > 0 {
		if json.Unmarshal(raw, &arguments) != nil {
			return RouterAction{}, errors.New("needle router output call arguments must be an object")
		}
	}
	a := RouterAction{Action: name, Limit: 3, Depth: 1}
	allowed := map[string]bool{"action": true}
	text := func(key string, required bool) (string, error) {
		allowed[key] = true
		raw := arguments[key]
		if len(raw) == 0 {
			if required {
				return "", errors.New("router action missing field")
			}
			return "", nil
		}
		var value string
		if json.Unmarshal(raw, &value) != nil || required && value == "" {
			return "", errors.New("router action field is invalid")
		}
		return value, nil
	}
	integer := func(key string, current int) (int, error) {
		allowed[key] = true
		raw := arguments[key]
		if len(raw) == 0 {
			return current, nil
		}
		var value int
		if json.Unmarshal(raw, &value) != nil {
			return 0, errors.New("router action field is invalid")
		}
		return value, nil
	}
	var err error
	switch name {
	case "search_then_read":
		a.Query, err = text("query", true)
		if err == nil {
			a.SearchMode, err = text("search_mode", false)
		}
		if err == nil && a.SearchMode != "" && !searchMode(a.SearchMode) {
			err = errors.New("router action field is invalid")
		}
	case "search_paths":
		a.Query, err = text("query", true)
		if err == nil {
			a.Limit, err = integer("limit", 3)
		}
		if err == nil {
			a.SearchMode, err = text("search_mode", false)
		}
		if a.SearchMode == "" {
			a.SearchMode = "lexical"
		}
		if err == nil && (a.Limit != 1 && a.Limit != 2 && a.Limit != 3 && a.Limit != 5 || !searchMode(a.SearchMode)) {
			err = errors.New("router action field is invalid")
		}
	case "status_field":
		a.Field, err = text("field", true)
		if alias := statusAliases[a.Field]; alias != "" {
			a.Field = alias
		}
		if err == nil && !statusFields[a.Field] {
			err = errors.New("router action field is invalid")
		}
	case "search_then_graph":
		a.Query, err = text("query", true)
		if err == nil {
			a.Depth, err = integer("depth", 1)
		}
		if err == nil {
			a.SearchMode, err = text("search_mode", false)
		}
		if a.SearchMode == "" {
			a.SearchMode = "lexical"
		}
		if err == nil && (a.Depth < 1 || a.Depth > 2 || !searchMode(a.SearchMode)) {
			err = errors.New("router action field is invalid")
		}
	case "read_field":
		a.IDOrPath, err = text("id_or_path", true)
		if err == nil {
			a.Field, err = text("field", true)
		}
		if alias := readAliases[a.Field]; alias != "" {
			a.Field = alias
		}
		if err == nil && (len(a.IDOrPath) > 512 || !readFields[a.Field]) {
			err = errors.New("router action field is invalid")
		}
	case "UNKNOWN":
	default:
		err = errors.New("router action is invalid")
	}
	if err != nil {
		return RouterAction{}, err
	}
	for key := range arguments {
		if !allowed[key] {
			return RouterAction{}, errors.New("router action has extra fields")
		}
	}
	return a, nil
}
func searchMode(value string) bool {
	return value == "lexical" || value == "semantic" || value == "hybrid"
}
