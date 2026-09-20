package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/umcp"
)

func catalogResource(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	parsed, _ := pyjson.Parse(string(raw)) // json.Marshal emits valid JSON
	text, err := pyjson.Dumps(parsed)
	if err != nil {
		return nil, err
	}
	return map[string]any{"mimeType": "application/json", "text": text}, nil
}
func (c *Catalog) registerCatalogResources(server *umcp.Server, handlers map[string]CatalogHandler) error {
	for _, meta := range c.source.Resources {
		uri := meta.URI
		// Fixed generated URIs and nonnil closures cannot fail registry validation.
		_ = server.Resources.Register(umcp.Resource{URI: uri, Name: meta.Name, Title: meta.Title, Description: meta.Description, MIME: meta.MIME, Read: func(ctx context.Context, _ map[string]any) (any, error) {
			if uri == "memory://catalog" {
				return catalogResource(c.Payload())
			}
			name := "memory_help"
			if uri == "memory://status" {
				name = "memory_status"
			}
			value, err := handlers[name](ctx, map[string]any{})
			if err != nil {
				return nil, err
			}
			return catalogResource(value)
		}})
	}
	for _, meta := range c.source.Templates {
		workflow := meta.Template == "memory://workflow/{goal}"
		name := "operation"
		values := []any{}
		enums := c.source.OperationValues
		if workflow {
			name = "goal"
			enums = c.source.WorkflowValues
		}
		for _, value := range enums {
			values = append(values, value)
		}
		_ = server.Resources.Register(umcp.Resource{URI: meta.Template, Name: meta.Name, Title: meta.Title, Description: meta.Description, MIME: meta.MIME, Template: true, Parameters: []umcp.Parameter{{Name: name, Types: []umcp.ParamType{umcp.StringParam}, Enum: values}}, Read: func(_ context.Context, args map[string]any) (any, error) {
			key, _ := args[name].(string)
			var value map[string]any
			var err error
			if workflow {
				value, err = c.Workflow(key)
			} else {
				value, err = c.Operation(key)
			}
			if err != nil {
				return nil, err
			}
			return catalogResource(value)
		}})
	}
	for _, meta := range c.source.Prompts {
		_ = server.Prompts.Register(umcp.Prompt{Name: meta.Name, Description: meta.Description, CallableDescription: c.source.PromptDescription, InputSchema: meta.Schema, Parameters: []umcp.Parameter{{Name: "target_path", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: "/skills/example.md"}, {Name: "asset_kind", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: "skill"}, {Name: "version", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: "1.0.0"}}, Call: func(_ context.Context, args map[string]any) (any, error) {
			// Typed prompt inputs are implemented; permissive Python str() for
			// non-string values remains a documented compatibility edge.
			return assetPublicationPrompt(args)
		}})
	}
	return nil
}
func assetPublicationPrompt(args map[string]any) (string, error) {
	a := toolArguments{values: args}
	path, kind, version := a.text("target_path"), a.text("asset_kind"), a.text("version")
	if a.err != nil {
		return "", a.err
	}
	change, err := pyjson.Dumps(map[string]any{"kind": "attach_asset_pack", "path": path, "asset_kind": kind, "version": version, "zip_base64": "<base64 ZIP bytes>"})
	if err != nil {
		return "", err
	}
	return strings.Join([]string{
		"Publish and verify a versioned asset pack with one authenticated curator profile.", "Target concept: " + path, "Asset kind: " + kind, "Version: " + version,
		"1. Read memory://workflow/asset_pack and memory://catalog/propose.",
		"2. For a skill pack, use canonical UTF-8/LF text with no trailing whitespace or final newline; make the concept body and ZIP-root SKILL.md byte-identical, then base64-encode the ZIP.",
		"3. Submit a propose operation (directly or through memory_execute) using the current repository revision and this change:", change,
		"4. Read the proposal and verify its generated manifest, SHA-256, target path, asset kind, and version.",
		"5. With the same authenticated curator profile, approve and apply using a fresh expected revision and durable idempotency key.",
		"6. Retrieve the accepted version with memory_asset_get and verify the returned manifest, SHA-256, and decoded ZIP bytes.",
		"Use memory_asset_stage_begin, raw HTTP upload, memory_asset_stage_status, and staged_asset_id only when the MCP request would exceed the configured ceiling or the client deliberately uses raw binary HTTP upload."}, "\n"), nil
}
