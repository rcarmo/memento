package umcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type observedFileStream struct {
	io.Reader
	closed bool
}

func (s *observedFileStream) Close() error { s.closed = true; return nil }
func TestExperimentalVersionIsolation(t *testing.T) {
	server := NewServer("versions")
	if server.SupportsProtocolVersion("2026-07-28") {
		t.Fatal("experimental revision enabled by default")
	}
	server.AdditionalProtocolVersions = []string{"2026-07-28"}
	if !server.SupportsProtocolVersion("2026-07-28") || server.SupportsProtocolVersion("2099-01-01") {
		t.Fatal("revision set")
	}
	for _, version := range []string{"2026-07-28", "2025-03-26", "2099-01-01"} {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": version}})
		r, err := server.Process(context.Background(), raw, RequestContext{})
		if err != nil || r.Error != nil {
			t.Fatal(r, err)
		}
		expected := version
		if version == "2099-01-01" {
			expected = "2025-03-26"
		}
		if r.Result.(map[string]any)["protocolVersion"] != expected {
			t.Fatal(r)
		}
	}
	if ProtocolVersions()[0] != "2025-03-26" {
		t.Fatal("package negotiation changed")
	}
}
func TestAuxiliaryFileStreams(t *testing.T) {
	for _, mode := range []string{"ok", "invalid", "short", "hook-error"} {
		t.Run(mode, func(t *testing.T) {
			stream := &observedFileStream{Reader: strings.NewReader("zip")}
			response := &HTTPResponse{Status: 200, BodyStream: stream, BodyLength: 3}
			if mode == "invalid" {
				response.Body = []byte("also bytes")
			}
			if mode == "short" {
				response.BodyLength = 4
			}
			hooks := HTTPHooks{Route: func(context.Context, string, string, map[string]string, []byte, string) (*HTTPResponse, error) {
				if mode == "hook-error" {
					return response, io.ErrUnexpectedEOF
				}
				return response, nil
			}}
			h, err := NewStreamableHTTP(NewServer("stream"), DefaultHTTPOptions(), hooks)
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "http://localhost/file", nil)
			aborted := false
			func() {
				defer func() {
					if p := recover(); p != nil {
						if p != http.ErrAbortHandler {
							panic(p)
						}
						aborted = true
					}
				}()
				h.ServeHTTP(recorder, request)
			}()
			if !stream.closed {
				t.Fatal("stream leaked")
			}
			switch mode {
			case "ok":
				if recorder.Code != 200 || recorder.Body.String() != "zip" || recorder.Header().Get("Content-Length") != "3" {
					t.Fatal(recorder)
				}
			case "invalid", "hook-error":
				if recorder.Code != 500 {
					t.Fatal(recorder.Code)
				}
			case "short":
				if !aborted {
					t.Fatal("truncated response accepted")
				}
			}
		})
	}
	if ValidateHTTPResponse(&HTTPResponse{Status: 200, BodyStream: &observedFileStream{}, BodyLength: -1}, 100) || ValidateHTTPResponse(&HTTPResponse{Status: 200, BodyStream: &observedFileStream{}, BodyLength: 101}, 100) {
		t.Fatal("stream bound validation")
	}
}
func TestFileTransferLogRedaction(t *testing.T) {
	for _, target := range []any{map[string]any{"url": "https://example.invalid/secret", "credentials": "omit"}, OrderedObject{{Name: "path", Value: "/secret"}, {Name: "credentials", Value: "include"}}} {
		if SanitizeLogData(target) != "[redacted file target]" {
			t.Fatal("target not redacted")
		}
	}
	input := map[string]any{"requestState": "secret-state", "uploads": []any{map[string]any{"path": "/upload", "credentials": "include", "headers": map[string]any{"X-Token": "secret"}}}, "data": "inline-secret", "downloads": []any{map[string]any{"url": "https://storage.invalid/?secret"}}}
	raw, _ := json.Marshal(SanitizeLogData(input))
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "/upload") {
		t.Fatal(string(raw))
	}
}
