package service

import (
	"context"
	"errors"
	"strings"

	"github.com/rcarmo/memento/go/umcp"
)

func accessParameters(name string) []umcp.Parameter {
	p := func(name string, kind umcp.ParamType, optional bool) umcp.Parameter {
		return umcp.Parameter{Name: name, Types: []umcp.ParamType{kind}, HasDefault: optional}
	}
	switch name {
	case "access_principal_list":
		return nil
	case "access_audit_list":
		return []umcp.Parameter{{Name: "limit", Types: []umcp.ParamType{umcp.IntegerParam}, HasDefault: true, Default: 50}}
	case "access_principal_create":
		return []umcp.Parameter{p("name", umcp.StringParam, true), p("roles", umcp.ArrayParam, true), p("read_prefixes", umcp.ArrayParam, true), p("write_prefixes", umcp.ArrayParam, true), p("idempotency_key", umcp.StringParam, true)}
	case "access_principal_update":
		return []umcp.Parameter{p("name", umcp.StringParam, false), p("roles", umcp.ArrayParam, false), p("read_prefixes", umcp.ArrayParam, false), p("write_prefixes", umcp.ArrayParam, false)}
	case "access_principal_rename":
		return []umcp.Parameter{p("name", umcp.StringParam, false), p("new_name", umcp.StringParam, false)}
	case "access_credential_rotate":
		return []umcp.Parameter{p("name", umcp.StringParam, false), p("idempotency_key", umcp.StringParam, false)}
	default:
		return []umcp.Parameter{p("name", umcp.StringParam, false)}
	}
}

func callAccessTool(ctx context.Context, store managedAccessStore, actor, name string, args map[string]any) (map[string]any, error) {
	switch name {
	case "access_principal_list":
		items, err := store.List(ctx)
		return map[string]any{"principals": items}, err
	case "access_audit_list":
		limit, err := integerArgument(args["limit"])
		if err != nil {
			return nil, err
		}
		items, err := store.Audit(ctx, limit)
		return map[string]any{"events": items}, err
	case "access_principal_create":
		n, r, reads, writes, key, err := requiredCreate(args)
		if err != nil {
			return nil, err
		}
		item, credential, err := store.Create(ctx, actor, n, r, reads, writes, &key)
		return map[string]any{"principal": item, "credential": credential}, err
	case "access_principal_update":
		item, err := store.Update(ctx, actor, args["name"].(string), mustStrings(args["roles"]), mustStrings(args["read_prefixes"]), mustStrings(args["write_prefixes"]))
		return map[string]any{"principal": item}, err
	case "access_principal_rename":
		item, err := store.Rename(ctx, actor, args["name"].(string), args["new_name"].(string))
		return map[string]any{"principal": item}, err
	case "access_principal_disable", "access_principal_enable":
		item, err := store.SetEnabled(ctx, actor, args["name"].(string), name == "access_principal_enable")
		return map[string]any{"principal": item}, err
	case "access_credential_rotate":
		key := args["idempotency_key"].(string)
		credential, err := store.Rotate(ctx, actor, args["name"].(string), &key)
		return map[string]any{"name": args["name"], "credential": credential}, err
	case "access_principal_revoke":
		item, err := store.Revoke(ctx, actor, args["name"].(string))
		return map[string]any{"principal": item}, err
	case "access_principal_delete":
		item, err := store.Delete(ctx, actor, args["name"].(string))
		return map[string]any{"principal": item}, err
	}
	return nil, errors.New("unsupported access tool: " + name)
}

func mustStrings(value any) []string {
	values, ok := stringsArgument(value)
	if !ok {
		panic("uMCP array coercion invariant")
	}
	return values
}
func stringsArgument(value any) ([]string, bool) {
	values, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i], ok = v.(string)
		if !ok {
			return nil, false
		}
	}
	return out, true
}
func requiredCreate(args map[string]any) (string, []string, []string, []string, string, error) {
	fields := []string{"name", "roles", "read_prefixes", "write_prefixes", "idempotency_key"}
	missing := []string{}
	for _, field := range fields {
		if args[field] == nil {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return "", nil, nil, nil, "", umcp.ArgumentError("access_principal_create requires MCP params.arguments fields: " + strings.Join(missing, ", ") + ". Prefixes are namespace paths such as '/skills/', not memory:// resource URIs.")
	}
	name, ok := args["name"].(string)
	if !ok {
		return "", nil, nil, nil, "", umcp.ArgumentError("name must be a string")
	}
	roles, ok := stringsArgument(args["roles"])
	if !ok {
		return "", nil, nil, nil, "", umcp.ArgumentError("roles must be an array of strings")
	}
	reads, ok := stringsArgument(args["read_prefixes"])
	if !ok {
		return "", nil, nil, nil, "", umcp.ArgumentError("read_prefixes must be an array of strings")
	}
	writes, ok := stringsArgument(args["write_prefixes"])
	if !ok {
		return "", nil, nil, nil, "", umcp.ArgumentError("write_prefixes must be an array of strings")
	}
	key, ok := args["idempotency_key"].(string)
	if !ok {
		return "", nil, nil, nil, "", umcp.ArgumentError("idempotency_key must be a string")
	}
	return name, roles, reads, writes, key, nil
}
