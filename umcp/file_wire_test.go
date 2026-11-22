package umcp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFileStreamAbortOnReferenceConnections(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "async"}[async], func(t *testing.T) {
			stream := &observedFileStream{Reader: strings.NewReader("short")}
			hooks := HTTPHooks{Route: func(context.Context, string, string, map[string]string, []byte, string) (*HTTPResponse, error) {
				return &HTTPResponse{Status: 200, BodyStream: stream, BodyLength: 100}, nil
			}}
			options := DefaultHTTPOptions()
			options.AsyncReference = async
			handler, err := NewStreamableHTTP(NewServer("abort"), options, hooks)
			if err != nil {
				t.Fatal(err)
			}
			defer handler.Close()
			server, client := net.Pipe()
			defer client.Close()
			client.SetDeadline(time.Now().Add(3 * time.Second))
			finished := make(chan error, 1)
			go func() {
				if async {
					finished <- ServeAsyncHTTPConnection(context.Background(), server, handler, DefaultAsyncHTTPConnectionOptions(AsyncStreamableParser))
				} else {
					finished <- ServeSyncHTTPConnection(context.Background(), server, handler, DefaultSyncHTTPConnectionOptions(false))
				}
			}()
			if _, err = io.WriteString(client, "GET /file HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			_, _ = io.ReadAll(client)
			if err = <-finished; !errors.Is(err, http.ErrAbortHandler) || !stream.closed {
				t.Fatal(err, stream.closed)
			}
		})
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "http://localhost/", nil)
	func() {
		defer func() {
			if recover() != "original panic" {
				t.Fatal("unrelated panic was swallowed")
			}
		}()
		_ = serveHTTPConnectionHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("original panic") }), recorder, request)
	}()
}
func TestFileHTTPVersionErrorsAndBasePath(t *testing.T) {
	server := NewServer("extra")
	server.AdditionalProtocolVersions = []string{"2026-07-28"}
	options := DefaultHTTPOptions()
	options.Endpoint = "/custom/mcp"
	hooks := HTTPHooks{Route: func(ctx context.Context, _ string, _ string, _ map[string]string, _ []byte, _ string) (*HTTPResponse, error) {
		if AuxiliaryMCPPath(ctx) != "/custom/mcp" {
			t.Fatal("untrusted/missing base path")
		}
		return &HTTPResponse{Status: 204}, nil
	}}
	h, err := NewStreamableHTTP(server, options, hooks)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if AuxiliaryMCPPath(context.Background()) != "/mcp" {
		t.Fatal("default base path")
	}
	request := httptest.NewRequest("GET", "http://localhost/child", nil)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	if recorder.Code != 204 {
		t.Fatal(recorder.Code)
	}
	errorResult := h.protocolVersionError(nil)
	supported := errorResult["supported"].([]string)
	if len(supported) != 3 || supported[2] != "2026-07-28" {
		t.Fatal(errorResult)
	}
}
