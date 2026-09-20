// Package umcp provides a pure-Go implementation of the pinned Python uMCP
// protocol behavior, including dynamic registries, sessions and all transports.
package umcp

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
)

// ProtocolVersions returns the source preference order without mutable globals.
func ProtocolVersions() []string { return []string{"2025-03-26", "2024-11-05"} }

// ExactOrFallback mirrors the reference helper without validating preferred.
func ExactOrFallback(accepted *string, preferred string) string {
	if accepted != nil {
		for _, version := range ProtocolVersions() {
			if *accepted == version {
				return version
			}
		}
	}
	return preferred
}

// ProtocolVersionError preserves the distinction between missing and unsupported.
func ProtocolVersionError(version *string) map[string]any {
	result := map[string]any{"error": "missing MCP-Protocol-Version header", "expected": ProtocolVersions()[0], "supported": ProtocolVersions()}
	if version != nil {
		result["error"] = "unsupported MCP-Protocol-Version header"
		result["received"] = *version
	}
	return result
}

var integer = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

func decode(raw json.RawMessage) (any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber() // Python accepts arbitrary-size integer IDs; do not round them.
	var value any
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, false
	}
	return value, true
}

func validID(value any) bool {
	switch v := value.(type) {
	case nil, string:
		return true
	case json.Number:
		return integer.MatchString(v.String())
	default:
		return false
	}
}

// ValidJSONRPCID accepts string/integer/null, but not bool, floats or containers.
func ValidJSONRPCID(raw json.RawMessage) bool {
	value, ok := decode(raw)
	return ok && validID(value)
}

// ValidJSONRPCResponse mirrors is_valid_jsonrpc_response, including its limited
// scope: the source helper does not itself enforce a jsonrpc version member.
func ValidJSONRPCResponse(raw json.RawMessage) bool {
	value, ok := decode(raw)
	if !ok {
		return false
	}
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	id, hasID := object["id"]
	if !hasID || !validID(id) {
		return false
	}
	_, result := object["result"]
	errorValue, failure := object["error"]
	if result == failure {
		return false
	}
	if !failure {
		return true
	}
	err, ok := errorValue.(map[string]any)
	if !ok {
		return false
	}
	code, ok := err["code"].(json.Number)
	_, message := err["message"].(string)
	return ok && integer.MatchString(code.String()) && message
}
