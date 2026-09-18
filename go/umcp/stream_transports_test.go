package umcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func streamServer() *Server {
	s := NewServer("StreamOracle")
	s.dispatcher.Handlers["initialize"] = func(ctx context.Context, params map[string]any) (any, *RPCError, error) {
		c := Context(ctx)
		return map[string]any{"transport": c.Transport, "params": params}, nil, nil
	}
	return s
}

func TestFileTransport(t *testing.T) {
	s := streamServer()
	var output bytes.Buffer
	input := "{\r\n\"jsonrpc\":\"2.0\",\r\"id\":1,\n\"method\":\"initialize\"\r\n}"
	if err := s.ServeFile(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	value, ok := decode(output.Bytes())
	if !ok || value.(map[string]any)["result"].(map[string]any)["transport"] != "file" {
		t.Fatal(output.String())
	}
	output.Reset()
	if err := s.ServeFile(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`), &output); err != nil || output.Len() != 0 {
		t.Fatal(err, output.String())
	}
	for _, input := range []string{"", `{} {}`, "\xff"} {
		output.Reset()
		err := s.ServeFile(context.Background(), strings.NewReader(input), &output)
		if input == "\xff" {
			if err == nil {
				t.Fatal("bad UTF8 accepted")
			}
		} else if err != nil || output.Len() == 0 {
			t.Fatal(err, output.String())
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.ServeFile(ctx, strings.NewReader(""), io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.ServeFile(context.Background(), failureReader{}, io.Discard); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := s.ServeFile(context.Background(), strings.NewReader("{}"), failureWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	s.dispatcher.Handlers["fail"] = func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, io.ErrClosedPipe }
	if err := s.ServeFile(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"fail"}`), io.Discard); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.ProcessFile(context.Background(), path, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := s.ProcessFile(context.Background(), path+"absent", io.Discard); err == nil {
		t.Fatal("missing file accepted")
	}
}

type deadlineConn struct {
	net.Conn
	err error
}

func (c deadlineConn) SetReadDeadline(time.Time) error  { return c.err }
func (c deadlineConn) SetWriteDeadline(time.Time) error { return c.err }

type flushStream struct {
	bytes.Buffer
	err     error
	flushed bool
}

func (w *flushStream) Flush() error { w.flushed = true; return w.err }

type badListener struct{ err error }

func (l badListener) Accept() (net.Conn, error) { return nil, l.err }
func (l badListener) Close() error              { return nil }
func (l badListener) Addr() net.Addr            { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1} }

func TestStreamHelpers(t *testing.T) {
	for _, c := range []struct {
		Input  string
		Strict bool
		Want   string
		Fail   bool
	}{{" \x1chello\r\n", false, "hello", false}, {"\xff", false, "�", false}, {"\xff", true, "", true}} {
		got, err := lineText(c.Input, c.Strict)
		if (err != nil) != c.Fail || got != c.Want {
			t.Fatal(c, got, err)
		}
	}
	for _, c := range []struct {
		Text  string
		Limit int
		Want  string
		Err   error
	}{{"abc\n", 3, "abc\n", nil}, {"abc", 3, "abc", io.EOF}, {"", 3, "", io.EOF}, {strings.Repeat("a", 9000) + "\n", 0, strings.Repeat("a", 9000) + "\n", nil}} {
		got, err := readLine(bufio.NewReader(strings.NewReader(c.Text)), c.Limit)
		if got != c.Want || !errors.Is(err, c.Err) {
			t.Fatal(len(got), err)
		}
	}
	if _, err := readLine(bufio.NewReader(strings.NewReader("abcd\n")), 3); err == nil {
		t.Fatal("line limit ignored")
	}
	w := &flushStream{}
	if err := encodeLine(w, map[string]any{"x": 1}); err != nil || !w.flushed {
		t.Fatal(err)
	}
	w = &flushStream{err: io.ErrClosedPipe}
	if err := encodeLine(w, nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := encodeLine(io.Discard, make(chan int)); err == nil {
		t.Fatal("invalid JSON encoded")
	}
	if tcpPeer(&net.TCPAddr{IP: net.ParseIP("::1"), Port: 42}) != "::1:42" {
		t.Fatal("IPv6 peer")
	}
	if tcpPeer(&net.UnixAddr{Name: "local", Net: "unix"}) != "local" {
		t.Fatal("local peer")
	}
	if DefaultTCPOptions(SyncTCP).IOTimeout != 30*time.Second || DefaultTCPOptions(AsyncTCP).MaxLineBytes != 65536 {
		t.Fatal("source defaults")
	}
}

func TestTCPConnectionLifecycle(t *testing.T) {
	for _, mode := range []TCPMode{SyncTCP, AsyncTCP} {
		s := streamServer()
		server, client := net.Pipe()
		done := make(chan error, 1)
		ctx, cancel := context.WithCancel(context.Background())
		go func() { done <- s.ServeTCPConnection(ctx, server, DefaultTCPOptions(mode)) }()
		_ = client.SetDeadline(time.Now().Add(2 * time.Second))
		_, err := io.WriteString(client, " \n"+`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"+`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n")
		if err != nil {
			t.Fatal(err)
		}
		reply, err := bufio.NewReader(client).ReadString('\n')
		if err != nil || !strings.Contains(reply, `"transport":"tcp"`) {
			t.Fatal(reply, err)
		}
		cancel()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("blocked I/O leaked")
		}
	}
	s := streamServer()
	for _, options := range []TCPOptions{{Mode: 9}, {IOTimeout: -1}, {MaxLineBytes: -1}} {
		a, b := net.Pipe()
		if err := s.ServeTCPConnection(context.Background(), a, options); err == nil {
			t.Fatal(options)
		}
		_ = b.Close()
	}
	a, b := net.Pipe()
	if err := s.ServeTCPConnection(context.Background(), deadlineConn{a, io.ErrClosedPipe}, DefaultTCPOptions(SyncTCP)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	_ = b.Close()
	a, b = net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.ServeTCPConnection(ctx, a, DefaultTCPOptions(SyncTCP)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	_ = b.Close()
}

func TestTCPConnectionFailureModes(t *testing.T) {
	for _, c := range []struct {
		Input   string
		Options TCPOptions
		Handler Handler
	}{{"\xff\n", DefaultTCPOptions(AsyncTCP), nil}, {"too long\n", TCPOptions{MaxLineBytes: 1}, nil}, {`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n", DefaultTCPOptions(SyncTCP), func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, io.ErrClosedPipe }}} {
		s := streamServer()
		if c.Handler != nil {
			s.dispatcher.Handlers["initialize"] = c.Handler
		}
		a, b := net.Pipe()
		done := make(chan error, 1)
		go func() { done <- s.ServeTCPConnection(context.Background(), a, c.Options) }()
		_ = b.SetDeadline(time.Now().Add(time.Second))
		_, _ = io.WriteString(b, c.Input)
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("bad input accepted")
			}
		case <-time.After(time.Second):
			t.Fatal("blocked")
		}
		_ = b.Close()
	}
	// Client abandons the response while handler is running.
	s := streamServer()
	entered, release := make(chan struct{}), make(chan struct{})
	s.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) {
		close(entered)
		<-release
		return nil, nil, nil
	}
	a, b := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- s.ServeTCPConnection(context.Background(), a, DefaultTCPOptions(SyncTCP)) }()
	_, _ = io.WriteString(b, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n")
	<-entered
	_ = b.Close()
	close(release)
	if err := <-done; err == nil {
		t.Fatal("write failure lost")
	}
	// A peer EOF is not an error.
	a, b = net.Pipe()
	_ = b.Close()
	if err := streamServer().ServeTCPConnection(context.Background(), a, TCPOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestTCPListenerShutdownAndFinalLine(t *testing.T) {
	s := streamServer()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.ServeTCP(ctx, listener, DefaultTCPOptions(SyncTCP)) }()
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = client.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = io.WriteString(client, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	_ = client.(*net.TCPConn).CloseWrite()
	reply, err := bufio.NewReader(client).ReadString('\n')
	if err != nil || !strings.Contains(reply, `"transport":"tcp"`) {
		t.Fatal(reply, err)
	}
	_ = client.Close()
	other, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	_ = other.SetReadDeadline(time.Now().Add(time.Second))
	_, _ = other.Read(make([]byte, 1))
	_ = other.Close()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener leak")
	}
	if err := s.ServeTCP(context.Background(), badListener{io.ErrClosedPipe}, TCPOptions{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := s.ServeTCP(context.Background(), badListener{}, TCPOptions{Mode: 9}); err == nil {
		t.Fatal("invalid listener options")
	}
}

func TestFileReferenceFixtures(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-file.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input    []byte `json:"input_bytes"`
		Expected []json.RawMessage
		Error    bool
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		s := streamServer()
		var output bytes.Buffer
		err = s.ServeFile(context.Background(), bytes.NewReader(c.Input), &output)
		if (err != nil) != c.Error {
			t.Fatal(err, c)
		}
		decoder := json.NewDecoder(&output)
		var actual []json.RawMessage
		for {
			var value json.RawMessage
			e := decoder.Decode(&value)
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			actual = append(actual, value)
		}
		if len(actual) != len(c.Expected) {
			t.Fatal(len(actual), len(c.Expected))
		}
		for i, v := range actual {
			a, _ := decode(v)
			b, _ := decode(c.Expected[i])
			aa, _ := json.Marshal(a)
			bb, _ := json.Marshal(b)
			if string(aa) != string(bb) {
				t.Fatal(string(aa), string(bb))
			}
		}
	}
}

func TestTCPPythonWireFixtures(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-tcp.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Mode  string
		Input struct {
			Name           string
			Prefix, Suffix []byte
			Padding, Chunk int
		}
		Expected []json.RawMessage
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Mode+"/"+c.Input.Name, func(t *testing.T) {
			mode := SyncTCP
			if c.Mode == "async" {
				mode = AsyncTCP
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- streamServer().ServeTCP(ctx, listener, DefaultTCPOptions(mode)) }()
			defer func() {
				cancel()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Error("server leaked")
				}
			}()
			client, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			_ = client.SetDeadline(time.Now().Add(3 * time.Second))
			payload := append(append(append([]byte{}, c.Input.Prefix...), bytes.Repeat([]byte(" "), c.Input.Padding)...), c.Input.Suffix...)
			chunk := c.Input.Chunk
			if chunk == 0 {
				chunk = max(1, len(payload))
			}
			for i := 0; i < len(payload); i += chunk {
				if _, err = client.Write(payload[i:min(i+chunk, len(payload))]); err != nil {
					t.Fatal(err)
				}
			}
			_ = client.(*net.TCPConn).CloseWrite()
			output, err := io.ReadAll(client)
			// Closing with unread oversized input can yield RST on either
			// implementation. The oracle likewise compares bytes before close.
			if err != nil && !errors.Is(err, syscall.ECONNRESET) {
				t.Fatal(err)
			}
			actual := []any{}
			decoder := json.NewDecoder(bytes.NewReader(output))
			decoder.UseNumber()
			for {
				var value any
				err = decoder.Decode(&value)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				actual = append(actual, value)
			}
			expected := []any{}
			for _, v := range c.Expected {
				value, ok := decode(v)
				if !ok {
					t.Fatal("fixture JSON")
				}
				expected = append(expected, value)
			}
			a, _ := json.Marshal(actual)
			b, _ := json.Marshal(expected)
			if string(a) != string(b) {
				t.Fatalf("wire mismatch: %s != %s", a, b)
			}
		})
	}
}

// A deterministic clock-free check that each Read/Write sets its own deadline,
// including after handler execution; handler time must not consume I/O timeout.
type trackedTimeoutConn struct {
	net.Conn
	readTimes, writeTimes []time.Time
}

func (c *trackedTimeoutConn) SetReadDeadline(d time.Time) error {
	c.readTimes = append(c.readTimes, d)
	return nil
}
func (c *trackedTimeoutConn) SetWriteDeadline(d time.Time) error {
	c.writeTimes = append(c.writeTimes, d)
	return nil
}
func TestTCPSocketTimeoutPerOperation(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	conn := &trackedTimeoutConn{Conn: a}
	entered, release := make(chan struct{}), make(chan struct{})
	s := streamServer()
	s.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) {
		close(entered)
		<-release
		return "done", nil, nil
	}
	done := make(chan error, 1)
	go func() { done <- s.ServeTCPConnection(context.Background(), conn, DefaultTCPOptions(SyncTCP)) }()
	_ = b.SetDeadline(time.Now().Add(3 * time.Second))
	_, err := io.WriteString(b, `{"jsonrpc":"2.0","id":1,`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.WriteString(b, `"method":"initialize"}`+"\n")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	before := time.Now()
	close(release)
	if _, err = bufio.NewReader(b).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	_ = b.Close()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if len(conn.readTimes) < 2 || len(conn.writeTimes) != 1 {
		t.Fatal("missing per-I/O deadlines", conn.readTimes, conn.writeTimes)
	}
	if conn.writeTimes[0].Before(before.Add(30 * time.Second)) {
		t.Fatal("deadline includes handler time")
	}
	a, b = net.Pipe()
	defer a.Close()
	defer b.Close()
	if _, err = (socketTimeout{Conn: deadlineConn{a, io.ErrClosedPipe}, timeout: time.Second}).Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	// An idle timeout closes the client instead of passing a partial line to the dispatcher.
	a, b = net.Pipe()
	defer b.Close()
	err = s.ServeTCPConnection(context.Background(), a, TCPOptions{IOTimeout: time.Millisecond})
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatal(err)
	}
}

func TestTCPConcurrentContextAndCancellation(t *testing.T) {
	s := streamServer()
	arrived := make(chan RequestContext, 2)
	s.dispatcher.Handlers["initialize"] = func(ctx context.Context, _ map[string]any) (any, *RPCError, error) {
		arrived <- Context(ctx)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.ServeTCP(ctx, listener, DefaultTCPOptions(SyncTCP)) }()
	peers := map[string]bool{}
	for range 2 {
		client, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		peers[client.LocalAddr().String()] = true
		_, err = io.WriteString(client, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n")
		if err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		select {
		case c := <-arrived:
			if c.Transport != "tcp" || !peers[c.Peer] || c.Principal != "" || c.SessionID != "" {
				t.Fatal(c)
			}
			delete(peers, c.Peer)
		case <-time.After(2 * time.Second):
			t.Fatal("clients were not concurrent")
		}
	}
	cancel()
	select {
	case err = <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cooperative handlers not cancelled")
	}
}

func FuzzStreamLine(f *testing.F) {
	for _, v := range []string{"", "\xff\n", "abc\r\n", strings.Repeat("a", 4096)} {
		f.Add(v, uint16(16), false)
	}
	f.Fuzz(func(t *testing.T, input string, limit uint16, strict bool) {
		reader := bufio.NewReader(strings.NewReader(input))
		line, err := readLine(reader, int(limit))
		if err == nil || err == io.EOF {
			_, _ = lineText(line, strict)
			if limit > 0 && len(strings.TrimSuffix(line, "\n")) > int(limit) {
				t.Fatal("limit exceeded")
			}
		}
	})
}

// Notify delivery is a server sink, not a per-connection reply. Guard against
// accidentally moving progress onto TCP during later transport refactors.
func TestTCPNotificationsKeepServerSink(t *testing.T) {
	s := streamServer()
	var mu sync.Mutex
	var notifications []string
	s.SetNotifier(func(method string, _ map[string]any) error {
		mu.Lock()
		defer mu.Unlock()
		notifications = append(notifications, method)
		return nil
	})
	s.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) {
		return "done", nil, s.send("notifications/progress", map[string]any{"progress": 1})
	}
	a, b := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- s.ServeTCPConnection(context.Background(), a, TCPOptions{}) }()
	_ = b.SetDeadline(time.Now().Add(time.Second))
	_, _ = io.WriteString(b, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n")
	line, err := bufio.NewReader(b).ReadString('\n')
	if err != nil || strings.Contains(line, "notifications") {
		t.Fatal(line, err)
	}
	_ = b.Close()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(notifications) != 1 || notifications[0] != "notifications/progress" {
		t.Fatal(notifications)
	}
}
