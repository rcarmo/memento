package umcp

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MediaAccepts mirrors the reference Accept parser, including last-q-wins and
// wildcard matching. It does not impose HTTP negotiation policy on callers.
func MediaAccepts(accept string, mediaTypes ...string) bool {
	if accept == "" {
		return true
	}
	normalized := make(map[string]bool, len(mediaTypes))
	for _, media := range mediaTypes {
		normalized[strings.ToLower(media)] = true
	}
	for _, item := range strings.Split(accept, ",") {
		parts := strings.Split(item, ";")
		media := strings.ToLower(strings.TrimSpace(parts[0]))
		quality := 1.0
		for _, part := range parts[1:] {
			part = strings.ToLower(strings.TrimSpace(part))
			if strings.HasPrefix(part, "q=") {
				quality = pythonQuality(part[2:])
			}
		}
		if quality <= 0 {
			continue
		}
		if media == "*/*" {
			return true
		}
		major, minor, _ := strings.Cut(media, "/")
		if minor == "*" {
			for candidate := range normalized {
				if strings.HasPrefix(candidate, major+"/") {
					return true
				}
			}
		}
		if normalized[media] {
			return true
		}
	}
	return false
}

var qualityNumber = regexp.MustCompile(`(?i)^[+-]?(?:inf(?:inity)?|nan|(?:(?:[0-9](?:_?[0-9])*)(?:\.(?:[0-9](?:_?[0-9])*)?)?|\.[0-9](?:_?[0-9])*)(?:e[+-]?[0-9](?:_?[0-9])*)?)$`)

// Python's float parser permits underscores between decimal digits; strconv
// permits some nondecimal forms instead. Restrict the grammar before parsing.
func normalizeDecimalDigits(value string) string {
	return strings.Map(func(r rune) rune {
		// Unicode decimal digits accepted by Python float, including Arabic/fullwidth.
		for _, block := range unicode.Digit.R16 {
			if uint32(r) >= uint32(block.Lo) && uint32(r) <= uint32(block.Hi) && (uint32(r)-uint32(block.Lo))%uint32(block.Stride) == 0 {
				return '0' + rune((uint32(r)-uint32(block.Lo))/uint32(block.Stride)%10)
			}
		}
		for _, block := range unicode.Digit.R32 {
			if uint32(r) >= block.Lo && uint32(r) <= block.Hi && (uint32(r)-block.Lo)%block.Stride == 0 {
				return '0' + rune((uint32(r)-block.Lo)/block.Stride%10)
			}
		}
		return r
	}, value)
}

func pythonQuality(value string) float64 {
	value = normalizeDecimalDigits(strings.TrimSpace(value))
	if !qualityNumber.MatchString(value) {
		return 0
	}
	value = strings.ReplaceAll(value, "_", "")
	if value == "+nan" || value == "-nan" {
		value = "nan"
	}
	// The validated decimal/special-value grammar has no syntax failures.
	// Range overflow intentionally becomes infinity, just as Python float does.
	result, _ := strconv.ParseFloat(value, 64)
	return result
}

// MediaAcceptsJSON tests application/json negotiation.
func MediaAcceptsJSON(accept string) bool { return MediaAccepts(accept, "application/json") }

// MediaAcceptsEventStream tests text/event-stream negotiation.
func MediaAcceptsEventStream(accept string) bool { return MediaAccepts(accept, "text/event-stream") }

// ContentTypeIsJSON ignores parameters and case, as the reference does.
func ContentTypeIsJSON(contentType string) bool {
	media, _, _ := strings.Cut(contentType, ";")
	return strings.EqualFold(strings.TrimSpace(media), "application/json")
}

// SingletonHTTPHeaders returns a copy of the source singleton header names.
func SingletonHTTPHeaders() []string {
	return []string{"host", "authorization", "origin", "accept", "content-type", "mcp-protocol-version", "mcp-session-id", "content-length", "transfer-encoding"}
}

// HasAmbiguousSingletonValues expects lowercase keys from the transport parser.
func HasAmbiguousSingletonValues(headers map[string]string) bool {
	for _, name := range []string{"host", "authorization", "origin", "mcp-protocol-version", "mcp-session-id", "content-length"} {
		if strings.Contains(headers[name], ",") {
			return true
		}
	}
	return false
}

// HasSingletonHeaderViolations checks multiplicity, including mandatory HTTP/1.1 Host.
func HasSingletonHeaderViolations(counts map[string]int, httpVersion string) bool {
	for _, name := range SingletonHTTPHeaders() {
		count := counts[name]
		if name == "host" {
			if httpVersion == "HTTP/1.1" {
				if count != 1 {
					return true
				}
			} else if count > 1 {
				return true
			}
		} else if count > 1 {
			return true
		}
	}
	return false
}

// HTTPStatusLine preserves the Python HTTPStatus reason phrases, including
// names that differ from net/http, and leaves unknown codes without a phrase.
func HTTPStatusLine(status int) string {
	phrase := http.StatusText(status)
	switch status {
	case 103:
		phrase = "Early Hints"
	case 413:
		phrase = "Content Too Large"
	case 414:
		phrase = "URI Too Long"
	case 416:
		phrase = "Range Not Satisfiable"
	case 418:
		phrase = "I'm a Teapot"
	case 422:
		phrase = "Unprocessable Content"
	case 425:
		phrase = "Too Early"
	}
	return strings.TrimRight(fmt.Sprintf("%d %s", status, phrase), " ")
}

// HTTPResponse is the Go equivalent of the immutable typed MCPHTTPResponse.
// Body bytes and header strings are never interpreted as trusted routing data.
type HTTPResponse struct {
	Status      int
	Body        []byte
	ContentType *string
	Headers     [][2]string
}

// ValidateHTTPResponse returns false for invalid bounds or header injection.
// Header limits count Unicode characters to mirror the Python reference; the
// transport must independently enforce the on-wire byte ceiling.
func ValidateHTTPResponse(response *HTTPResponse, maxBytes int) bool {
	if response == nil || response.Status < 100 || response.Status > 599 || maxBytes < 0 {
		return false
	}
	remaining := maxBytes - len(response.Body)
	if remaining < 0 {
		return false
	}
	if response.ContentType != nil {
		if strings.ContainsAny(*response.ContentType, "\r\n") {
			return false
		}
		remaining -= len("Content-Type") + utf8.RuneCountInString(*response.ContentType)
	}
	for _, header := range response.Headers {
		if strings.ContainsAny(header[0], "\r\n") || strings.ContainsAny(header[1], "\r\n") {
			return false
		}
		remaining -= utf8.RuneCountInString(header[0]) + utf8.RuneCountInString(header[1])
	}
	return remaining >= 0
}
