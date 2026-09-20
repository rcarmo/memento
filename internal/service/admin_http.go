package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

const adminRoutePrefix = "/admin"

var adminHeaders = [][2]string{{"Cache-Control", "no-store"}, {"X-Content-Type-Options", "nosniff"}}

type AdminHTTP struct {
	Store                 managedAccessStore
	ProtectedReadPrefixes []string
}

func adminJSON(payload any, status int) (*umcp.HTTPResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	mime := "application/json; charset=utf-8"
	return &umcp.HTTPResponse{Status: status, Body: body, ContentType: &mime, Headers: adminHeaders}, nil
}
func adminError(message string, status int) (*umcp.HTTPResponse, error) {
	return adminJSON(map[string]any{"error": message}, status)
}
func adminAccessError(message string) error { return &access.AccessError{Message: message} }
func adminCallError(err error) (*umcp.HTTPResponse, error) {
	if err == nil {
		return nil, nil
	}
	var accessErr *access.AccessError
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &accessErr) || errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return adminError(err.Error(), 400)
	}
	return nil, err
}
func adminString(payload map[string]any, field string) string {
	if value := payload[field]; value != nil {
		return fmt.Sprint(value)
	}
	return ""
}
func adminStrings(payload map[string]any, field string) ([]string, error) {
	value := payload[field]
	if value == nil {
		return nil, nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, adminAccessError(field + " must be an array of strings")
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		item, ok := value.(string)
		if !ok {
			return nil, adminAccessError(field + " must be an array of strings")
		}
		out = append(out, item)
	}
	return out, nil
}
func adminObject(body []byte) (map[string]any, error) {
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	object, ok := payload.(map[string]any)
	if !ok {
		return nil, adminAccessError("request body must be an object")
	}
	return object, nil
}
func (h AdminHTTP) principal(item access.ManagedPrincipal) map[string]any {
	payload := map[string]any{
		"name":           item.Name,
		"roles":          item.Roles,
		"read_prefixes":  item.ReadPrefixes,
		"write_prefixes": item.WritePrefixes,
		"enabled":        item.Enabled,
		"revoked":        item.Revoked,
		"deleted":        item.Deleted,
		"updated_at":     item.UpdatedAt,
	}
	if warning := access.BroadReadGrantWarning(item.Roles, item.ReadPrefixes, h.ProtectedReadPrefixes); warning != nil {
		payload["warnings"] = []string{*warning}
	}
	return payload
}
func (h AdminHTTP) authenticate(ctx context.Context, headers map[string]string) (*access.Principal, error) {
	value := headers["authorization"]
	if !strings.HasPrefix(value, "Bearer ") || h.Store == nil {
		return nil, nil
	}
	return h.Store.Authenticate(ctx, strings.TrimPrefix(value, "Bearer "))
}
func (h AdminHTTP) principalAction(ctx context.Context, actor, path string, payload map[string]any) (any, error) {
	parts := strings.Split(strings.TrimPrefix(path, adminRoutePrefix+"/api/principals/"), "/")
	name, action := parts[0], "update"
	if len(parts) > 1 {
		action = parts[1]
	}
	switch action {
	case "update":
		roles, err := adminStrings(payload, "roles")
		if err != nil {
			return nil, err
		}
		reads, err := adminStrings(payload, "read_prefixes")
		if err != nil {
			return nil, err
		}
		writes, err := adminStrings(payload, "write_prefixes")
		if err != nil {
			return nil, err
		}
		item, err := h.Store.Update(ctx, actor, name, roles, reads, writes)
		if err != nil {
			return nil, err
		}
		return map[string]any{"principal": item}, nil
	case "rename":
		item, err := h.Store.Rename(ctx, actor, name, adminString(payload, "new_name"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"principal": item}, nil
	case "disable", "enable":
		item, err := h.Store.SetEnabled(ctx, actor, name, action == "enable")
		if err != nil {
			return nil, err
		}
		return map[string]any{"principal": item}, nil
	case "rotate":
		credential, err := h.Store.Rotate(ctx, actor, name, nil)
		if err != nil {
			return nil, err
		}
		return map[string]any{"name": name, "credential": credential}, nil
	case "revoke":
		item, err := h.Store.Revoke(ctx, actor, name)
		if err != nil {
			return nil, err
		}
		return map[string]any{"principal": item}, nil
	case "delete":
		item, err := h.Store.Delete(ctx, actor, name)
		if err != nil {
			return nil, err
		}
		return map[string]any{"principal": item}, nil
	default:
		return map[string]any{"error": "not found"}, nil
	}
}
func (h AdminHTTP) Handle(ctx context.Context, method, path string, headers map[string]string, body []byte, _ string) (*umcp.HTTPResponse, error) {
	if path != adminRoutePrefix && !strings.HasPrefix(path, adminRoutePrefix+"/") {
		return nil, nil
	}
	if h.Store == nil {
		return adminError("access management is not configured", 503)
	}
	switch {
	case method == "GET" && (path == adminRoutePrefix || path == adminRoutePrefix+"/"):
		return adminStaticResponse("index.html"), nil
	case method == "GET" && path == adminRoutePrefix+"/app.js":
		return adminStaticResponse("app.js"), nil
	case !strings.HasPrefix(path, adminRoutePrefix+"/api/"):
		return adminError("not found", 404)
	}
	actor, err := h.authenticate(ctx, headers)
	if err != nil {
		return nil, err
	}
	if actor == nil || !slices.Contains(actor.Roles, "admin") {
		return adminError("admin bearer credential required", 401)
	}
	payload, err := adminObject(body)
	if err != nil {
		return adminCallError(err)
	}
	switch {
	case method == "GET" && path == adminRoutePrefix+"/api/principals":
		items, err := h.Store.List(ctx)
		if err != nil {
			return adminCallError(err)
		}
		principals := make([]map[string]any, 0, len(items))
		for _, item := range items {
			principals = append(principals, h.principal(item))
		}
		return adminJSON(map[string]any{"principals": principals}, 200)
	case method == "GET" && path == adminRoutePrefix+"/api/activity":
		items, err := h.Store.Audit(ctx, 50)
		if err != nil {
			return adminCallError(err)
		}
		return adminJSON(map[string]any{"events": items}, 200)
	case method == "POST" && path == adminRoutePrefix+"/api/principals":
		roles, err := adminStrings(payload, "roles")
		if err != nil {
			return adminCallError(err)
		}
		reads, err := adminStrings(payload, "read_prefixes")
		if err != nil {
			return adminCallError(err)
		}
		writes, err := adminStrings(payload, "write_prefixes")
		if err != nil {
			return adminCallError(err)
		}
		item, credential, err := h.Store.Create(ctx, actor.Name, adminString(payload, "name"), roles, reads, writes, nil)
		if err != nil {
			return adminCallError(err)
		}
		return adminJSON(map[string]any{"principal": h.principal(item), "credential": credential}, 201)
	case method == "POST" && strings.HasPrefix(path, adminRoutePrefix+"/api/principals/"):
		result, err := h.principalAction(ctx, actor.Name, path, payload)
		if err != nil {
			return adminCallError(err)
		}
		if response, ok := result.(map[string]any); ok && response["error"] != nil {
			return adminJSON(response, 404)
		}
		return adminJSON(result, 200)
	default:
		return adminError("not found", 404)
	}
}
