package umcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPythonSyncHTTPParser(t *testing.T) {
	raw, err := os.ReadFile("testdata/parity/umcp-sync-http.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Python string
		Cases  []struct {
			Legacy bool
			Input  struct {
				Name                   string
				Prefix, Repeat, Suffix []byte
				Count                  int
			}
			Expected struct {
				Status                                    int
				Method, Target, Version, Message, Explain string
				Headers, First                            map[string]string
				Counts                                    map[string]int
				Pairs                                     [][2]string
				KeepAlive                                 bool `json:"keep_alive"`
				ExpectContinue                            bool `json:"expect_continue"`
				Remaining                                 []byte
			}
		}
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	if fixtures.Python != "3.13.14" {
		t.Fatal("review stdlib reference update", fixtures.Python)
	}
	for _, c := range fixtures.Cases {
		prefix := "streamable/"
		if c.Legacy {
			prefix = "sse/"
		}
		t.Run(prefix+c.Input.Name, func(t *testing.T) {
			input := append(append(append([]byte{}, c.Input.Prefix...), bytes.Repeat(c.Input.Repeat, c.Input.Count)...), c.Input.Suffix...)
			reader := bufio.NewReader(bytes.NewReader(input))
			request, err := ParseSyncHTTPRequest(reader, c.Legacy)
			if c.Expected.Status != 200 {
				if c.Expected.Status == 0 && errors.Is(err, io.EOF) {
					return
				}
				var failure *SyncHTTPParseError
				if !errors.As(err, &failure) || failure.Status != c.Expected.Status {
					t.Fatalf("error %v expected %+v", err, c.Expected)
				}
				if failure.Status != 0 && (failure.Message != c.Expected.Message || failure.Explain != c.Expected.Explain || failure.Version != c.Expected.Version) {
					t.Fatalf("%+v != %+v", failure, c.Expected)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if request.Method != c.Expected.Method || request.Target != c.Expected.Target || request.Version != c.Expected.Version || request.KeepAlive != c.Expected.KeepAlive || request.ExpectContinue != c.Expected.ExpectContinue || !reflect.DeepEqual(request.First, c.Expected.First) || !reflect.DeepEqual(request.Last, c.Expected.Headers) || !reflect.DeepEqual(request.Counts, c.Expected.Counts) || !reflect.DeepEqual(request.Headers, c.Expected.Pairs) {
				t.Fatalf("%+v != %+v", request, c.Expected)
			}
			rest, err := io.ReadAll(reader)
			if err != nil || !bytes.Equal(rest, c.Expected.Remaining) {
				t.Fatal("unconsumed bytes", len(rest), len(c.Expected.Remaining), err)
			}
		})
	}
}

func TestSyncHTTPParserErrors(t *testing.T) {
	if _, err := ParseSyncHTTPRequest(bufio.NewReader(failureReader{}), false); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	input := io.MultiReader(strings.NewReader("GET / HTTP/1.1\n"), failureReader{})
	if _, err := ParseSyncHTTPRequest(bufio.NewReader(input), false); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if syncHTTPError(501, "", "", "").Error() != "HTTP parse failure: 501 Not Implemented" {
		t.Fatal("error")
	}
	if got := splitEmailLines("one\rtwo\r\nthree\nfour"); !reflect.DeepEqual(got, []string{"one\r", "two\r\n", "three\n", "four"}) {
		t.Fatal(got)
	}
}

func FuzzSyncHTTPParser(f *testing.F) {
	for _, input := range []string{"", "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n", "GET /\n", "\xff"} {
		f.Add(input, false)
	}
	f.Fuzz(func(t *testing.T, input string, legacy bool) {
		_, _ = ParseSyncHTTPRequest(bufio.NewReader(strings.NewReader(input)), legacy)
	})
}

func TestSyncHTTPDefaultErrorExplanation(t *testing.T) {
	if got := syncHTTPError(431, "", "", ""); got.Explain != "The server is unwilling to process the request because its header fields are too large" {
		t.Fatal(got)
	}
}
