package umcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// AsyncHTTPConnectionOptions supplies the reference parser and connection
// lifetime defaults. Legacy SSE always serves one request per connection.
type AsyncHTTPConnectionOptions struct {
	Parser      AsyncHTTPParserOptions
	MaxRequests int
}

func DefaultAsyncHTTPConnectionOptions(mode AsyncHTTPParserMode) AsyncHTTPConnectionOptions {
	return AsyncHTTPConnectionOptions{Parser: DefaultAsyncHTTPParserOptions(mode), MaxRequests: 1000}
}

// ServeAsyncHTTPConnection owns conn. Use with an AsyncReference StreamableHTTP
// or AsyncSSE LegacySSE handler configured with the same limits/origins. This
// adapter preserves raw parser semantics; net/http.Server must not parse first.
// Closing/cancelling the connection interrupts I/O and cooperative requests.
// There is no write timeout in the async reference; cancellation closes a peer
// that stops reading. Handler deadlines must be supplied by the application.
func ServeAsyncHTTPConnection(ctx context.Context, conn net.Conn, handler http.Handler, options AsyncHTTPConnectionOptions) error {
	defer conn.Close()
	if handler == nil || options.MaxRequests <= 0 || options.Parser.Mode > AsyncSSEParser || options.Parser.MaxRequestBytes <= 0 || options.Parser.ReadTimeout <= 0 {
		return errors.New("invalid async HTTP connection options")
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	options.Parser.SetReadDeadline = conn.SetReadDeadline
	reader := bufio.NewReader(conn)
	legacy := options.Parser.Mode == AsyncSSEParser
	for number := 1; ; number++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		parsed, err := ParseAsyncHTTPRequest(reader, options.Parser)
		if err != nil {
			var failure *HTTPParseError
			if errors.As(err, &failure) && failure.Status != 0 {
				output := newHTTPWireWriter(conn, false, legacy, "HTTP/1.1")
				if legacy {
					sseEmpty(output, failure.Status, failure.Origin)
				} else {
					emptyHTTP(output, failure.Status, failure.Origin)
				}
				if writeErr := output.FlushError(); writeErr != nil {
					return writeErr
				}
			}
			return err
		}
		request := parsed.request(ctx, conn.RemoteAddr().String())
		output := newHTTPWireWriter(conn, parsed.KeepAlive && number < options.MaxRequests, legacy, parsed.Version)
		handler.ServeHTTP(output, request)
		if err = output.FlushError(); err != nil {
			return err
		}
		if !output.keepAlive || legacy || output.streaming {
			return nil
		}
	}
}

type rawTargetKey struct{}

func requestTarget(r *http.Request) string {
	if target, ok := r.Context().Value(rawTargetKey{}).(string); ok {
		return target
	}
	return r.URL.RequestURI()
}

func (p *ParsedHTTPRequest) request(ctx context.Context, remote string) *http.Request {
	headers := http.Header{}
	for key, value := range p.Headers {
		if key != "host" || value == "" {
			headers[http.CanonicalHeaderKey(key)] = []string{value}
		}
	}
	// Opaque preserves targets and percent escapes byte-for-byte, including
	// malformed escapes which Python deliberately leaves for route parsing.
	target, query, hasQuery := strings.Cut(p.Target, "?")
	return (&http.Request{Method: p.Method, URL: &url.URL{Opaque: target, RawQuery: query, ForceQuery: hasQuery}, RequestURI: p.Target, Proto: p.Version, ProtoMajor: 1, ProtoMinor: int(p.Version[len(p.Version)-1] - '0'), Header: headers, Host: p.Headers["host"], RemoteAddr: remote, ContentLength: int64(len(p.Body)), Body: io.NopCloser(strings.NewReader(string(p.Body)))}).WithContext(context.WithValue(ctx, rawTargetKey{}, p.Target))
}

// httpWireWriter emits unchunked source framing. Each handler must supply a
// Content-Length for finite responses (the uMCP handlers do), or an SSE stream.
// Streaming flush errors must reach handlers, particularly async SSE POST acks.
type httpWireWriter struct {
	writer                              *bufio.Writer
	header                              http.Header
	keepAlive, legacy, streaming, wrote bool
	version                             string
	err                                 error
}

func newHTTPWireWriter(output io.Writer, keepAlive, legacy bool, version string) *httpWireWriter {
	return &httpWireWriter{writer: bufio.NewWriter(output), header: http.Header{}, keepAlive: keepAlive, legacy: legacy, version: version}
}
func (w *httpWireWriter) Header() http.Header { return w.header }
func (w *httpWireWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.streaming = w.header.Get("Content-Type") == "text/event-stream"
	if !w.legacy {
		w.header.Del("Connection")
		if !w.keepAlive {
			w.header.Set("Connection", "close")
		} else if w.version == "HTTP/1.0" {
			w.header.Set("Connection", "keep-alive")
		}
	}
	if w.header.Get("Content-Length") == "" && !w.streaming {
		w.keepAlive = false
		w.header.Set("Connection", "close")
	}
	statusLine := HTTPStatusLine(status)
	if status == 413 {
		statusLine = "413 Payload Too Large"
	}
	// Fields originate from trusted hooks or fixed handlers and must not split
	// responses. This guard also protects custom embedding handlers.
	for key, values := range w.header {
		for _, value := range values {
			if strings.ContainsAny(key+value, "\r\n") {
				w.err = errors.New("invalid response header")
				return
			}
		}
	}
	var head strings.Builder
	head.WriteString("HTTP/1.1 " + statusLine + "\r\n")
	keys := make([]string, 0, len(w.header))
	for key := range w.header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range w.header[key] {
			head.WriteString(key + ": " + value + "\r\n")
		}
	}
	head.WriteString("\r\n")
	_, w.err = w.writer.WriteString(head.String())
}
func (w *httpWireWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(200)
	}
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.writer.Write(p)
	w.err = err
	return n, err
}
func (w *httpWireWriter) Flush() { _ = w.FlushError() }
func (w *httpWireWriter) FlushError() error {
	if !w.wrote {
		w.header.Set("Content-Length", strconv.Itoa(0))
		w.WriteHeader(200)
	}
	if w.err != nil {
		return w.err
	}
	w.err = w.writer.Flush()
	return w.err
}

// ServeAsyncHTTP accepts concurrent connections and waits for cooperative
// handlers at shutdown. Connection parse/I/O failures do not stop the listener.
func ServeAsyncHTTP(ctx context.Context, listener net.Listener, handler http.Handler, options AsyncHTTPConnectionOptions) error {
	if handler == nil || options.MaxRequests <= 0 || options.Parser.Mode > AsyncSSEParser || options.Parser.MaxRequestBytes <= 0 || options.Parser.ReadTimeout <= 0 {
		_ = listener.Close()
		return fmt.Errorf("invalid async HTTP server options")
	}
	return serveConnections(ctx, listener, func(ctx context.Context, conn net.Conn) { _ = ServeAsyncHTTPConnection(ctx, conn, handler, options) })
}
