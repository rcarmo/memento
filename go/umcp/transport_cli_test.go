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
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPythonTransportArgs(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-cli.json")
	if err != nil {
		t.Fatal(err)
	}
	// Preserve arbitrary-size source integers when decoding the oracle.
	var fixtures []struct {
		Args     []string
		Expected map[string]json.RawMessage
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixtures {
		config, err := ParseTransportArgs(c.Args)
		text := func(key string) string { var s string; _ = json.Unmarshal(c.Expected[key], &s); return s }
		if want := text("error"); want != "" {
			if err == nil || err.Error() != want {
				t.Fatal(c.Args, err, want)
			}
			continue
		}
		if err != nil {
			t.Fatal(c.Args, err)
		}
		if config.Mode != text("mode") {
			t.Fatal(c.Args, config)
		}
		if config.Mode == "file" {
			if config.File != text("path") {
				t.Fatal(config)
			}
			continue
		}
		if config.Mode == "stdio" {
			continue
		}
		if config.Host != text("host") || config.Port.String() != string(c.Expected["port"]) {
			t.Fatal(c.Args, config, c.Expected)
		}
		if config.Mode != "tcp" {
			var origins []string
			_ = json.Unmarshal(c.Expected["allowed_origins"], &origins)
			if config.MaxRequestBytes.String() != string(c.Expected["max_request_bytes"]) || !reflect.DeepEqual(config.AllowedOrigins, origins) {
				t.Fatal(c.Args, config, c.Expected)
			}
		}
		if config.Mode == "streamable-http" && config.Endpoint != text("endpoint") {
			t.Fatal(config)
		}
	}
}

func TestRunTransportFileAndStdio(t *testing.T) {
	for _, async := range []bool{false, true} {
		s := streamServer()
		var out bytes.Buffer
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
		if err := s.RunTransport(context.Background(), nil, strings.NewReader(body+"\n"), &out, async, HTTPHooks{}); err != nil || !strings.Contains(out.String(), `"transport":"stdio"`) {
			t.Fatal(err, out.String())
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "request.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if err := s.RunTransport(context.Background(), []string{path}, nil, &out, async, HTTPHooks{}); err != nil || !strings.Contains(out.String(), `"transport":"file"`) {
			t.Fatal(err, out.String())
		}
		s.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) {
			return "done", nil, s.send("notifications/test", nil)
		}
		out.Reset()
		if err := s.RunTransport(context.Background(), nil, strings.NewReader(body), &out, async, HTTPHooks{}); err != nil {
			t.Fatal(err)
		}
		if len(strings.Split(strings.TrimSpace(out.String()), "\n")) != 2 {
			t.Fatal(out.String())
		}
	}
	s := NewServer("test")
	for _, c := range []struct {
		Args []string
		In   io.Reader
		Out  io.Writer
	}{
		{[]string{"--tcp"}, nil, io.Discard}, {nil, nil, nil}, {nil, nil, io.Discard}, {[]string{"missing.json"}, nil, io.Discard},
		{[]string{"--tcp", "--port", "-1"}, nil, io.Discard}, {[]string{"--http", "--port", "65536"}, nil, io.Discard},
		{[]string{"--tcp", "--port", strings.Repeat("9", 30)}, nil, io.Discard}, {[]string{"--http", "--port", "0", "--max-request-bytes", strings.Repeat("9", 30)}, nil, io.Discard},
		{[]string{"--http", "--port", "0", "--endpoint", "bad"}, nil, io.Discard},
		{[]string{"--tcp", "--port", "0", "--host", "bad\x00host"}, nil, io.Discard},
		{[]string{"--tcp", "--port", "0"}, nil, failureWriter{}},
		{[]string{"--tcp", "--port", "0"}, nil, &flushStream{err: io.ErrClosedPipe}},
	} {
		if err := s.RunTransport(context.Background(), c.Args, c.In, c.Out, false, HTTPHooks{}); err == nil {
			t.Fatal(c.Args)
		}
	}
	if _, err := cliInteger(strings.Repeat("0", 4301)); err == nil {
		t.Fatal("integer limit")
	}
	if TransportExitCode(nil) != 0 || TransportExitCode(context.Canceled) != 0 || TransportExitCode(io.ErrClosedPipe) != 1 {
		t.Fatal("exit convention")
	}
}

func TestStreamableHTTPSettings(t *testing.T) {
	s := NewServer("test")
	s.SetHandler("synthetic", func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, nil })
	if s.dispatcher.Handlers["synthetic"] == nil {
		t.Fatal("handler replacement")
	}
	if NewServer("").Name != "MCPServer" {
		t.Fatal("default name")
	}
	want := DefaultStreamableHTTPSettings()
	if s.StreamableHTTP != want || want.validate() != nil {
		t.Fatal(s.StreamableHTTP)
	}
	bad := []StreamableHTTPSettings{
		{}, {SessionTTL: -1, Keepalive: 1, RequestTimeout: 1, MaxSessions: 1, MaxRequestsConnection: 1},
		{SessionTTL: 1, Keepalive: -1, RequestTimeout: 1, MaxSessions: 1, MaxRequestsConnection: 1},
		{SessionTTL: 1, Keepalive: 1, RequestTimeout: -1, MaxSessions: 1, MaxRequestsConnection: 1},
		{SessionTTL: 1, Keepalive: 1, RequestTimeout: 1, MaxSessions: -1, MaxRequestsConnection: 1},
		{SessionTTL: 1, Keepalive: 1, RequestTimeout: 1, MaxSessions: 1, MaxRequestsConnection: -1},
	}
	for _, settings := range bad {
		if settings.validate() == nil {
			t.Fatal(settings)
		}
		s.StreamableHTTP = settings
		if err := s.RunTransport(context.Background(), []string{"--http", "--port", "0"}, nil, io.Discard, false, HTTPHooks{}); err == nil {
			t.Fatal("invalid settings opened listener", settings)
		}
	}
	s.StreamableHTTP = StreamableHTTPSettings{time.Second, 2 * time.Second, 3 * time.Second, 4, 5}
	config, err := ParseTransportArgs([]string{"--http", "--port", "0", "--endpoint", "/x", "--allowed-origin", "https://x", "--max-request-bytes", "123"})
	if err != nil {
		t.Fatal(err)
	}
	httpOptions := s.streamableHTTPOptions(config, false, true)
	if !httpOptions.AsyncReference || httpOptions.Endpoint != "/x" || httpOptions.MaxRequestBytes != 123 || httpOptions.LocalBind || !reflect.DeepEqual(httpOptions.AllowedOrigins, []string{"https://x"}) || httpOptions.SessionTTL != time.Second || httpOptions.MaxSessions != 4 || httpOptions.Keepalive != 2*time.Second {
		t.Fatal(httpOptions)
	}
	syncOptions := s.syncHTTPOptions("streamable-http")
	if syncOptions.Legacy || syncOptions.IOTimeout != 3*time.Second || syncOptions.MaxRequests != 5 {
		t.Fatal(syncOptions)
	}
	if !s.syncHTTPOptions("sse").Legacy {
		t.Fatal("legacy options")
	}
	asyncOptions := s.asyncHTTPOptions(AsyncStreamableParser, "streamable-http", 123)
	if asyncOptions.Parser.ReadTimeout != 3*time.Second || asyncOptions.Parser.MaxRequestBytes != 123 || asyncOptions.MaxRequests != 5 {
		t.Fatal(asyncOptions)
	}
	legacy := s.asyncHTTPOptions(AsyncSSEParser, "sse", 456)
	if legacy.Parser.ReadTimeout != 30*time.Second || legacy.Parser.MaxRequestBytes != 456 || legacy.MaxRequests != 1000 {
		t.Fatal(legacy)
	}
}

type fixedResolver struct {
	addresses []net.IPAddr
	err       error
}

func (r fixedResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return r.addresses, r.err
}

func TestTransportMultiBind(t *testing.T) {
	ctx := context.Background()
	one, err := listenTransport(ctx, fixedResolver{}, "127.0.0.1", "0", true)
	if err != nil || len(one) != 1 {
		t.Fatal(one, err)
	}
	for _, l := range one {
		l.Close()
	}
	one, err = listenTransport(ctx, fixedResolver{}, "ignored", "0", false)
	if err == nil {
		for _, l := range one {
			l.Close()
		}
		t.Fatal("single bind ignored host")
	}
	listeners, err := listenTransport(ctx, fixedResolver{addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}, {IP: nil}, {IP: net.ParseIP("127.0.0.1")}, {IP: net.ParseIP("127.0.0.2")}}}, "host", "0", true)
	if err != nil || len(listeners) != 2 {
		t.Fatal(listeners, err)
	}
	serve := func(ctx context.Context, l net.Listener) error { <-ctx.Done(); return ctx.Err() }
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := serveTransportListeners(cancelled, listeners, serve); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := listenTransport(ctx, fixedResolver{err: io.ErrClosedPipe}, "host", "0", true); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if _, err := listenTransport(ctx, fixedResolver{}, "host", "0", true); err == nil {
		t.Fatal("empty resolution")
	}
	// A later bind failure closes every earlier listener.
	block, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strings.Split(block.Addr().String(), ":")[1]
	opened, err := listenTransport(ctx, fixedResolver{addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.2")}, {IP: net.ParseIP("127.0.0.1")}}}, "host", port, true)
	block.Close()
	if err == nil {
		for _, l := range opened {
			l.Close()
		}
		t.Fatal("bind collision")
	}
	// The first listener error stops and drains every sibling.
	a, b := net.Pipe()
	a.Close()
	fake := []net.Listener{badListener{io.ErrClosedPipe}, &oneConnListener{conn: b}}
	if err := serveTransportListeners(ctx, fake, func(context.Context, net.Listener) error { return io.ErrClosedPipe }); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

type oneConnListener struct{ conn net.Conn }

func (l *oneConnListener) Accept() (net.Conn, error) {
	if l.conn == nil {
		return nil, net.ErrClosed
	}
	c := l.conn
	l.conn = nil
	return c, nil
}
func (l *oneConnListener) Close() error {
	if l.conn != nil {
		return l.conn.Close()
	}
	return nil
}
func (l *oneConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

func TestRunTransportNetwork(t *testing.T) {
	for _, async := range []bool{false, true} {
		for _, mode := range []string{"tcp", "sse", "streamable-http"} {
			s := NewServer("test")
			reader, writer := io.Pipe()
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				done <- s.RunTransport(ctx, []string{"--transport", mode, "--port", "0"}, nil, writer, async, HTTPHooks{})
			}()
			announced := make(chan string, 1)
			go func() { line, _ := bufio.NewReader(reader).ReadString('\n'); announced <- strings.TrimSpace(line) }()
			var line string
			select {
			case line = <-announced:
			case <-time.After(2 * time.Second):
				cancel()
				t.Fatal("missing announcement")
			}
			address := strings.TrimPrefix(line, "Listening on ")
			if mode != "tcp" {
				address = strings.Split(strings.Split(line, "http://")[1], "/")[0]
			}
			if mode == "tcp" {
				conn, err := net.DialTimeout("tcp", address, time.Second)
				if err != nil {
					t.Fatal(err)
				}
				_ = conn.SetDeadline(time.Now().Add(time.Second))
				_, _ = io.WriteString(conn, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")
				reply, err := bufio.NewReader(conn).ReadString('\n')
				conn.Close()
				if err != nil || !strings.Contains(reply, `"tools"`) {
					t.Fatal(reply, err)
				}
			} else {
				path := "/missing"
				want := 404
				if mode == "streamable-http" {
					want = 405
				}
				reply, err := (&http.Client{Timeout: time.Second}).Get("http://" + address + path)
				if err != nil {
					t.Fatal(err)
				}
				reply.Body.Close()
				if reply.StatusCode != want {
					t.Fatal(reply.StatusCode, want)
				}
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("shutdown leaked")
			}
			reader.Close()
			writer.Close()
		}
	}
}

func FuzzTransportArgs(f *testing.F) {
	for _, input := range []string{"", "--port\x000\x00--http", "--transport\x00stdio", "--host"} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) { _, _ = ParseTransportArgs(strings.Split(input, "\x00")) })
}

func TestRunTransportClearsPreviousSink(t *testing.T) {
	s := streamServer()
	h, err := NewStreamableHTTP(s, DefaultHTTPOptions(), HTTPHooks{})
	if err != nil {
		t.Fatal(err)
	}
	h.Close()
	s.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) {
		return nil, nil, s.send("notifications/test", nil)
	}
	var output bytes.Buffer
	err = s.RunTransport(context.Background(), nil, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`), &output, false, HTTPHooks{})
	if err != nil || !strings.Contains(output.String(), `"notifications/test"`) {
		t.Fatal(err, output.String())
	}
}
