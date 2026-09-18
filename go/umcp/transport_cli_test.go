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
