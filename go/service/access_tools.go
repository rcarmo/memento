package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/umcp"
)

//go:embed access_tools.json
var accessToolDefinitions []byte

type managedAccessReader interface {
	ManagedIdentity
	List(context.Context) ([]access.ManagedPrincipal, error)
	Audit(context.Context, int) ([]access.AuditEntry, error)
}

type accessToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations"`
}

// RegisterAccessReadTools installs the two read-only managed-administration
// tools. Mutations are registered only once their credential contracts land.
func (j *Jobs) RegisterAccessReadTools(server *umcp.Server) error {
	return j.registerAccessReadTools(server, accessToolDefinitions)
}

func (j *Jobs) registerAccessReadTools(server *umcp.Server, raw []byte) error {
	if server == nil {
		return errors.New("access tools require a uMCP server")
	}
	store, ok := j.Identity.managed.(managedAccessReader)
	if !ok {
		return errors.New("access management is not configured")
	}
	var definitions []accessToolDefinition
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&definitions); err != nil {
		return err
	}
	previous := server.Tools.Visible
	server.Tools.Visible = func(ctx context.Context, tool umcp.Tool) bool {
		if previous != nil && !previous(ctx, tool) {
			return false
		}
		if tool.Name != "access_principal_list" && tool.Name != "access_audit_list" {
			return true
		}
		_, err := j.accessAdmin(ctx)
		return err == nil
	}
	for _, definition := range definitions[:2] {
		definition := definition
		parameters := []umcp.Parameter{}
		if definition.Name == "access_audit_list" {
			parameters = append(parameters, umcp.Parameter{Name: "limit", Types: []umcp.ParamType{umcp.IntegerParam}, HasDefault: true, Default: 50})
		}
		// Registration cannot fail: every generated definition receives a handler.
		_ = server.Tools.Register(umcp.Tool{Name: definition.Name, Description: definition.Description, InputSchema: definition.Schema, Annotations: definition.Annotations, Parameters: parameters, Call: func(ctx context.Context, args map[string]any) (any, error) {
			if _, err := j.accessAdmin(ctx); err != nil {
				return nil, err
			}
			var result map[string]any
			if definition.Name == "access_principal_list" {
				items, err := store.List(ctx)
				if err != nil {
					return nil, err
				}
				result = map[string]any{"principals": items}
			} else {
				limit, err := integerArgument(args["limit"])
				if err != nil {
					return nil, err
				}
				items, err := store.Audit(ctx, limit)
				if err != nil {
					return nil, err
				}
				result = map[string]any{"events": items}
			}
			return MCPEnvelope(result)
		}})
	}
	return nil
}

func (j *Jobs) accessAdmin(ctx context.Context) (access.Principal, error) {
	principal, _, err := j.Identity.requestPrincipal(ctx)
	if err != nil {
		return access.Principal{}, err
	}
	for _, role := range principal.Roles {
		if role == "admin" {
			return principal, nil
		}
	}
	return access.Principal{}, errors.New("admin access is required")
}

func integerArgument(value any) (int, error) {
	switch value := value.(type) {
	case int:
		return value, nil
	case json.Number:
		n, err := value.Int64()
		if err == nil {
			return int(n), nil
		}
	}
	return 0, umcp.ArgumentError("limit must be an integer")
}
