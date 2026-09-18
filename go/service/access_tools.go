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

type managedAccessStore interface {
	ManagedIdentity
	List(context.Context) ([]access.ManagedPrincipal, error)
	Audit(context.Context, int) ([]access.AuditEntry, error)
	Create(context.Context, string, string, []string, []string, []string, *string) (access.ManagedPrincipal, string, error)
	Update(context.Context, string, string, []string, []string, []string) (access.ManagedPrincipal, error)
	Rename(context.Context, string, string, string) (access.ManagedPrincipal, error)
	SetEnabled(context.Context, string, string, bool) (access.ManagedPrincipal, error)
	Rotate(context.Context, string, string, *string) (string, error)
	Revoke(context.Context, string, string) (access.ManagedPrincipal, error)
	Delete(context.Context, string, string) (access.ManagedPrincipal, error)
}

type accessToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations"`
}

// RegisterAccessTools installs all managed-administration tools from the exact
// generated Python metadata. Visibility and calls independently require admin.
func (j *Jobs) RegisterAccessTools(server *umcp.Server) error {
	return j.registerAccessTools(server, accessToolDefinitions, 10)
}

// RegisterAccessReadTools retains the bounded two-tool composition API.
func (j *Jobs) RegisterAccessReadTools(server *umcp.Server) error {
	return j.registerAccessTools(server, accessToolDefinitions, 2)
}

func (j *Jobs) registerAccessReadTools(server *umcp.Server, raw []byte) error {
	return j.registerAccessTools(server, raw, 2)
}

func (j *Jobs) registerAccessTools(server *umcp.Server, raw []byte, count int) error {
	if server == nil {
		return errors.New("access tools require a uMCP server")
	}
	store, ok := j.Identity.managed.(managedAccessStore)
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
		if len(tool.Name) < len("access_") || tool.Name[:len("access_")] != "access_" {
			return true
		}
		_, err := j.accessAdmin(ctx)
		return err == nil
	}
	if count > len(definitions) {
		return errors.New("incomplete access tool metadata")
	}
	for _, definition := range definitions[:count] {
		definition := definition
		parameters := accessParameters(definition.Name)
		// Registration cannot fail: every generated definition receives a handler.
		_ = server.Tools.Register(umcp.Tool{Name: definition.Name, Description: definition.Description, InputSchema: definition.Schema, Annotations: definition.Annotations, Parameters: parameters, Call: func(ctx context.Context, args map[string]any) (any, error) {
			actor, err := j.accessAdmin(ctx)
			if err != nil {
				return nil, err
			}
			result, err := callAccessTool(ctx, store, actor.Name, definition.Name, args)
			if err != nil {
				return nil, err
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
