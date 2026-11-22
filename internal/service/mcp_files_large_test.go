package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/rcarmo/memento/umcp"
)

func TestMCPFilesLargeConfirmation(t *testing.T) {
	h := newFileHarness(t)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: "large.txt", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	block := bytes.Repeat([]byte("x"), 64<<10)
	for i := 0; i < 128; i++ {
		if _, err = entry.Write(block); err != nil {
			t.Fatal(err)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	raw := buffer.Bytes()
	args := fileArguments(raw, "large")
	state, headers := h.begin(args)
	status, _ := h.request("POST", "/mcp/files/upload", "actor", raw, headers)
	if status != 200 {
		t.Fatal(status)
	}
	p := fileParams(fileUploadTool, args)
	p["requestState"] = state
	request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": p})
	trusted := umcp.RequestContext{Transport: "streamable-http", ProtocolVersion: fileProtocol, Principal: "actor"}
	// Warm statement/runtime caches, then measure confirmation without HTTP upload
	// buffering or fixture construction. No pack bytes belong in the result.
	record, err := h.files.record(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = h.jobs.Controls.Staging.Metadata(context.Background(), "actor", record.StagedID); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	r, err := h.server.Process(context.Background(), request, trusted)
	runtime.ReadMemStats(&after)
	if err != nil || r.Error != nil || r.Result.(map[string]any)["isError"] == true {
		t.Fatal(r, err)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("8MiB immutable upload confirmation allocated %d bytes", allocated)
	if allocated > 1<<20 {
		t.Fatalf("metadata confirmation loaded pack-sized data: %d bytes", allocated)
	}
}
