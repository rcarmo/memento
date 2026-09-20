package umcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// ServeStdio serves sequential newline-delimited requests, as both pinned
// Python bases do. Blank lines are skipped, incomplete final lines are handled,
// and notifications write no response. The caller owns streams; cancellation
// does not close a caller-owned blocking reader (close it to interrupt a read).
// Unlike the HTTP transport, the reference stdio transport has no line limit.
func ServeStdio(ctx context.Context, input io.Reader, output io.Writer, handlers map[string]Handler) error {
	var writeMu sync.Mutex
	write := func(value any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		encoder := json.NewEncoder(output)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(value)
	}
	dispatcher := Dispatcher{Handlers: handlers, Notify: func(method string, params map[string]any) error {
		return write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	}}
	return serveStdioLoop(ctx, input, write, dispatcher.Process)
}

func (s *Server) serveStdio(ctx context.Context, input io.Reader, output io.Writer) error {
	return serveStdioLoop(ctx, input, func(value any) error { return encodeLine(output, value) }, s.Process)
}
func serveStdioLoop(ctx context.Context, input io.Reader, write func(any) error, process func(context.Context, []byte, RequestContext) (*Response, error)) error {
	reader := bufio.NewReader(input)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			text := strings.TrimFunc(replaceInvalidUTF8(line), func(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) })
			if text != "" {
				response, err := process(ctx, []byte(text), RequestContext{Transport: "stdio"})
				if err != nil {
					return err
				}
				if response != nil {
					if err = write(response); err != nil {
						return err
					}
				}
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// Python UTF-8 errors=replace consumes the valid prefix of an incomplete
// multi-byte sequence as one error, unlike replacing each invalid byte.
func replaceInvalidUTF8(text string) string {
	var out strings.Builder
	for len(text) > 0 {
		r, n := utf8.DecodeRuneInString(text)
		if r != utf8.RuneError || n > 1 {
			out.WriteString(text[:n])
			text = text[n:]
			continue
		}
		consumed := 1
		first := text[0]
		length := 0
		switch {
		case first >= 0xc2 && first <= 0xdf:
			length = 2
		case first >= 0xe0 && first <= 0xef:
			length = 3
		case first >= 0xf0 && first <= 0xf4:
			length = 4
		}
		for consumed < length && consumed < len(text) {
			b := text[consumed]
			if b < 0x80 || b > 0xbf {
				break
			}
			if consumed == 1 && ((first == 0xe0 && b < 0xa0) || (first == 0xed && b > 0x9f) || (first == 0xf0 && b < 0x90) || (first == 0xf4 && b > 0x8f)) {
				break
			}
			consumed++
		}
		out.WriteRune(utf8.RuneError)
		text = text[consumed:]
	}
	return out.String()
}
