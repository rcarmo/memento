package umcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPythonAsyncHTTPParser(t *testing.T) {
	raw, err := os.ReadFile("testdata/parity/umcp-raw-http.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Mode  string
		Input struct {
			Name                   string
			Prefix, Repeat, Suffix []byte
			Count                  int
		}
		Expected struct {
			Status                                int
			Method, Path, Target, Version, Origin string
			Headers                               map[string]string
			Counts                                map[string]int
			Body                                  []byte
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Mode+"/"+c.Input.Name, func(t *testing.T) {
			mode := AsyncStreamableParser
			if c.Mode == "sse" {
				mode = AsyncSSEParser
			}
			options := DefaultAsyncHTTPParserOptions(mode)
			options.MaxRequestBytes = 1 << 20
			input := append(append(append([]byte{}, c.Input.Prefix...), bytes.Repeat(c.Input.Repeat, c.Input.Count)...), c.Input.Suffix...)
			request, err := ParseAsyncHTTPRequest(bufio.NewReader(bytes.NewReader(input)), options)
			if c.Expected.Status != 200 {
				var failure *HTTPParseError
				if !errors.As(err, &failure) || failure.Status != c.Expected.Status || failure.Origin != c.Expected.Origin {
					t.Fatalf("%v != %+v", err, c.Expected)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			target := c.Expected.Target
			if mode == AsyncStreamableParser {
				target = c.Expected.Path
			}
			if request.Method != c.Expected.Method || request.Target != target || !reflect.DeepEqual(request.Headers, c.Expected.Headers) || !bytes.Equal(request.Body, c.Expected.Body) {
				t.Fatalf("%+v != %+v", request, c.Expected)
			}
			if mode == AsyncSSEParser && (request.Version != c.Expected.Version || !reflect.DeepEqual(request.HeaderCounts, c.Expected.Counts)) {
				t.Fatalf("%+v != %+v", request, c.Expected)
			}
			if mode == AsyncStreamableParser && request.Origin != c.Expected.Origin {
				t.Fatal(request.Origin, c.Expected.Origin)
			}
		})
	}
}

func TestHTTPParserPipelining(t *testing.T) {
	for _, mode := range []AsyncHTTPParserMode{AsyncStreamableParser, AsyncSSEParser} {
		options := DefaultAsyncHTTPParserOptions(mode)
		reader := bufio.NewReader(strings.NewReader("POST /mcp HTTP/1.1\r\nHost: localhost\r\nContent-Length: 2\r\n\r\n{}GET /next HTTP/1.0\r\nConnection: keep-alive\r\n\r\n"))
		for i := range 2 {
			request, err := ParseAsyncHTTPRequest(reader, options)
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 && string(request.Body) != "{}" || i == 1 && request.Target != "/next" {
				t.Fatal(request)
			}
			if request.KeepAlive != (mode == AsyncStreamableParser) {
				t.Fatal("keepalive", request)
			}
		}
		_, err := ParseAsyncHTTPRequest(reader, options)
		var failure *HTTPParseError
		if !errors.As(err, &failure) {
			t.Fatal(err)
		}
	}
}

func TestHTTPParserErrorsAndTimeouts(t *testing.T) {
	for _, o := range []AsyncHTTPParserOptions{{Mode: 2}, {MaxRequestBytes: 1}, {ReadTimeout: 1}} {
		if _, err := ParseAsyncHTTPRequest(bufio.NewReader(strings.NewReader("")), o); err == nil {
			t.Fatal(o)
		}
	}
	if (&HTTPParseError{Status: 400}).Error() != "HTTP parse failure: 400 Bad Request" {
		t.Fatal("error string")
	}
	for _, mode := range []AsyncHTTPParserMode{AsyncStreamableParser, AsyncSSEParser} {
		for _, failCall := range []int{1, 2, 5} {
			n := 0
			options := DefaultAsyncHTTPParserOptions(mode)
			options.SetReadDeadline = func(time.Time) error {
				n++
				if n == failCall {
					return io.ErrClosedPipe
				}
				return nil
			}
			_, err := ParseAsyncHTTPRequest(bufio.NewReader(strings.NewReader("POST /mcp HTTP/1.1\nHost: localhost\nContent-Length: 2\n\n{}")), options)
			var failure *HTTPParseError
			if !errors.As(err, &failure) {
				t.Fatal(err)
			}
			want := 400
			if mode == AsyncStreamableParser && failCall == 1 {
				want = 0
			}
			if failure.Status != want {
				t.Fatal(failure)
			}
		}
		// Actual idle read interrupted by the supplied per-read deadline.
		a, b := net.Pipe()
		options := DefaultAsyncHTTPParserOptions(mode)
		options.ReadTimeout = time.Millisecond
		options.SetReadDeadline = a.SetReadDeadline
		_, err := ParseAsyncHTTPRequest(bufio.NewReader(a), options)
		a.Close()
		b.Close()
		if err == nil {
			t.Fatal("idle reader accepted")
		}
	}
	for _, c := range []struct {
		Value  string
		Size   int64
		Status int
	}{{"+1_2", 12, 0}, {"１２", 12, 0}, {"1\x1c", 0, 400}, {"0", 0, 0}, {"-1", 0, 400}, {"1001", 0, 413}, {"invalid", 0, 400}} {
		size, status := httpContentLength(c.Value, 1000)
		if size != c.Size || status != c.Status {
			t.Fatal(c, size, status)
		}
	}
	if latin1("\xc3\xa9") != "Ã©" {
		t.Fatal("latin1 must decode individual bytes")
	}
}

func FuzzAsyncHTTPParser(f *testing.F) {
	for _, input := range []string{"", "GET / HTTP/1.1\nHost: localhost\n\n", "POST / HTTP/1.1\nHost: localhost\nContent-Length: 2\n\n{}"} {
		f.Add(input, false)
	}
	f.Fuzz(func(t *testing.T, input string, legacy bool) {
		mode := AsyncStreamableParser
		if legacy {
			mode = AsyncSSEParser
		}
		options := DefaultAsyncHTTPParserOptions(mode)
		options.MaxRequestBytes = 4096
		request, err := ParseAsyncHTTPRequest(bufio.NewReader(strings.NewReader(input)), options)
		if err == nil && int64(len(request.Body)) > options.MaxRequestBytes {
			t.Fatal("body limit")
		}
	})
}
