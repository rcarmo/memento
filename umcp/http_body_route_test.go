package umcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPBodyRouteRawAdmission(t *testing.T) {
	for _, async := range []bool{false, true} {
		for _, tc := range []struct {
			name, token      string
			expect, accepted bool
		}{
			{"reject without expect", "denied", false, false},
			{"reject with expect", "denied", true, false},
			{"accept with expect", "allowed", true, true},
		} {
			t.Run(fmt.Sprintf("async=%t/%s", async, tc.name), func(t *testing.T) {
				opts := DefaultHTTPOptions()
				opts.AsyncReference = async
				opts.Endpoint = "/custom/mcp"
				hooks := HTTPHooks{BodyRoute: &HTTPBodyRoute{Match: func(path string) bool { return path == "/files/upload" }, Handle: func(_ context.Context, method, path string, headers map[string]string, size int64, body io.Reader) (*HTTPResponse, error) {
					if method != "POST" || path != "/files/upload" || size != 3 {
						return nil, fmt.Errorf("unexpected request %s %s (%d)", method, path, size)
					}
					if headers["x-test-credential"] != "allowed" {
						return &HTTPResponse{Status: 401}, nil
					}
					data, err := io.ReadAll(body)
					if err != nil || string(data) != "zip" {
						return nil, fmt.Errorf("body=%q: %v", data, err)
					}
					return &HTTPResponse{Status: 200, Body: []byte("ok")}, nil
				}}}
				h, err := NewStreamableHTTP(NewServer("body-route"), opts, hooks)
				if err != nil {
					t.Fatal(err)
				}
				defer h.Close()
				server, client := net.Pipe()
				defer client.Close()
				if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					t.Fatal(err)
				}
				done := make(chan error, 1)
				go func() {
					if async {
						done <- ServeAsyncHTTPConnection(context.Background(), server, h, DefaultAsyncHTTPConnectionOptions(AsyncStreamableParser))
					} else {
						done <- ServeSyncHTTPConnection(context.Background(), server, h, DefaultSyncHTTPConnectionOptions(false))
					}
				}()
				expect := ""
				if tc.expect {
					expect = "Expect: 100-continue\r\n"
				}
				// Send the headers alone. Rejected requests must not wait for body bytes or
				// ask the client to continue. The accepted request gets Continue on read.
				if _, err = fmt.Fprintf(client, "POST /custom/mcp/files/upload HTTP/1.1\r\nHost: localhost\r\nContent-Length: 3\r\nX-Test-Credential: %s\r\n%s\r\n", tc.token, expect); err != nil {
					t.Fatal(err)
				}
				reader := bufio.NewReader(client)
				r, err := http.ReadResponse(reader, &http.Request{Method: "POST"})
				if err != nil {
					t.Fatal(err)
				}
				if tc.accepted {
					if r.StatusCode != 100 {
						t.Fatal(r.Status)
					}
					r.Body.Close()
					if _, err = io.WriteString(client, "zip"); err != nil {
						t.Fatal(err)
					}
					r, err = http.ReadResponse(reader, &http.Request{Method: "POST"})
					if err != nil {
						t.Fatal(err)
					}
					raw, err := io.ReadAll(r.Body)
					r.Body.Close()
					if err != nil || r.StatusCode != 200 || string(raw) != "ok" {
						t.Fatal(r.Status, string(raw), err)
					}
				} else {
					r.Body.Close()
					if r.StatusCode != 401 {
						t.Fatal(r.Status)
					}
				}
				if !r.Close {
					t.Fatal("transfer connection was reusable")
				}
				if err = <-done; err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestModernHTTPRejectsDuplicateMirroredHeaders(t *testing.T) {
	// In the async parser two fields can normalize to a single comma-joined
	// value. It must still be rejected even if that value matches the body.
	const body = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"one,two","_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
	wire := fmt.Sprintf("POST /mcp HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nMcp-Protocol-Version: 2026-07-28\r\nMcp-Method: tools/call\r\nMcp-Name: one\r\nMcp-Name: two\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
	for _, async := range []bool{false, true} {
		t.Run(fmt.Sprint(async), func(t *testing.T) {
			server := NewServer("duplicates")
			server.AdditionalProtocolVersions = []string{StatelessHTTPVersion}
			opts := DefaultHTTPOptions()
			opts.AsyncReference = async
			h, err := NewStreamableHTTP(server, opts, HTTPHooks{})
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			peer, client := net.Pipe()
			defer client.Close()
			if err = client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				if async {
					done <- ServeAsyncHTTPConnection(context.Background(), peer, h, DefaultAsyncHTTPConnectionOptions(AsyncStreamableParser))
				} else {
					done <- ServeSyncHTTPConnection(context.Background(), peer, h, DefaultSyncHTTPConnectionOptions(false))
				}
			}()
			if _, err = io.WriteString(client, wire); err != nil {
				t.Fatal(err)
			}
			r, err := http.ReadResponse(bufio.NewReader(client), &http.Request{Method: "POST"})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil || r.StatusCode != 400 || !strings.Contains(string(raw), `"code":-32020`) {
				t.Fatal(r.Status, string(raw), err)
			}
			if err = <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMCPNameHeaderEncoding(t *testing.T) {
	for _, value := range []struct {
		header, want string
		ok           bool
	}{
		{"ordinary", "ordinary", true}, {"=?base64?w6k=?=", "é", true},
		{"=?base64?*?=", "", false}, {"é", "", false}, {"", "", false},
	} {
		got, ok := decodeMCPName(value.header)
		if ok != value.ok || ok && got != value.want {
			t.Errorf("header %q: %q %t", value.header, got, ok)
		}
	}
}

func BenchmarkDeferredHTTPBody(b *testing.B) {
	const size = 1 << 20
	header := fmt.Sprintf("POST /mcp/files/upload HTTP/1.1\r\nHost: localhost\r\nContent-Length: %d\r\n\r\n", size)
	wire := header + strings.Repeat("z", size)
	for _, deferred := range []bool{false, true} {
		b.Run(fmt.Sprintf("deferred=%t", deferred), func(b *testing.B) {
			options := DefaultAsyncHTTPParserOptions(AsyncStreamableParser)
			var match func(string) bool
			if deferred {
				match = func(string) bool { return true }
			}
			input := strings.NewReader(wire)
			reader := bufio.NewReader(input)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				input.Reset(wire)
				reader.Reset(input)
				p, err := parseAsyncHTTPRequest(reader, options, match)
				if err != nil || deferred && (!p.deferredBody || p.deferredSize != size || len(p.Body) != 0) || !deferred && len(p.Body) != size {
					b.Fatal(err)
				}
			}
		})
	}
}
