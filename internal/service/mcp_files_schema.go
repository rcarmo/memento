package service

import (
	"context"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/umcp"
)

func fileTools() []umcp.Tool {
	text := func() map[string]any { return map[string]any{"type": "string"} }
	object := func(properties map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	digest := object(map[string]any{"algorithm": map[string]any{"type": "string", "const": "sha-256"}, "value": text()}, "algorithm", "value")
	file := object(map[string]any{"id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}, "name": text(), "mimeType": map[string]any{"type": "string", "const": "application/zip"}, "size": map[string]any{"type": "integer", "minimum": 1, "maximum": assets.MaxZIPBytes}, "digest": digest}, "id", "mimeType", "size", "digest")
	file["x-mcp-file"] = map[string]any{"accept": []string{"application/zip"}, "maxSize": assets.MaxZIPBytes}
	// No inline data is accepted or advertised. Keeping these tools separate leaves
	// the historical staging/get signatures and executor surface unchanged.
	fallback := func(context.Context, map[string]any) (any, error) {
		return nil, umcp.ArgumentError("File tools require the negotiated protocol adapter")
	}
	return []umcp.Tool{
		{Name: fileUploadTool, Description: "Experimental files extension: stage a validated ZIP asset pack. Does not propose or apply changes.", InputSchema: object(map[string]any{"asset_kind": text(), "version": text(), "idempotency_key": text(), "file": file}, "asset_kind", "version", "idempotency_key", "file"), Annotations: map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}, Call: fallback},
		{Name: fileDownloadTool, Description: "Experimental files extension: download an accepted ZIP asset version without placing bytes in model context.", InputSchema: object(map[string]any{"id_or_path": text(), "asset_kind": text(), "version": text()}, "id_or_path", "asset_kind"), Annotations: map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}, Call: fallback},
	}
}
