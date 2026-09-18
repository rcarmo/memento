package umcp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// SyncHTTPRequest is the BaseHTTPRequestHandler parse result, before uMCP's
// route-dependent body reads. Headers keeps duplicates/order; First and Last
// model Python Message.get and dict(Message.items()) respectively.
type SyncHTTPRequest struct {
	Method, Target, Version   string
	Headers                   [][2]string
	First, Last               map[string]string
	Counts                    map[string]int
	KeepAlive, ExpectContinue bool
}

// SyncHTTPParseError preserves Python's HTML error fields. Status zero means
// no response. Version HTTP/0.9 suppresses response headers in the source.
type SyncHTTPParseError struct {
	Status                    int
	Message, Explain, Version string
}

func (e *SyncHTTPParseError) Error() string {
	return fmt.Sprintf("HTTP parse failure: %d %s", e.Status, e.Message)
}
func syncHTTPError(status int, message, explain, version string) *SyncHTTPParseError {
	if message == "" {
		message = strings.TrimPrefix(HTTPStatusLine(status), strconv.Itoa(status)+" ")
	}
	if explain == "" {
		switch status {
		case 400:
			explain = "Bad request syntax or unsupported method"
		case 414:
			explain = "URI is too long"
		case 431:
			explain = "The server is unwilling to process the request because its header fields are too large"
		case 501:
			explain = "Server does not support this operation"
		case 505:
			explain = "Cannot fulfill request"
		}
	}
	return &SyncHTTPParseError{Status: status, Message: message, Explain: explain, Version: version}
}

// readPythonLine implements binary readline(size), including its early return
// exactly at size without waiting for a delimiter (unlike asyncio.readline).
func readPythonLine(reader *bufio.Reader, size int) (string, error) {
	var out strings.Builder
	for out.Len() < size {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF && out.Len() > 0 {
				return out.String(), nil
			}
			return "", err
		}
		out.WriteByte(b)
		if b == '\n' {
			break
		}
	}
	return out.String(), nil
}

// ParseSyncHTTPRequest mirrors Python 3.13.14's BaseHTTPRequestHandler. Legacy
// SSE has protocol_version HTTP/1.0; Streamable uses HTTP/1.1. Deadlines and
// cancellation are supplied by the connection. No request body is read here.
func ParseSyncHTTPRequest(reader *bufio.Reader, legacy bool) (*SyncHTTPRequest, error) {
	raw, err := readPythonLine(reader, 65537)
	if err != nil {
		return nil, err
	}
	fail := func(code int, message, explain, version string) (*SyncHTTPRequest, error) {
		return nil, syncHTTPError(code, message, explain, version)
	}
	if len(raw) > 65536 {
		return fail(414, "", "", "")
	}
	line := strings.TrimRight(latin1(raw), "\r\n")
	words := strings.FieldsFunc(line, pythonSpace)
	if len(words) == 0 {
		return fail(0, "", "", "")
	}
	request := &SyncHTTPRequest{Version: "HTTP/0.9", Headers: [][2]string{}, First: map[string]string{}, Last: map[string]string{}, Counts: map[string]int{}}
	if len(words) >= 3 {
		version := words[len(words)-1]
		major, minor, valid := syncHTTPVersion(version)
		if !valid {
			return fail(400, "Bad request version ("+pythonRepr(version)+")", "", request.Version)
		}
		request.KeepAlive = !legacy && (major > 1 || major == 1 && minor >= 1)
		if major >= 2 {
			return fail(505, "Invalid HTTP version ("+strings.TrimPrefix(version, "HTTP/")+")", "", request.Version)
		}
		request.Version = version
	}
	if len(words) < 2 || len(words) > 3 {
		return fail(400, "Bad request syntax ("+pythonRepr(line)+")", "", request.Version)
	}
	request.Method, request.Target = words[0], words[1]
	if len(words) == 2 {
		if request.Method != "GET" {
			return fail(400, "Bad HTTP/0.9 request type ("+pythonRepr(request.Method)+")", "", request.Version)
		}
	}
	if strings.HasPrefix(request.Target, "//") {
		request.Target = "/" + strings.TrimLeft(request.Target, "/")
	}
	if len(words) == 2 {
		return request, nil
	}
	var lines strings.Builder
	for count := 1; ; count++ {
		raw, err = readPythonLine(reader, 65537)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if len(raw) > 65536 {
			return fail(431, "Line too long", "got more than 65536 bytes when reading header line", request.Version)
		}
		if count > 100 {
			return fail(431, "Too many headers", "got more than 100 headers", request.Version)
		}
		lines.WriteString(latin1(raw))
		if raw == "\r\n" || raw == "\n" || err == io.EOF {
			break
		}
	}
	request.Headers = parseSyncHeaders(lines.String())
	for _, pair := range request.Headers {
		name := strings.ToLower(pair[0])
		request.Counts[name]++
		if request.Counts[name] == 1 {
			request.First[name] = pair[1]
		}
		request.Last[name] = pair[1]
	}
	connection := strings.ToLower(request.First["connection"])
	if connection == "close" {
		request.KeepAlive = false
	} else if connection == "keep-alive" && !legacy {
		request.KeepAlive = true
	}
	request.ExpectContinue = !legacy && request.Version >= "HTTP/1.1" && strings.EqualFold(request.First["expect"], "100-continue")
	return request, nil
}
func syncHTTPVersion(version string) (uint64, uint64, bool) {
	if !strings.HasPrefix(version, "HTTP/") {
		return 0, 0, false
	}
	parts := strings.Split(version[5:], ".")
	if len(parts) != 2 {
		return 0, 0, false
	}
	numbers := [2]uint64{}
	for i, part := range parts {
		if len(part) == 0 || len(part) > 10 {
			return 0, 0, false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return 0, 0, false
			}
		}
		numbers[i], _ = strconv.ParseUint(part, 10, 64)
	}
	return numbers[0], numbers[1], true
}

// Email's compat32 parser stops collecting fields at the first non-header
// line, ignores empty names/initial continuations and preserves folded values.
func parseSyncHeaders(text string) [][2]string {
	lines := splitEmailLines(text)
	collected := []string{}
	for _, line := range lines {
		if line == "\n" || line == "\r" || line == "\r\n" {
			break
		}
		if line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(line, "From ") {
			colon := strings.IndexByte(line, ':')
			if colon < 0 {
				break
			}
			valid := true
			for _, c := range line[:colon] {
				if c < 33 || c > 126 {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
		}
		collected = append(collected, line)
	}
	pairs := [][2]string{}
	active := false
	for i, line := range collected {
		if line[0] == ' ' || line[0] == '\t' {
			if active {
				pairs[len(pairs)-1][1] += line
			}
			continue
		}
		active = false
		if strings.HasPrefix(line, "From ") {
			if i == len(collected)-1 {
				break
			}
			continue
		}
		key, value, _ := strings.Cut(line, ":")
		if key == "" {
			continue
		}
		pairs = append(pairs, [2]string{key, strings.TrimLeft(value, " \t\r\n")})
		active = true
	}
	for i := range pairs {
		pairs[i][1] = strings.TrimRight(pairs[i][1], "\r\n")
	}
	return pairs
}
func splitEmailLines(text string) []string {
	lines := []string{}
	for len(text) > 0 {
		end := strings.IndexAny(text, "\r\n")
		if end < 0 {
			lines = append(lines, text)
			break
		}
		end++
		if text[end-1] == '\r' && end < len(text) && text[end] == '\n' {
			end++
		}
		lines = append(lines, text[:end])
		text = text[end:]
	}
	return lines
}
