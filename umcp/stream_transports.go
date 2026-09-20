package umcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// TCPMode selects one of the pinned Python reference transport behaviours.
// Sync TCP replaces invalid UTF-8 and has a 30-second I/O timeout. Async TCP
// rejects invalid UTF-8, has no socket timeout and limits lines to 64 KiB.
type TCPMode uint8

const (
	SyncTCP TCPMode = iota
	AsyncTCP
)

// TCPOptions describes framing policy, not authentication. uMCP's original raw
// TCP transport does not authenticate or bind a principal. Keep it on loopback
// unless an independently reviewed trusted transport supplies access control.
type TCPOptions struct {
	Mode         TCPMode
	IOTimeout    time.Duration
	MaxLineBytes int
}

// DefaultTCPOptions returns the defaults of the chosen reference mode.
func DefaultTCPOptions(mode TCPMode) TCPOptions {
	if mode == AsyncTCP {
		return TCPOptions{Mode: mode, MaxLineBytes: 65536}
	}
	return TCPOptions{Mode: mode, IOTimeout: 30 * time.Second}
}

func lineText(line string, strict bool) (string, error) {
	if strict && !utf8.ValidString(line) {
		return "", errors.New("invalid UTF-8 in TCP request")
	}
	return strings.TrimFunc(replaceInvalidUTF8(line), func(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }), nil
}

var errLineLimit = errors.New("request line exceeds reader limit")

// readLine preserves Python readline's final unterminated line. Asyncio's limit
// excludes the newline byte itself. Zero limit preserves the unbounded sync
// behaviour; use a separately reviewed deployment cap for untrusted streams.
func readLine(reader *bufio.Reader, limit int) (string, error) {
	var out strings.Builder
	for {
		fragment, err := reader.ReadSlice('\n')
		length := out.Len() + len(fragment)
		if len(fragment) > 0 && fragment[len(fragment)-1] == '\n' {
			length--
		}
		if limit > 0 && length > limit {
			return "", errLineLimit
		}
		out.Write(fragment)
		if err == bufio.ErrBufferFull {
			continue
		}
		return out.String(), err
	}
}

func encodeLine(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	if flusher, ok := output.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// socketTimeout gives each blocking operation a fresh timeout, like Python's
// socket.settimeout. A deadline spanning parsing/handler execution would reject
// a valid response after a slow handler or a line delivered in small fragments.
type socketTimeout struct {
	net.Conn
	timeout time.Duration
}

func (c socketTimeout) Read(p []byte) (int, error) {
	if err := c.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}

func (c socketTimeout) Write(p []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}

// ServeTCPConnection owns and closes conn, even on handler errors. Requests are
// sequential per connection but different connections may run concurrently.
// Notifications retain the server's configured sink (stdout in the reference),
// not the TCP connection. Cancellation closes blocked I/O without leaking it.
func (s *Server) ServeTCPConnection(ctx context.Context, conn net.Conn, options TCPOptions) error {
	defer conn.Close()
	if options.Mode != SyncTCP && options.Mode != AsyncTCP || options.MaxLineBytes < 0 || options.IOTimeout < 0 {
		return errors.New("invalid TCP options")
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	stream := conn
	if options.IOTimeout > 0 {
		stream = socketTimeout{Conn: conn, timeout: options.IOTimeout}
	}
	reader := bufio.NewReader(stream)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, readErr := readLine(reader, options.MaxLineBytes)
		// Python readline raises on limit/timeout errors before dispatching any
		// partial request, but processes the final nonempty line on plain EOF.
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		if len(line) > 0 {
			text, err := lineText(line, options.Mode == AsyncTCP)
			if err != nil {
				return err
			}
			if text != "" {
				response, err := s.Process(ctx, []byte(text), RequestContext{Transport: "tcp", Peer: tcpPeer(conn.RemoteAddr())})
				if err != nil {
					return err
				}
				if response != nil {
					if err = encodeLine(stream, response); err != nil {
						return err
					}
				}
			}
		}
		if readErr == io.EOF {
			return nil
		}
	}
}

func tcpPeer(address net.Addr) string {
	host, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return address.String()
	}
	// Python formats IPv6 socket tuples without brackets here.
	return host + ":" + port
}

// ServeTCP owns listener until shutdown, closes live client connections on
// cancellation, and waits for their handlers to finish. Connection errors are
// isolated as in the source; cancellation/deadlines inside handlers remain
// cooperative. Application code must not ignore context indefinitely.
func (s *Server) ServeTCP(ctx context.Context, listener net.Listener, options TCPOptions) error {
	if options.Mode != SyncTCP && options.Mode != AsyncTCP || options.IOTimeout < 0 || options.MaxLineBytes < 0 {
		_ = listener.Close()
		return errors.New("invalid TCP options")
	}
	return serveConnections(ctx, listener, func(ctx context.Context, conn net.Conn) { _ = s.ServeTCPConnection(ctx, conn, options) })
}

func serveConnections(ctx context.Context, listener net.Listener, serve func(context.Context, net.Conn)) error {
	defer listener.Close()
	children, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	stop := context.AfterFunc(children, func() { _ = listener.Close() })
	defer stop()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		workers.Add(1)
		go func() { defer workers.Done(); serve(children, conn) }()
	}
}

// ServeFile processes one complete UTF-8 JSON document, not a JSONL stream.
// Python text-file mode folds CRLF and CR to LF before parsing. Notifications
// use the server's configured sink; no reply is written for an acknowledged one.
// The caller owns input/output; close a blocked reader to interrupt its read.
func (s *Server) ServeFile(ctx context.Context, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	if !utf8.Valid(raw) {
		return errors.New("invalid UTF-8 in request file")
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\r", "\n")
	response, err := s.Process(ctx, []byte(text), RequestContext{Transport: "file"})
	if err != nil {
		return err
	}
	if response == nil {
		return nil
	}
	return encodeLine(output, response)
}

// ProcessFile opens and closes a local request file with explicit I/O errors.
// It never treats an MCP argument as a filename; only the host CLI calls it.
func (s *Server) ProcessFile(ctx context.Context, path string, output io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("request file: %w", err)
	}
	defer file.Close()
	return s.ServeFile(ctx, file, output)
}
