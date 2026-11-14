package service

import (
	_ "embed"
	"encoding/json"
)

// Stable service envelopes, including ordinary domain-error responses. These
// schemas describe the complete structuredContent, not only its data member.
// Dynamic execute/route and mutation payloads stay schema-less in this patch.
//
//go:embed output_schemas.json
var outputSchemaDefinitions []byte

var stableOutputSchemas = loadOutputSchemas(outputSchemaDefinitions)

func loadOutputSchemas(raw []byte) map[string]map[string]any {
	var schemas map[string]map[string]any
	// Embedded JSON is validated by contract tests. No runtime I/O or model work.
	if err := json.Unmarshal(raw, &schemas); err != nil {
		panic("invalid embedded MCP output schemas: " + err.Error())
	}
	return schemas
}

func toolOutputSchema(name string) map[string]any { return stableOutputSchemas[name] }
