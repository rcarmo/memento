package umcp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"
)

// StatelessHTTPVersion is the first MCP revision with per-request HTTP
// metadata and no initialize handshake.
const StatelessHTTPVersion = "2026-07-28"

const protocolVersionMeta = "io.modelcontextprotocol/protocolVersion"

func (h *StreamableHTTP) modernRequest(r *http.Request, params map[string]any) bool {
	if !h.server.SupportsProtocolVersion(StatelessHTTPVersion) {
		return false
	}
	header := r.Header.Get("Mcp-Protocol-Version")
	meta, _ := params["_meta"].(map[string]any)
	version, _ := meta[protocolVersionMeta].(string)
	// Legacy requests keep their transport contract. A modern body version
	// cannot be used to bypass validation by supplying a legacy header.
	return version == StatelessHTTPVersion || header == StatelessHTTPVersion || header != "" && !h.server.SupportsProtocolVersion(header)
}

func (h *StreamableHTTP) validateModernRequest(r *http.Request, request, params map[string]any) *RPCError {
	header := r.Header.Get("Mcp-Protocol-Version")
	meta, _ := params["_meta"].(map[string]any)
	version, _ := meta[protocolVersionMeta].(string)
	mismatch := func(key string) *RPCError {
		return &RPCError{Code: -32020, Message: "Missing or mismatched " + key + " request metadata"}
	}
	if header == "" || header != version {
		return mismatch("Mcp-Protocol-Version")
	}
	counts, _ := r.Context().Value(asyncHeaderCountsKey{}).(map[string]int)
	for _, key := range []string{"Mcp-Protocol-Version", "Mcp-Method", "Mcp-Name"} {
		if parsed := syncRequest(r); parsed != nil && parsed.Counts[strings.ToLower(key)] > 1 {
			return mismatch(key)
		}
		if len(r.Header.Values(key)) > 1 || counts[strings.ToLower(key)] > 1 {
			return mismatch(key)
		}
	}
	if !h.server.SupportsProtocolVersion(header) {
		return &RPCError{Code: -32022, Message: "Unsupported protocol version", Data: map[string]any{"requested": header, "supported": append([]string{StatelessHTTPVersion}, ProtocolVersions()...)}}
	}
	method, _ := request["method"].(string)
	if method == "" || r.Header.Get("Mcp-Method") != method {
		return mismatch("Mcp-Method")
	}
	key := ""
	switch method {
	case "tools/call", "prompts/get":
		key = "name"
	case "resources/read":
		key = "uri"
	}
	if key != "" {
		name, _ := params[key].(string)
		value, ok := decodeMCPName(r.Header.Get("Mcp-Name"))
		if !ok || name == "" || value != name {
			return mismatch("Mcp-Name")
		}
	} else if len(r.Header.Values("Mcp-Name")) != 0 {
		return mismatch("Mcp-Name")
	}
	return nil
}

func decodeMCPName(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	if encoded, ok := strings.CutPrefix(value, "=?base64?"); ok {
		if encoded, ok := strings.CutSuffix(encoded, "?="); ok {
			raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
			return string(raw), err == nil && utf8.Valid(raw)
		}
	}
	for i := range len(value) {
		if value[i] < 0x20 || value[i] > 0x7e {
			return "", false
		}
	}
	return value, true
}

func modernFailure(request map[string]any, err *RPCError) *Response {
	var id json.RawMessage
	// JSON-RPC error IDs may be null. Keep numbers in their decoded spelling.
	if value := request["id"]; validID(value) {
		id, _ = json.Marshal(value)
	}
	return &Response{ID: id, Error: err}
}

func modernMethodSupported(method any) bool {
	switch method {
	case "server/discover", "ping", "tools/list", "tools/call":
		return true
	}
	return false
}
