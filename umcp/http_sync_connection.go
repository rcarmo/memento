package umcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SyncHTTPConnectionOptions preserves BaseHTTPRequestHandler connection policy.
// Legacy SSE defaults to HTTP/1.0 without a socket timeout; Streamable HTTP
// uses a 30-second per-blocking-operation timeout and 1000 requests per peer.
type SyncHTTPConnectionOptions struct {
	Legacy      bool
	IOTimeout   time.Duration
	MaxRequests int
}

func DefaultSyncHTTPConnectionOptions(legacy bool) SyncHTTPConnectionOptions {
	if legacy {
		return SyncHTTPConnectionOptions{Legacy: true, MaxRequests: 1000}
	}
	return SyncHTTPConnectionOptions{IOTimeout: 30 * time.Second, MaxRequests: 1000}
}

type syncRequestKey struct{}

func syncRequest(r *http.Request) *SyncHTTPRequest {
	p, _ := r.Context().Value(syncRequestKey{}).(*SyncHTTPRequest)
	return p
}
func forceSyncClose(w http.ResponseWriter, r *http.Request) {
	if syncRequest(r) != nil {
		w.Header().Set("Connection", "close")
	}
}

func (p *SyncHTTPRequest) request(ctx context.Context, reader *bufio.Reader, remote string) *http.Request {
	parsed := &ParsedHTTPRequest{Method: p.Method, Target: p.Target, Version: p.Version, Headers: p.First}
	r := parsed.request(context.WithValue(ctx, syncRequestKey{}, p), remote)
	// Body consumption is deferred to the chosen route. Large/invalid lengths
	// never cause an eager allocation, and a skipped body remains pipelined input.
	value, ok := p.First["content-length"]
	if !ok {
		value = "0"
	}
	size, status := httpContentLength(value, math.MaxInt64)
	if status == 400 {
		size = -1
	} else if status == 413 {
		size = math.MaxInt64
	}
	r.ContentLength = size
	r.Body = io.NopCloser(io.LimitReader(reader, max(0, size)))
	return r
}

// ServeSyncHTTPConnection owns conn and mirrors the sync Python transport's
// route-dependent reads, interim Continue, HTML errors and connection policy.
// Use a synchronous StreamableHTTP or SyncSSE LegacySSE handler.
func ServeSyncHTTPConnection(ctx context.Context, conn net.Conn, handler http.Handler, options SyncHTTPConnectionOptions) error {
	defer conn.Close()
	if handler == nil || options.IOTimeout < 0 || options.MaxRequests <= 0 {
		return errors.New("invalid sync HTTP connection options")
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	stream := conn
	if options.IOTimeout > 0 {
		stream = socketTimeout{Conn: conn, timeout: options.IOTimeout}
	}
	reader := bufio.NewReader(stream)
	for number := 1; ; number++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		parsed, err := ParseSyncHTTPRequest(reader, options.Legacy)
		if err != nil {
			var failure *SyncHTTPParseError
			if errors.As(err, &failure) && failure.Status != 0 {
				w := newSyncWireWriter(stream, options.Legacy, failure.Version, false, false)
				w.writeError(failure, "")
				if writeErr := w.FlushError(); writeErr != nil {
					return writeErr
				}
			}
			return err
		}
		w := newSyncWireWriter(stream, options.Legacy, parsed.Version, parsed.KeepAlive, !options.Legacy && number >= options.MaxRequests)
		if parsed.ExpectContinue {
			if _, err = io.WriteString(stream, "HTTP/1.1 100 Continue\r\n\r\n"); err != nil {
				return err
			}
		}
		supported := parsed.Method == "GET" || parsed.Method == "POST" || parsed.Method == "OPTIONS" || (!options.Legacy && parsed.Method == "DELETE")
		if !supported {
			w.writeError(syncHTTPError(501, "Unsupported method ("+pythonRepr(parsed.Method)+")", "", parsed.Version), parsed.Method)
		} else {
			handler.ServeHTTP(w, parsed.request(ctx, reader, conn.RemoteAddr().String()))
		}
		if err = w.FlushError(); err != nil {
			return err
		}
		if !w.keepAlive || w.streaming {
			return nil
		}
	}
}

func ServeSyncHTTP(ctx context.Context, listener net.Listener, handler http.Handler, options SyncHTTPConnectionOptions) error {
	if handler == nil || options.IOTimeout < 0 || options.MaxRequests <= 0 {
		_ = listener.Close()
		return errors.New("invalid sync HTTP server options")
	}
	return serveConnections(ctx, listener, func(ctx context.Context, conn net.Conn) { _ = ServeSyncHTTPConnection(ctx, conn, handler, options) })
}

type syncWireWriter struct {
	*httpWireWriter
	legacySync, forceClose bool
	statusLine             string
}

func newSyncWireWriter(output io.Writer, legacy bool, version string, keepAlive, forceClose bool) *syncWireWriter {
	return &syncWireWriter{httpWireWriter: newHTTPWireWriter(output, keepAlive, false, version), legacySync: legacy, forceClose: forceClose}
}
func (w *syncWireWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.streaming = w.header.Get("Content-Type") == "text/event-stream"
	if strings.EqualFold(w.header.Get("Connection"), "close") {
		w.keepAlive = false
	} else if strings.EqualFold(w.header.Get("Connection"), "keep-alive") {
		w.keepAlive = true
	}
	if !w.legacySync && !w.streaming {
		if w.forceClose {
			w.keepAlive = false
		}
		if !w.keepAlive {
			w.header.Set("Connection", "close")
		} else if w.version == "HTTP/1.0" {
			w.header.Set("Connection", "keep-alive")
		}
	}
	if w.version == "HTTP/0.9" {
		return
	}
	protocol := "HTTP/1.1"
	if w.legacySync {
		protocol = "HTTP/1.0"
	}
	statusLine := w.statusLine
	if statusLine == "" {
		statusLine = HTTPStatusLine(status)
	}
	var head strings.Builder
	head.WriteString(protocol + " " + statusLine + "\r\nServer: BaseHTTP/0.6 Python/3.13.14\r\nDate: " + time.Now().UTC().Format(http.TimeFormat) + "\r\n")
	for key, values := range w.header {
		for _, value := range values {
			if strings.ContainsAny(key+value, "\r\n") {
				w.err = errors.New("invalid response header")
				return
			}
			head.WriteString(key + ": " + value + "\r\n")
		}
	}
	head.WriteString("\r\n")
	raw, err := encodeLatin1(head.String())
	if err != nil {
		w.err = err
		return
	}
	_, w.err = w.writer.Write(raw)
}
func encodeLatin1(text string) ([]byte, error) {
	raw := make([]byte, 0, len(text))
	for _, r := range text {
		if r > 255 {
			return nil, errors.New("response header is not Latin-1")
		}
		raw = append(raw, byte(r))
	}
	return raw, nil
}
func (w *syncWireWriter) Write(p []byte) (int, error) {
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
func (w *syncWireWriter) Flush() { _ = w.FlushError() }
func (w *syncWireWriter) FlushError() error {
	if !w.wrote {
		w.header.Set("Content-Length", "0")
		w.WriteHeader(200)
	}
	if w.err != nil {
		return w.err
	}
	w.err = w.writer.Flush()
	return w.err
}
func (w *syncWireWriter) writeError(failure *SyncHTTPParseError, method string) {
	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	body := fmt.Sprintf("<!DOCTYPE HTML>\n<html lang=\"en\">\n    <head>\n        <meta charset=\"utf-8\">\n        <title>Error response</title>\n    </head>\n    <body>\n        <h1>Error response</h1>\n        <p>Error code: %d</p>\n        <p>Message: %s.</p>\n        <p>Error code explanation: %d - %s.</p>\n    </body>\n</html>\n", failure.Status, escape.Replace(failure.Message), failure.Status, escape.Replace(failure.Explain))
	w.statusLine = strconv.Itoa(failure.Status) + " " + failure.Message
	w.Header().Set("Connection", "close")
	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(failure.Status)
	if method != "HEAD" {
		_, _ = w.Write([]byte(body))
	}
}
