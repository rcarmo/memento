package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/umcp"
)

type fileHarness struct {
	t      *testing.T
	jobs   *Jobs
	base   string
	files  *FileTransfers
	server *umcp.Server
	host   *httptest.Server
}

func newFileHarness(t *testing.T, async ...bool) *fileHarness {
	t.Helper()
	j, base := jobsTest(t)
	authorization := access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{}}
	var tokens []BearerPrincipal
	for _, name := range []string{"actor", "other", "reader"} {
		roles := []string{"reader", "proposer", "curator"}
		if name == "reader" {
			roles = []string{"reader"}
		}
		authorization.Principals[name] = access.NamespacePolicy{Roles: roles, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}
		tokens = append(tokens, BearerPrincipal{Token: name, Principal: access.Principal{Name: name, Roles: roles}})
	}
	var err error
	j.Identity, err = NewIdentity(tokens, authorization, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	j.Controls.Staging = &assets.StagingStore{DB: j.Controls.Queue.Proposals.DB, Now: j.Controls.Queue.Now}
	server := umcp.NewServer("files-test")
	server.SetNotificationOutput(nil)
	if err = j.RegisterAssetReadProposalTools(server, nil); err != nil {
		t.Fatal(err)
	}
	f, err := NewFileTransfers(context.Background(), j, server, true)
	if err != nil {
		t.Fatal(err)
	}
	hooks := j.Identity.HTTPHooks()
	hooks.BodyRoute = f.HTTPRoute()
	options := umcp.DefaultHTTPOptions()
	options.AsyncReference = len(async) > 0 && async[0]
	options.MaxRequestBytes = 72 << 20
	transport, err := umcp.NewStreamableHTTP(server, options, hooks)
	if err != nil {
		t.Fatal(err)
	}
	host := httptest.NewServer(transport)
	host.Client().Timeout = 5 * time.Second
	t.Cleanup(func() { host.Close(); transport.Close(); j.Workers.Drain(context.Background()) })
	return &fileHarness{t, j, base, f, server, host}
}
func (h *fileHarness) request(method, path, principal string, body []byte, headers map[string]string) (int, []byte) {
	h.t.Helper()
	req, err := http.NewRequest(method, h.host.URL+path, bytes.NewReader(body))
	if err != nil {
		h.t.Fatal(err)
	}
	if principal != "" {
		req.Header.Set("Authorization", "Bearer "+principal)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", fileProtocol)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	r, err := h.host.Client().Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	return r.StatusCode, raw
}
func (h *fileHarness) rpc(method, principal string, params map[string]any) map[string]any {
	h.t.Helper()
	raw, headers := modernFileRequest(method, params)
	status, body := h.request("POST", "/mcp", principal, raw, headers)
	if status != 200 {
		h.t.Fatalf("HTTP%d %s", status, body)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		h.t.Fatalf("JSON %v %s", err, body)
	}
	return result
}
func modernFileRequest(method string, params map[string]any) ([]byte, map[string]string) {
	wire := make(map[string]any, len(params)+1)
	for key, value := range params {
		wire[key] = value
	}
	metadata := map[string]any{"io.modelcontextprotocol/protocolVersion": fileProtocol}
	if meta, ok := params["_meta"].(map[string]any); ok {
		for key, value := range meta {
			metadata[key] = value
		}
	}
	wire["_meta"] = metadata
	headers := map[string]string{"Content-Type": "application/json", "Mcp-Method": method}
	if method == "tools/call" {
		if name, ok := params["name"].(string); ok {
			headers["Mcp-Name"] = name
		}
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": wire})
	return raw, headers
}

func fileMeta(upload, download bool) map[string]any {
	settings := map[string]any{}
	if upload {
		settings["upload"] = map[string]any{}
	}
	if download {
		settings["download"] = map[string]any{}
	}
	return map[string]any{"io.modelcontextprotocol/clientCapabilities": map[string]any{"extensions": map[string]any{fileExtension: settings}}}
}
func fileArguments(raw []byte, key string) map[string]any {
	sum := sha256.Sum256(raw)
	return map[string]any{"asset_kind": "docs", "version": "1.0.0", "idempotency_key": key, "file": map[string]any{"id": "file-1", "name": "../ignored.zip", "mimeType": "application/zip", "size": json.Number(fmt.Sprint(len(raw))), "digest": map[string]any{"algorithm": "sha-256", "value": base64.RawURLEncoding.EncodeToString(sum[:])}}}
}
func fileParams(name string, args map[string]any) map[string]any {
	return map[string]any{"name": name, "arguments": args, "_meta": fileMeta(true, true)}
}
func fileResult(t *testing.T, r map[string]any) map[string]any {
	t.Helper()
	if r["error"] != nil {
		t.Fatal(r)
	}
	return r["result"].(map[string]any)
}
func fileRPCError(t *testing.T, r map[string]any, code float64) {
	t.Helper()
	e, ok := r["error"].(map[string]any)
	if !ok || e["code"] != code {
		t.Fatalf("want RPC %v: %v", code, r)
	}
}
func (h *fileHarness) begin(args map[string]any) (string, map[string]string) {
	h.t.Helper()
	r := fileResult(h.t, h.rpc("tools/call", "actor", fileParams(fileUploadTool, args)))
	if r["resultType"] != "upload_required" {
		h.t.Fatal(r)
	}
	target := r["uploads"].([]any)[0].(map[string]any)
	if target["path"] != "/files/upload" || target["credentials"] != "include" || target["method"] != "POST" {
		h.t.Fatal(target)
	}
	headers := map[string]string{}
	for key, value := range target["headers"].(map[string]any) {
		headers[key] = value.(string)
	}
	return r["requestState"].(string), headers
}
func TestMCPFilesNegotiationAndLegacy(t *testing.T) {
	h := newFileHarness(t)
	discovered := fileResult(t, h.rpc("server/discover", "actor", map[string]any{}))
	extensions := discovered["capabilities"].(map[string]any)["extensions"].(map[string]any)
	if extensions[fileExtension].(map[string]any)["maxInlineBytes"] != float64(0) {
		t.Fatal(discovered)
	}
	count := func(p map[string]any) int {
		result := fileResult(t, h.rpc("tools/list", "actor", p))
		count := 0
		for _, item := range result["tools"].([]any) {
			name := item.(map[string]any)["name"]
			if name == fileUploadTool || name == fileDownloadTool {
				count++
			}
		}
		return count
	}
	if count(map[string]any{}) != 0 || count(map[string]any{"_meta": fileMeta(true, true)}) != 2 || count(map[string]any{"_meta": fileMeta(false, false)}) != 0 {
		t.Fatal("capability discovery")
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{"_meta": fileMeta(true, true)}})
	status, body := h.request("POST", "/mcp", "actor", raw, map[string]string{"Content-Type": "application/json", "Mcp-Protocol-Version": "2025-03-26"})
	if status != 200 || bytes.Contains(body, []byte(`"name":"memory_asset_upload"`)) {
		t.Fatal(status, string(body))
	}
	args := fileArguments(submitZIP(t), "missing-cap")
	p := fileParams(fileUploadTool, args)
	delete(p, "_meta")
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32021)
	p = fileParams(fileDownloadTool, map[string]any{"id_or_path": "/a.md", "asset_kind": "docs"})
	delete(p, "_meta")
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32021)
	p = fileParams(fileUploadTool, args)
	r := fileResult(t, h.rpc("tools/call", "reader", p))
	if r["isError"] != true {
		t.Fatal(r)
	}
}
func TestMCPFilesUploadContinuation(t *testing.T) {
	h := newFileHarness(t)
	raw := submitZIP(t)
	args := fileArguments(raw, "upload")
	state, headers := h.begin(args)
	p := fileParams(fileUploadTool, args)
	p["requestState"] = state
	before := fileResult(t, h.rpc("tools/call", "actor", p))
	if before["isError"] != true || before["resultType"] != "complete" {
		t.Fatal(before)
	}
	for _, principal := range []string{"", "other"} {
		status, _ := h.request("POST", "/mcp/files/upload", principal, raw, headers)
		if status == 200 {
			t.Fatal("unbound upload", principal)
		}
	}
	bad := append([]byte{}, raw...)
	bad[0] ^= 1
	status, _ := h.request("POST", "/mcp/files/upload", "actor", bad, headers)
	if status != 400 {
		t.Fatal(status)
	}
	status, _ = h.request("POST", "/mcp/files/upload", "actor", raw[:len(raw)-1], headers)
	if status != 400 {
		t.Fatal(status)
	}
	status, body := h.request("POST", "/mcp/files/upload", "actor", raw, headers)
	if status != 200 || bytes.Contains(body, []byte("staged_asset_id")) {
		t.Fatal(status, string(body))
	}
	status, _ = h.request("POST", "/mcp/files/upload", "actor", raw, headers)
	if status != 200 {
		t.Fatal("idempotent transfer", status)
	}
	result := fileResult(t, h.rpc("tools/call", "actor", p))
	data := result["structuredContent"].(map[string]any)
	id := data["staged_asset_id"]
	if id == nil || data["state"] != "ready" {
		t.Fatal(result)
	}
	repeated := fileResult(t, h.rpc("tools/call", "actor", p))
	if repeated["structuredContent"].(map[string]any)["staged_asset_id"] != id {
		t.Fatal(repeated)
	}
	// JSON Schema instance equality ignores number spelling and member ordering.
	args["file"].(map[string]any)["size"] = json.Number(fmt.Sprint(len(raw)) + ".0")
	if fileResult(t, h.rpc("tools/call", "actor", p))["isError"] == true {
		t.Fatal("numeric equality")
	}
	fileRPCError(t, h.rpc("tools/call", "other", p), -32602)
	changed := fileParams(fileUploadTool, fileArguments(raw, "changed"))
	changed["requestState"] = state
	fileRPCError(t, h.rpc("tools/call", "actor", changed), -32602)
	changed["name"] = "memory_asset_get"
	fileRPCError(t, h.rpc("tools/call", "actor", changed), -32602)
	changed = fileParams(fileUploadTool, args)
	changed["requestState"] = state
	changed["_meta"] = fileMeta(true, false)
	fileRPCError(t, h.rpc("tools/call", "actor", changed), -32602)
	p["requestState"] = state[:42] + "!"
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32602)
	p["requestState"] = state
	args["extra"] = true
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32602)
	delete(args, "extra")
	// Reconstructing the adapter does not invalidate stored continuation state.
	second := umcp.NewServer("second")
	if _, err := NewFileTransfers(context.Background(), h.jobs, second, true); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": p})
	response, err := second.Process(context.Background(), request, umcp.RequestContext{Transport: "streamable-http", ProtocolVersion: fileProtocol, Principal: "actor"})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	encoded, _ := json.Marshal(result)
	if bytes.Contains(encoded, []byte(state)) || bytes.Contains(encoded, []byte(base64.StdEncoding.EncodeToString(raw))) {
		t.Fatal("secret/bytes entered result")
	}
	h.files.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32602)
	status, _ = h.request("POST", "/mcp/files/upload", "actor", raw, headers)
	if status != 404 {
		t.Fatal(status)
	}
}
func TestMCPFilesRejectMetadata(t *testing.T) {
	h := newFileHarness(t)
	raw := submitZIP(t)
	mutations := map[string]func(map[string]any){
		"inline":      func(f map[string]any) { f["data"] = base64.StdEncoding.EncodeToString(raw) },
		"null-inline": func(f map[string]any) { f["data"] = nil },
		"zero":        func(f map[string]any) { f["size"] = json.Number("0") },
		"oversize":    func(f map[string]any) { f["size"] = json.Number("52428801") },
		"exponent":    func(f map[string]any) { f["size"] = json.Number("1e999999999") },
		"fraction":    func(f map[string]any) { f["size"] = json.Number("1.000000000000000000001") },
		"mime":        func(f map[string]any) { f["mimeType"] = "application/pdf" },
		"digest":      func(f map[string]any) { f["digest"].(map[string]any)["value"] = "not-a-sha" },
		"name":        func(f map[string]any) { f["name"] = strings.Repeat("x", 1025) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			args := fileArguments(raw, name)
			mutate(args["file"].(map[string]any))
			r := fileResult(t, h.rpc("tools/call", "actor", fileParams(fileUploadTool, args)))
			if r["isError"] != true {
				t.Fatal(r)
			}
		})
	}
	var count int
	if err := h.jobs.Controls.Staging.DB.QueryRow("SELECT COUNT(*) FROM mcp_file_transfers").Scan(&count); err != nil || count != 0 {
		t.Fatal("metadata rejection created targets", count, err)
	}
}
func (h *fileHarness) acceptPack() (string, map[string]any) {
	args := fileArguments(submitZIP(h.t), "accepted")
	state, headers := h.begin(args)
	status, _ := h.request("POST", "/mcp/files/upload", "actor", submitZIP(h.t), headers)
	if status != 200 {
		h.t.Fatal(status)
	}
	p := fileParams(fileUploadTool, args)
	p["requestState"] = state
	r := fileResult(h.t, h.rpc("tools/call", "actor", p))
	stage := r["structuredContent"].(map[string]any)["staged_asset_id"]
	call := func(name string, args map[string]any) map[string]any {
		r := fileResult(h.t, h.rpc("tools/call", "actor", fileParams(name, args)))
		v := r["structuredContent"].(map[string]any)
		if v["status"] != "success" {
			h.t.Fatal(v)
		}
		return v
	}
	proposed := call("memory_propose", map[string]any{"intent": "file adapter test", "base_revision": h.base, "changes": []any{map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": "docs", "version": "1.0.0", "staged_asset_id": stage}}})
	id := proposed["data"].(map[string]any)["proposal"].(map[string]any)["proposal_id"]
	call("memory_proposal_review", map[string]any{"proposal_id": id, "decision": "approve"})
	call("memory_proposal_apply", map[string]any{"proposal_id": id, "expected_revision": h.base, "idempotency_key": "apply-file"})
	return state, args
}
func TestMCPFilesDownloadAndPolicy(t *testing.T) {
	h := newFileHarness(t)
	h.acceptPack()
	args := map[string]any{"id_or_path": "/a.md", "asset_kind": "docs"}
	r := fileResult(t, h.rpc("tools/call", "actor", fileParams(fileDownloadTool, args)))
	target := r["_meta"].(map[string]any)[fileExtension].(map[string]any)["downloads"].([]any)[0].(map[string]any)
	headers := map[string]string{}
	for key, value := range target["headers"].(map[string]any) {
		headers[key] = value.(string)
	}
	contextOnly, _ := json.Marshal(map[string]any{"content": r["content"], "structuredContent": r["structuredContent"]})
	if bytes.Contains(contextOnly, []byte(headers["X-Memento-File-Token"])) || bytes.Contains(contextOnly, []byte("/files/")) {
		t.Fatal("target in model context")
	}
	status, raw := h.request("GET", "/mcp/files/download", "actor", nil, headers)
	if status != 200 || !bytes.Equal(raw, submitZIP(t)) {
		t.Fatal(status, len(raw))
	}
	status, _ = h.request("GET", "/mcp/files/download", "other", nil, headers)
	if status != 404 {
		t.Fatal(status)
	}
	policy := h.jobs.Identity.authorization.Principals["actor"]
	policy.ReadPrefixes = []string{"/denied/"}
	h.jobs.Identity.authorization.Principals["actor"] = policy
	status, _ = h.request("GET", "/mcp/files/download", "actor", nil, headers)
	if status != 404 {
		t.Fatal("permission not rechecked", status)
	}
	policy.ReadPrefixes = []string{"/"}
	h.jobs.Identity.authorization.Principals["actor"] = policy
	// A changed archive must not be sent even if its metadata is unchanged.
	entries, err := os.ReadDir(h.jobs.Controls.Queue.Paths.CurrentDir + "/.assets")
	if err != nil || len(entries) == 0 {
		t.Fatal(entries, err)
	}
	archive := h.jobs.Controls.Queue.Paths.CurrentDir + "/.assets/" + entries[0].Name() + "/docs/1.0.0.zip"
	corrupt := append([]byte{}, submitZIP(t)...)
	corrupt[len(corrupt)-1] ^= 1
	if err = os.WriteFile(archive, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	status, _ = h.request("GET", "/mcp/files/download", "actor", nil, headers)
	if status != 404 {
		t.Fatal("corruption not caught", status)
	}
}
func FuzzMCPFileMetadata(f *testing.F) {
	valid, _ := json.Marshal(fileArguments([]byte("fixture"), "fuzz"))
	f.Add(string(valid))
	f.Add(`{"asset_kind":"docs","version":"1.0.0","idempotency_key":"a","file":{"id":"f","mimeType":"application/zip","size":1e999999999}}`)
	f.Add(`null`)
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			return
		}
		var args map[string]any
		d := json.NewDecoder(strings.NewReader(raw))
		d.UseNumber()
		if d.Decode(&args) != nil {
			return
		}
		input, canonical, err := parseFileArguments(args)
		if err == nil {
			var normalized map[string]any
			d := json.NewDecoder(strings.NewReader(canonical))
			d.UseNumber()
			if err := d.Decode(&normalized); err != nil {
				t.Fatal(err)
			}
			again, roundtrip, err := parseFileArguments(normalized)
			if err != nil || input != again || canonical != roundtrip {
				t.Fatal("unstable request binding", err)
			}
		}
	})
}
