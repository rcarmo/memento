package umcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

type memoryConn struct {
	input *bytes.Reader
	bytes.Buffer
}

func (c *memoryConn) Read(p []byte) (int, error) { return c.input.Read(p) }
func (c *memoryConn) Close() error               { return nil }
func (c *memoryConn) LocalAddr() net.Addr        { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1} }
func (c *memoryConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}
func (c *memoryConn) SetDeadline(time.Time) error      { return nil }
func (c *memoryConn) SetReadDeadline(time.Time) error  { return nil }
func (c *memoryConn) SetWriteDeadline(time.Time) error { return nil }

func TestPythonSyncHTTPWire(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-sync-wire.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Legacy bool
			Input  struct {
				Name                   string
				Prefix, Repeat, Suffix []byte
				Count                  int
			}
			Expected struct {
				Status   int
				Protocol string
				Headers  map[string]string
				Body     any
				Interim  bool
			}
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Cases {
		prefix := "streamable/"
		if c.Legacy {
			prefix = "sse/"
		}
		t.Run(prefix+c.Input.Name, func(t *testing.T) {
			input := append(append(append([]byte{}, c.Input.Prefix...), bytes.Repeat(c.Input.Repeat, c.Input.Count)...), c.Input.Suffix...)
			conn := &memoryConn{input: bytes.NewReader(input)}
			s := NewServer("SyncWireOracle")
			var handler http.Handler
			if c.Legacy {
				options := DefaultSSEOptions(SyncSSE)
				options.MaxRequestBytes = 1 << 20
				h, err := NewLegacySSE(s, options, HTTPHooks{})
				if err != nil {
					t.Fatal(err)
				}
				defer h.Close()
				handler = h
			} else {
				options := DefaultHTTPOptions()
				options.MaxRequestBytes = 1 << 20
				h, err := NewStreamableHTTP(s, options, HTTPHooks{Route: func(_ context.Context, method, path string, headers map[string]string, body []byte, _ string) (*HTTPResponse, error) {
					bodyValues := []int{}
					for _, b := range body {
						bodyValues = append(bodyValues, int(b))
					}
					raw, err := json.Marshal(map[string]any{"method": method, "path": path, "headers": headers, "body": bodyValues})
					mime := "application/json"
					return &HTTPResponse{Status: 200, ContentType: &mime, Body: raw}, err
				}})
				if err != nil {
					t.Fatal(err)
				}
				defer h.Close()
				handler = h
			}
			options := DefaultSyncHTTPConnectionOptions(c.Legacy)
			options.MaxRequests = 1
			_ = ServeSyncHTTPConnection(context.Background(), conn, handler, options)
			output := conn.Bytes()
			interim := bytes.HasPrefix(output, []byte("HTTP/1.1 100 Continue\r\n\r\n"))
			if interim {
				output = output[len("HTTP/1.1 100 Continue\r\n\r\n"):]
			}
			if c.Expected.Status == 0 {
				want, _ := c.Expected.Body.(string)
				if json.Valid(output) && json.Valid([]byte(want)) {
					var actual, expected any
					_ = json.Unmarshal(output, &actual)
					_ = json.Unmarshal([]byte(want), &expected)
					if !reflect.DeepEqual(actual, expected) {
						t.Fatal(actual, expected)
					}
				} else if string(output) != want {
					t.Fatalf("body %q != %q", output, want)
				}
				return
			}
			head, body, ok := bytes.Cut(output, []byte("\r\n\r\n"))
			if !ok {
				t.Fatalf("missing response %q", output)
			}
			lines := strings.Split(string(head), "\r\n")
			parts := strings.SplitN(lines[0], " ", 3)
			if parts[0] != c.Expected.Protocol || parts[1] != strconv.Itoa(c.Expected.Status) || interim != c.Expected.Interim {
				t.Fatal(lines[0], c.Expected)
			}
			headers := map[string]string{}
			for _, line := range lines[1:] {
				k, v, _ := strings.Cut(line, ":")
				k = strings.ToLower(k)
				if k != "date" && k != "server" {
					headers[k] = strings.TrimSpace(v)
				}
			}
			var bodyValue any = string(body)
			if headers["content-type"] == "application/json" {
				delete(headers, "content-length")
				if err = json.Unmarshal(body, &bodyValue); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(headers, c.Expected.Headers) || !reflect.DeepEqual(bodyValue, c.Expected.Body) {
				t.Fatalf("headers %v != %v; body %.500v != %.500v", headers, c.Expected.Headers, bodyValue, c.Expected.Body)
			}
		})
	}
}

func TestSyncHTTPConnectionFailures(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { emptyHTTP(w, 200, "") })
	a, b := net.Pipe()
	if err := ServeSyncHTTPConnection(context.Background(), a, nil, DefaultSyncHTTPConnectionOptions(false)); err == nil {
		t.Fatal("nil handler")
	}
	b.Close()
	a, b = net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ServeSyncHTTPConnection(ctx, a, handler, DefaultSyncHTTPConnectionOptions(false)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	b.Close()
	for _, input := range []string{"GET / HTTP/2.0\n", "GET / HTTP/1.1\nHost: localhost\n\n", "GET / HTTP/1.1\nHost: localhost\nExpect: 100-continue\n\n"} {
		a, b = net.Pipe()
		_ = b.SetDeadline(time.Now().Add(time.Second))
		done := make(chan error, 1)
		go func() {
			done <- ServeSyncHTTPConnection(context.Background(), writeFailureConn{a}, handler, DefaultSyncHTTPConnectionOptions(false))
		}()
		_, _ = io.WriteString(b, input)
		if err := <-done; !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
		b.Close()
	}
	if err := ServeSyncHTTP(context.Background(), badListener{}, nil, DefaultSyncHTTPConnectionOptions(false)); err == nil {
		t.Fatal("invalid listener")
	}
}

func TestSyncWireWriter(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		var output bytes.Buffer
		w := newSyncWireWriter(&output, legacy, "HTTP/1.0", true, false)
		w.WriteHeader(200)
		w.WriteHeader(500)
		if _, err := w.Write([]byte("test")); err != nil {
			t.Fatal(err)
		}
		w.Flush()
		if strings.Contains(output.String(), "500") {
			t.Fatal(output.String())
		}
		output.Reset()
		w = newSyncWireWriter(&output, legacy, "HTTP/1.1", true, false)
		if err := w.FlushError(); err != nil {
			t.Fatal(err)
		}
		output.Reset()
		w = newSyncWireWriter(&output, legacy, "HTTP/1.1", false, false)
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event"))
		if err := w.FlushError(); err != nil || !w.keepAlive {
			t.Fatal(err)
		}
	}
	if _, err := encodeLatin1("x€"); err == nil {
		t.Fatal("non Latin1")
	}
	for _, value := range []string{"€", "x\r\ninjected"} {
		w := newSyncWireWriter(io.Discard, false, "HTTP/1.1", false, false)
		w.Header().Set("X", value)
		if _, err := w.Write(nil); err == nil {
			t.Fatal("bad header accepted")
		}
		if err := w.FlushError(); err == nil {
			t.Fatal("error lost")
		}
	}
	w := newSyncWireWriter(failureWriter{}, false, "HTTP/1.1", false, false)
	w.Header().Set("X", strings.Repeat("a", 5000))
	w.WriteHeader(200)
	if err := w.FlushError(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	w = newSyncWireWriter(failureWriter{}, false, "HTTP/1.1", false, false)
	if _, err := w.Write(bytes.Repeat([]byte("a"), 8192)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

func TestSyncContinueBeforeBody(t *testing.T) {
	_, h := testHTTP(t)
	a, b := net.Pipe()
	defer b.Close()
	_ = b.SetDeadline(time.Now().Add(2 * time.Second))
	done := make(chan error, 1)
	options := DefaultSyncHTTPConnectionOptions(false)
	options.MaxRequests = 1
	go func() { done <- ServeSyncHTTPConnection(context.Background(), a, h, options) }()
	_, err := io.WriteString(b, "POST /mcp HTTP/1.1\r\nHost: localhost\r\nExpect: 100-continue\r\nAuthorization: Bearer reader\r\nContent-Type: application/json\r\nContent-Length: 2\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(b)
	reply, err := http.ReadResponse(reader, nil)
	if err != nil || reply.StatusCode != 100 {
		t.Fatal(reply, err)
	}
	reply.Body.Close()
	_, err = io.WriteString(b, "{}")
	if err != nil {
		t.Fatal(err)
	}
	reply, err = http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, reply.Body)
	reply.Body.Close()
	if reply.StatusCode != 400 || !reply.Close {
		t.Fatal(reply)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSyncRouteDependentReadsAndTimeout(t *testing.T) {
	_, h := testHTTP(t)
	for _, raw := range []string{
		"POST /mcp HTTP/1.1\nHost: localhost\nContent-Length: 3\n\n{}",
		"POST /mcp HTTP/1.1\nHost: localhost\nContent-Length: 3\n\n",
	} {
		a, b := net.Pipe()
		defer b.Close()
		_ = b.SetDeadline(time.Now().Add(time.Second))
		options := DefaultSyncHTTPConnectionOptions(false)
		options.IOTimeout = 5 * time.Millisecond
		done := make(chan error, 1)
		go func() { done <- ServeSyncHTTPConnection(context.Background(), a, h, options) }()
		_, _ = io.WriteString(b, raw)
		reply, err := http.ReadResponse(bufio.NewReader(b), nil)
		if err != nil {
			t.Fatal(err)
		}
		reply.Body.Close()
		if reply.StatusCode != 400 || !reply.Close {
			t.Fatal(reply)
		}
		if err = <-done; err != nil {
			t.Fatal(err)
		}
	}
}
