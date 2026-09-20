package umcp

import (
	"bufio"
	"errors"
	"io"
	"math/big"
	"strings"
	"time"
	"unicode"
)

// AsyncHTTPParserMode selects the pinned asyncio parsers. The synchronous
// Python transports use BaseHTTPRequestHandler and require a separate parser.
type AsyncHTTPParserMode uint8

const (
	AsyncStreamableParser AsyncHTTPParserMode = iota
	AsyncSSEParser
)

// AsyncHTTPParserOptions includes per-line and whole-body read timeouts. The
// deadline hook is optional for in-memory inputs; network callers must supply
// conn.SetReadDeadline. Cancellation/connection ownership belongs to the server.
type AsyncHTTPParserOptions struct {
	Mode            AsyncHTTPParserMode
	MaxRequestBytes int64
	ReadTimeout     time.Duration
	SetReadDeadline func(time.Time) error
	AllowedOrigins  []string
	LocalBind       bool
}

func DefaultAsyncHTTPParserOptions(mode AsyncHTTPParserMode) AsyncHTTPParserOptions {
	return AsyncHTTPParserOptions{Mode: mode, MaxRequestBytes: 4 << 20, ReadTimeout: 30 * time.Second, LocalBind: true}
}

// ParsedHTTPRequest preserves unescaped targets and Python's normalised
// headers, including names/values that net/http would reject or rewrite.
type ParsedHTTPRequest struct {
	Method, Target, Version string
	Headers                 map[string]string
	HeaderCounts            map[string]int
	Body                    []byte
	KeepAlive               bool
	Origin                  string
}

// HTTPParseError gives the source HTTP status and CORS origin. Status zero
// means close without a response (EOF/timeout before a Streamable request, or
// asyncio's unhandled line-limit exception). All parse failures close the peer.
type HTTPParseError struct {
	Status int
	Origin string
}

func (e *HTTPParseError) Error() string { return "HTTP parse failure: " + HTTPStatusLine(e.Status) }

func pythonSpace(r rune) bool         { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }
func pythonStrip(value string) string { return strings.TrimFunc(value, pythonSpace) }
func latin1(raw string) string {
	runes := make([]rune, len(raw))
	for i := 0; i < len(raw); i++ {
		runes[i] = rune(raw[i])
	}
	return string(runes)
}
func ascii(raw string) bool {
	for i := range raw {
		if raw[i] > 127 {
			return false
		}
	}
	return true
}

// ParseAsyncHTTPRequest reads exactly one request, leaving pipelined bytes in
// reader. It matches the two pinned parsers rather than net/http's RFC policy.
func ParseAsyncHTTPRequest(reader *bufio.Reader, options AsyncHTTPParserOptions) (*ParsedHTTPRequest, error) {
	if options.Mode > AsyncSSEParser || options.MaxRequestBytes <= 0 || options.ReadTimeout <= 0 {
		return nil, errors.New("invalid HTTP parser options")
	}
	legacy := options.Mode == AsyncSSEParser
	fail := func(status int, origin string) (*ParsedHTTPRequest, error) {
		return nil, &HTTPParseError{Status: status, Origin: origin}
	}
	deadline := func() error {
		if options.SetReadDeadline != nil {
			return options.SetReadDeadline(time.Now().Add(options.ReadTimeout))
		}
		return nil
	}
	line := func() (string, error) {
		if err := deadline(); err != nil {
			return "", err
		}
		return readLine(reader, 65536)
	}
	requestLine, err := line()
	if err != nil && err != io.EOF || requestLine == "" {
		if legacy {
			return fail(400, "")
		}
		return fail(0, "")
	}
	if len(requestLine) > 8192 {
		if legacy {
			return fail(400, "")
		}
		return fail(414, "")
	}
	if !ascii(requestLine) {
		return fail(400, "")
	}
	var parts []string
	if legacy {
		parts = strings.Split(pythonStrip(requestLine), " ")
	} else {
		parts = strings.FieldsFunc(requestLine, pythonSpace)
	}
	if len(parts) != 3 || (parts[2] != "HTTP/1.0" && parts[2] != "HTTP/1.1") {
		return fail(400, "")
	}
	request := &ParsedHTTPRequest{Method: parts[0], Target: parts[1], Version: parts[2], Headers: map[string]string{}, HeaderCounts: map[string]int{}}
	headerBytes := 0
	var headerLines []string
	// SSE's for range(100) includes the blank terminator; Streamable permits
	// 100 fields and checks its count at the next read (including terminator).
	for count := 0; ; count++ {
		if legacy && count == 100 {
			return fail(400, "")
		}
		raw, err := line()
		if err != nil && err != io.EOF {
			if !legacy && errors.Is(err, errLineLimit) {
				return fail(0, "")
			}
			return fail(400, "")
		}
		headerBytes += len(raw)
		if raw == "" || headerBytes > 65536 || (!legacy && count > 100) {
			return fail(400, "")
		}
		if raw == "\r\n" || raw == "\n" {
			break
		}
		if !legacy && (raw[0] == ' ' || raw[0] == '\t') {
			return fail(400, "")
		}
		if legacy {
			if !parseHTTPHeader(request, raw, true) {
				return fail(400, "")
			}
		} else {
			// The Streamable parser validates field syntax only after all
			// lines arrive. Do not mask a later transport/line-limit failure.
			headerLines = append(headerLines, raw)
		}
	}
	if !legacy {
		for _, raw := range headerLines {
			if !parseHTTPHeader(request, raw, false) {
				return fail(400, "")
			}
		}
		connection := map[string]bool{}
		for _, token := range strings.Split(request.Headers["connection"], ",") {
			connection[strings.ToLower(pythonStrip(token))] = true
		}
		request.KeepAlive = !connection["close"] && (request.Version == "HTTP/1.1" || connection["keep-alive"])
		origin := request.Headers["origin"]
		if origin != "" && request.HeaderCounts["origin"] == 1 {
			allowed, err := OriginIsAllowed(origin, options.AllowedOrigins, options.LocalBind, request.Headers["host"])
			if err == nil && allowed {
				request.Origin = origin
			}
		}
	}
	_, transfer := request.Headers["transfer-encoding"]
	if HasSingletonHeaderViolations(request.HeaderCounts, request.Version) || HasAmbiguousSingletonValues(request.Headers) || (!legacy && transfer) {
		return fail(400, request.Origin)
	}
	if !legacy && request.Headers["origin"] != "" && request.Origin == "" {
		return fail(403, "")
	}
	length, exists := request.Headers["content-length"]
	if !exists || legacy && length == "" {
		length = "0"
	}
	size, status := httpContentLength(length, options.MaxRequestBytes)
	if status != 0 {
		return fail(status, request.Origin)
	}
	request.Body = []byte{}
	if size > 0 {
		if err := deadline(); err != nil {
			return fail(400, request.Origin)
		}
		// Bound allocation by available input, not an untrusted advertised length.
		body, err := io.ReadAll(io.LimitReader(reader, size))
		if err != nil || int64(len(body)) != size {
			return fail(400, request.Origin)
		}
		request.Body = body
	}
	return request, nil
}

func parseHTTPHeader(request *ParsedHTTPRequest, raw string, legacy bool) bool {
	key, value, ok := strings.Cut(raw, ":")
	if !ok {
		return false
	}
	if legacy {
		key = latin1(key)
	} else if !ascii(key) {
		return false
	}
	key = strings.ToLower(pythonStrip(key))
	value = pythonStrip(latin1(value))
	if !legacy && key == "" {
		return false
	}
	request.HeaderCounts[key]++
	if request.HeaderCounts[key] > 1 && !legacy {
		request.Headers[key] += ", " + value
	} else {
		request.Headers[key] = value
	}
	return true
}

func httpContentLength(value string, maxBytes int64) (int64, int) {
	// int() strips Unicode whitespace, but not Python str.strip's additional
	// U+001C..U+001F separators. Values have already passed header str.strip.
	value = normalizeDecimalDigits(strings.TrimSpace(value))
	if !pythonInt.MatchString(value) {
		return 0, 400
	}
	value = strings.ReplaceAll(value, "_", "")
	if len(strings.TrimLeft(value, "+-")) > 4300 {
		return 0, 400
	}
	size, ok := new(big.Int).SetString(value, 10)
	if !ok || size.Sign() < 0 {
		return 0, 400
	}
	if size.Cmp(big.NewInt(maxBytes)) > 0 {
		return 0, 413
	}
	return size.Int64(), 0
}
