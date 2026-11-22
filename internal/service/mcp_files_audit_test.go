package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type countedUpload struct {
	io.Reader
	reads int
}

func (r *countedUpload) Read(p []byte) (int, error) { r.reads++; return r.Reader.Read(p) }

func TestMCPFilesAdmissionBeforeRead(t *testing.T) {
	h := newFileHarness(t)
	raw := submitZIP(t)
	state, _ := h.begin(fileArguments(raw, "admission"))
	for _, tt := range []struct {
		name, principal, token, mime string
		size                         int64
		busy                         bool
		status                       int
	}{
		{"unauthenticated", "", state, "application/zip", int64(len(raw)), false, 401},
		{"wrong principal", "other", state, "application/zip", int64(len(raw)), false, 404},
		{"wrong token", "actor", "bad", "application/zip", int64(len(raw)), false, 404},
		{"wrong type", "actor", state, "application/pdf", int64(len(raw)), false, 415},
		{"wrong length", "actor", state, "application/zip", int64(len(raw)) + 1, false, 400},
		{"busy", "actor", state, "application/zip", int64(len(raw)), true, 429},
		{"accepted", "actor", state, "application/zip", int64(len(raw)), false, 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.busy {
				h.files.gate <- struct{}{}
				defer func() { <-h.files.gate }()
			}
			reader := &countedUpload{Reader: bytes.NewReader(raw)}
			headers := map[string]string{fileTokenHeader: tt.token, "content-type": tt.mime}
			if tt.principal != "" {
				headers["authorization"] = "Bearer " + tt.principal
			}
			r, err := h.files.handle(context.Background(), "POST", "/files/upload", headers, tt.size, reader)
			if err != nil || r.Status != tt.status || (reader.reads > 0) != (tt.status == 200) {
				t.Fatalf("status=%v reads=%d err=%v", r, reader.reads, err)
			}
		})
	}
}

func TestMCPFilesUploadCommitRollback(t *testing.T) {
	h := newFileHarness(t)
	raw := submitZIP(t)
	args := fileArguments(raw, "commit-rollback")
	state, _ := h.begin(args)
	base := time.Now().Truncate(time.Second)
	calls := 0
	// Let the body finish, then make the transfer expire in the binder, after
	// SQLite has staged the pack but before this transaction can commit.
	h.files.now = func() time.Time {
		calls++
		if calls >= 4 {
			return base.Add(2 * time.Hour)
		}
		return base
	}
	r, err := h.files.handle(context.Background(), "POST", "/files/upload", map[string]string{"authorization": "Bearer actor", fileTokenHeader: state, "content-type": "application/zip"}, int64(len(raw)), bytes.NewReader(raw))
	if err != nil || r.Status != 404 || calls != 4 {
		t.Fatalf("status=%v clock calls=%d err=%v", r, calls, err)
	}
	db := h.jobs.Controls.Staging.DB
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM staged_assets WHERE idempotency_key='mcp-files:commit-rollback'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("asset write survived rollback: count=%d err=%v", count, err)
	}
	var staged string
	if err := db.QueryRow(`SELECT staged_id FROM mcp_file_transfers WHERE token_digest=?`, transferDigest(state)).Scan(&staged); err != nil || staged != "" {
		t.Fatalf("receipt survived rollback: %q %v", staged, err)
	}
}

func TestMCPFilesInactiveKeyAndCompletedReplay(t *testing.T) {
	for _, state := range []string{"consumed", "expired"} {
		t.Run(state, func(t *testing.T) {
			h := newFileHarness(t)
			raw := submitZIP(t)
			args := fileArguments(raw, "inactive")
			prior, put := h.begin(args)
			if status, _ := h.request("POST", "/mcp/files/upload", "actor", raw, put); status != 200 {
				t.Fatal(status)
			}
			params := fileParams(fileUploadTool, args)
			params["requestState"] = prior
			original := fileResult(t, h.rpc("tools/call", "actor", params))
			db := h.jobs.Controls.Staging.DB
			if _, err := db.Exec(`UPDATE staged_assets SET state=?,blob_bytes=X'' WHERE idempotency_key='mcp-files:inactive'`, state); err != nil {
				t.Fatal(err)
			}
			next, putNew := h.begin(args)
			if status, _ := h.request("POST", "/mcp/files/upload", "actor", raw, putNew); status != 400 {
				t.Fatal("unavailable bytes acknowledged", state, status)
			}
			var staged string
			if err := db.QueryRow(`SELECT staged_id FROM mcp_file_transfers WHERE token_digest=?`, transferDigest(next)).Scan(&staged); err != nil || staged != "" {
				t.Fatal(staged, err)
			}
			if status, _ := h.request("POST", "/mcp/files/upload", "actor", raw, put); status != 200 {
				t.Fatal("completed transfer retry", status)
			}
			replay := fileResult(t, h.rpc("tools/call", "actor", params))
			if original["structuredContent"].(map[string]any)["staged_asset_id"] != replay["structuredContent"].(map[string]any)["staged_asset_id"] {
				t.Fatal("completed receipt changed")
			}
		})
	}
}

func TestMCPFilesModernWireContract(t *testing.T) {
	h := newFileHarness(t)
	discover := fileResult(t, h.rpc("server/discover", "actor", nil))
	if discover["resultType"] != "complete" || discover["ttlMs"] != float64(0) || discover["cacheScope"] != "private" || discover["supportedVersions"].([]any)[0] != fileProtocol {
		t.Fatal(discover)
	}
	caps := discover["capabilities"].(map[string]any)
	if len(caps) != 2 || caps["tools"] == nil || caps["extensions"] == nil {
		t.Fatal(caps)
	}
	list := fileResult(t, h.rpc("tools/list", "actor", nil))
	if list["resultType"] != "complete" || list["ttlMs"] != float64(0) || list["cacheScope"] != "private" {
		t.Fatal(list)
	}
	for _, tc := range []struct {
		name, method string
		params       map[string]any
		headers      map[string]string
		status       int
		code         float64
	}{
		{"missing method", "ping", nil, map[string]string{"Mcp-Method": ""}, 400, -32020},
		{"different method", "ping", nil, map[string]string{"Mcp-Method": "tools/list"}, 400, -32020},
		{"wrong version in body", "ping", map[string]any{"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": "2025-03-26"}}, nil, 400, -32020},
		{"legacy header with modern metadata", "ping", nil, map[string]string{"Mcp-Protocol-Version": "2025-03-26"}, 400, -32020},
		{"missing version header", "ping", nil, map[string]string{"Mcp-Protocol-Version": ""}, 400, -32020},
		{"unsupported version", "ping", map[string]any{"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": "2099-01-01"}}, map[string]string{"Mcp-Protocol-Version": "2099-01-01"}, 400, -32022},
		{"unsupported header differs from body", "ping", nil, map[string]string{"Mcp-Protocol-Version": "2099-01-01"}, 400, -32020},
		{"missing tool name", "tools/call", fileParams(fileUploadTool, fileArguments(submitZIP(t), "no-name")), map[string]string{"Mcp-Name": ""}, 400, -32020},
		{"wrong tool name", "tools/call", fileParams(fileUploadTool, fileArguments(submitZIP(t), "wrong-name")), map[string]string{"Mcp-Name": fileDownloadTool}, 400, -32020},
		{"unavailable method", "some/unknown", nil, nil, 404, -32601},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, headers := modernFileRequest(tc.method, tc.params)
			for key, value := range tc.headers {
				headers[key] = value
			}
			status, body := h.request("POST", "/mcp", "actor", raw, headers)
			var r map[string]any
			if status != tc.status || json.Unmarshal(body, &r) != nil {
				t.Fatalf("HTTP %d %s", status, body)
			}
			fileRPCError(t, r, tc.code)
			if r["id"] != float64(1) {
				t.Fatal(r)
			}
			if tc.code == -32022 {
				data := r["error"].(map[string]any)["data"].(map[string]any)
				if data["requested"] != "2099-01-01" || len(data["supported"].([]any)) != 3 {
					t.Fatal(data)
				}
			}
		})
	}
	var count int
	if err := h.jobs.Controls.Staging.DB.QueryRow(`SELECT COUNT(*) FROM mcp_file_transfers`).Scan(&count); err != nil || count != 0 {
		t.Fatal("invalid metadata reached file tool", count, err)
	}
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`)
	if status, body := h.request("POST", "/mcp", "actor", raw, map[string]string{"Content-Type": "application/json", "Mcp-Method": "ping"}); status != 400 || !bytes.Contains(body, []byte(`-32020`)) {
		t.Fatal(status, string(body))
	}
	for _, method := range []string{"GET", "DELETE"} {
		if status, _ := h.request(method, "/mcp", "actor", nil, nil); status != 405 {
			t.Fatal(method, status)
		}
	}
	// A modern request does not depend on an initialize/legacy session.
	modernBody, modernHeaders := modernFileRequest("ping", nil)
	modernHeaders["Mcp-Session-Id"] = "old-or-unknown"
	if status, body := h.request("POST", "/mcp", "actor", modernBody, modernHeaders); status != 200 || !bytes.Contains(body, []byte(`"resultType":"complete"`)) {
		t.Fatal(status, string(body))
	}
	// The legacy request and its result require none of the new mirrored headers.
	if status, body := h.request("POST", "/mcp", "actor", raw, map[string]string{"Content-Type": "application/json", "Mcp-Protocol-Version": "2025-03-26"}); status != 200 || string(body) != `{"jsonrpc":"2.0","result":{},"id":1}` {
		t.Fatal(status, string(body))
	}
	req, err := http.NewRequest("OPTIONS", h.host.URL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", h.host.URL)
	resp, err := h.host.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Mcp-Method, Mcp-Name") {
		t.Fatal(resp.Status, resp.Header)
	}
}

func TestMCPFilesDigestCanonicalForm(t *testing.T) {
	raw := submitZIP(t)
	args := fileArguments(raw, "digest")
	object := args["file"].(map[string]any)["digest"].(map[string]any)
	good := object["value"].(string)
	if _, _, err := parseFileArguments(args); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{good + "\n", good[:42] + "\n", good + "=", strings.Repeat("A", 1<<20), strings.Repeat("A", 42) + "B"} {
		object["value"] = value
		if _, _, err := parseFileArguments(args); err == nil {
			t.Fatal("accepted non-canonical digest of length", len(value))
		}
	}
}
