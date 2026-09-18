package umcp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAsyncHTTPConnectionPipeline(t *testing.T) {
	_, h := testHTTP(t)
	h.options.AsyncReference = true
	options := DefaultAsyncHTTPConnectionOptions(AsyncStreamableParser)
	options.MaxRequests = 2
	a, b := net.Pipe()
	defer b.Close()
	_ = b.SetDeadline(time.Now().Add(2 * time.Second))
	done := make(chan error, 1)
	go func() { done <- ServeAsyncHTTPConnection(context.Background(), a, h, options) }()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	raw := "POST /mcp HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer reader\r\nMCP-Protocol-Version: 2025-03-26\r\nContent-Type: application/json\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	writeDone := make(chan error, 1)
	go func() {
		_, err := io.WriteString(b, raw+strings.Replace(raw, "HTTP/1.1", "HTTP/1.0", 1))
		writeDone <- err
	}()
	reader := bufio.NewReader(b)
	for i := range 2 {
		reply, err := http.ReadResponse(reader, nil)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reply.Body)
		reply.Body.Close()
		if err != nil || reply.StatusCode != 200 || !strings.Contains(string(data), `"tools"`) {
			t.Fatal(reply, data, err)
		}
		if reply.Close != (i == 1) {
			t.Fatal("connection lifetime", reply.Header)
		}
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestAsyncHTTPConnectionParserAndCancellation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { emptyHTTP(w, 200, "") })
	for _, mode := range []AsyncHTTPParserMode{AsyncStreamableParser, AsyncSSEParser} {
		for _, input := range []string{"GET / HTTP/2.0\n", "GET / HTTP/1.0\n\n"} {
			a, b := net.Pipe()
			_ = b.SetDeadline(time.Now().Add(2 * time.Second))
			done := make(chan error, 1)
			options := DefaultAsyncHTTPConnectionOptions(mode)
			options.MaxRequests = 1
			go func() { done <- ServeAsyncHTTPConnection(context.Background(), a, handler, options) }()
			_, _ = io.WriteString(b, input)
			reply, err := http.ReadResponse(bufio.NewReader(b), nil)
			if err != nil {
				t.Fatal(err)
			}
			reply.Body.Close()
			err = <-done
			if strings.Contains(input, "2.0") {
				if reply.StatusCode != 400 || err == nil {
					t.Fatal(reply.StatusCode, err)
				}
			} else if reply.StatusCode != 200 || err != nil {
				t.Fatal(reply.StatusCode, err)
			}
			b.Close()
		}
	}
	a, b := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ServeAsyncHTTPConnection(ctx, a, handler, DefaultAsyncHTTPConnectionOptions(AsyncStreamableParser)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	b.Close()
	a, b = net.Pipe()
	if err := ServeAsyncHTTPConnection(context.Background(), a, nil, DefaultAsyncHTTPConnectionOptions(AsyncStreamableParser)); err == nil {
		t.Fatal("nil handler")
	}
	b.Close()
	a, b = net.Pipe()
	b.Close()
	if err := ServeAsyncHTTPConnection(context.Background(), a, handler, DefaultAsyncHTTPConnectionOptions(AsyncStreamableParser)); err == nil {
		t.Fatal("EOF result")
	}
	// Parser errors and normal replies must propagate network write failures.
	for _, bad := range []bool{false, true} {
		a, b = net.Pipe()
		done := make(chan error, 1)
		options := DefaultAsyncHTTPConnectionOptions(AsyncStreamableParser)
		go func() { done <- ServeAsyncHTTPConnection(context.Background(), writeFailureConn{a}, handler, options) }()
		input := "GET / HTTP/1.1\nHost: localhost\n\n"
		if bad {
			input = "GET / HTTP/2.0\n"
		}
		_, _ = io.WriteString(b, input)
		if err := <-done; !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
		b.Close()
	}
}

type writeFailureConn struct{ net.Conn }

func (c writeFailureConn) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestHTTPWireWriter(t *testing.T) {
	var output bytes.Buffer
	w := newHTTPWireWriter(&output, true, false, "HTTP/1.0")
	w.Header().Set("Content-Length", "2")
	w.WriteHeader(200)
	w.WriteHeader(500)
	if _, err := w.Write([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := w.FlushError(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Connection: keep-alive\r\n") || strings.Contains(output.String(), "500") {
		t.Fatal(output.String())
	}
	output.Reset()
	w = newHTTPWireWriter(&output, true, false, "HTTP/1.1")
	if _, err := w.Write([]byte("body")); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	if !strings.Contains(output.String(), "Connection: close") || w.keepAlive {
		t.Fatal(output.String())
	}
	output.Reset()
	w = newHTTPWireWriter(&output, true, false, "HTTP/1.1")
	if err := w.FlushError(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Content-Length: 0") {
		t.Fatal(output.String())
	}
	w = newHTTPWireWriter(io.Discard, false, false, "HTTP/1.1")
	w.Header().Set("X-Invalid", "one\r\ntwo")
	if _, err := w.Write(nil); err == nil {
		t.Fatal("injected header")
	}
	if err := w.FlushError(); err == nil {
		t.Fatal("forgot write error")
	}
	output.Reset()
	w = newHTTPWireWriter(&output, false, true, "HTTP/1.1")
	sseEmpty(w, 413, "")
	w.Flush()
	if !strings.HasPrefix(output.String(), "HTTP/1.1 413 Payload Too Large") {
		t.Fatal(output.String())
	}
	w = newHTTPWireWriter(failureWriter{}, false, false, "HTTP/1.1")
	w.Header().Set("X-Large", strings.Repeat("x", 5000))
	w.WriteHeader(200)
	if err := w.FlushError(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	w = newHTTPWireWriter(failureWriter{}, false, false, "HTTP/1.1")
	if _, err := w.Write(bytes.Repeat([]byte("a"), 8192)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

func TestAsyncHTTPListenerAndSSE(t *testing.T) {
	_, h := testSSE(t, AsyncSSE)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	options := DefaultAsyncHTTPConnectionOptions(AsyncSSEParser)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- ServeAsyncHTTP(ctx, listener, h, options) }()
	client := &http.Client{Timeout: 3 * time.Second}
	base := "http://" + listener.Addr().String()
	request, _ := http.NewRequest("GET", base+"/sse", nil)
	request.Header.Set("Authorization", "Bearer reader")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	kind, endpoint, err := readSSE(reader)
	if err != nil || kind != "endpoint" {
		t.Fatal(kind, endpoint, err)
	}
	request, _ = http.NewRequest("POST", base+endpoint, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	request.Header.Set("Authorization", "Bearer reader")
	request.Header.Set("Content-Type", "application/json")
	reply, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reply.Body.Close()
	if reply.StatusCode != 202 {
		t.Fatal(reply.StatusCode)
	}
	kind, data, err := readSSE(reader)
	if err != nil || kind != "message" || !strings.Contains(data, `"transport":"sse"`) {
		t.Fatal(kind, data, err)
	}
	h.Close()
	cancel()
	if err = <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = ServeAsyncHTTP(context.Background(), badListener{}, nil, options); err == nil {
		t.Fatal("invalid listener")
	}
}

func TestRawTargetAndEmptyHost(t *testing.T) {
	p := &ParsedHTTPRequest{Method: "GET", Target: "//host/path?sessionId=x%zz", Version: "HTTP/1.1", Headers: map[string]string{"host": ""}}
	request := p.request(context.Background(), "127.0.0.1:4")
	if requestTarget(request) != p.Target || request.URL.RawQuery != "sessionId=x%zz" || badHeaders(request) {
		t.Fatal(request, requestTarget(request))
	}
}

func TestStreamableFlushFailure(t *testing.T) {
	_, h := testHTTP(t)
	id := initialiseHTTP(t, h)
	for _, fail := range []int{1, 2} {
		r, _ := http.NewRequest("GET", "http://localhost/mcp", nil)
		r.Header.Set("Authorization", "Bearer reader")
		r.Header.Set("MCP-Protocol-Version", "2025-03-26")
		r.Header.Set("Mcp-Session-Id", id)
		writer := &failingSSEFlush{failFlush: fail}
		h.ServeHTTP(writer, r)
		if writer.flushes != fail {
			t.Fatal("flush failure not propagated")
		}
	}
}
