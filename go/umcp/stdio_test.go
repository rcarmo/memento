package umcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestStdioFramingAndNotifications(t *testing.T) {
	input := " \t\r\n\x1c\n" + `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" + `{"jsonrpc":"2.0","id":1,"method":"echo","params":{"_meta":{"progressToken":"p"}}}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"unknown"}`
	var out bytes.Buffer
	handler := func(ctx context.Context, p map[string]any) (any, *RPCError, error) {
		if Context(ctx).Transport != "stdio" {
			t.Error("wrong transport")
		}
		message := "work"
		if err := NotifyProgress(ctx, 1, 2, &message); err != nil {
			return nil, nil, err
		}
		return "ok", nil, nil
	}
	if err := ServeStdio(context.Background(), strings.NewReader(input), &out, map[string]Handler{"echo": handler}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatal(out.String())
	}
	expected := []string{`{"jsonrpc":"2.0","method":"notifications/progress","params":{"progressToken":"p","progress":1,"total":2,"message":"work"}}`, `{"jsonrpc":"2.0","id":1,"result":"ok"}`, `{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"Method not found: unknown"}}`}
	for i, s := range lines {
		a, _ := decode([]byte(s))
		b, _ := decode([]byte(expected[i]))
		if !reflect.DeepEqual(a, b) {
			t.Error(s, expected[i])
		}
	}
}

type failureReader struct{}

func (failureReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }

type failureWriter struct{}

func (failureWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestStdioErrorPaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ServeStdio(ctx, strings.NewReader(""), io.Discard, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := ServeStdio(context.Background(), failureReader{}, io.Discard, nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := ServeStdio(context.Background(), strings.NewReader("{}\n"), failureWriter{}, nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	boom := errors.New("handler failed")
	handlers := map[string]Handler{"fail": func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, boom }}
	if err := ServeStdio(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"fail"}`), io.Discard, handlers); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}

func TestUTF8Replacement(t *testing.T) {
	for _, c := range []struct {
		Bytes []byte
		Want  string
	}{
		{[]byte("valid 😀"), "valid 😀"}, {[]byte{0xff, 0xff}, "��"}, {[]byte{0xe2, 0x82}, "�"}, {[]byte{0xf0, 0x9f, 0x92}, "�"}, {[]byte{0xc2}, "�"}, {[]byte{0xc2, 'x'}, "�x"}, {[]byte{0xed, 0xa0, 0x80}, "���"}, {[]byte{0xe0, 0x80, 0x80}, "���"}, {[]byte{0xf0, 0x80, 0x80, 0x80}, "����"}, {[]byte{0xf4, 0x90, 0x80, 0x80}, "����"}, {[]byte{0xef, 0xbf, 0xbd}, "�"},
	} {
		if got := replaceInvalidUTF8(string(c.Bytes)); got != c.Want {
			t.Errorf("%x -> %q != %q", c.Bytes, got, c.Want)
		}
	}
	var out bytes.Buffer
	input := append([]byte(`{"jsonrpc":"2.0","id":"`), 0xe2, 0x82)
	input = append(input, []byte(`","method":"unknown"}`)...)
	if err := ServeStdio(context.Background(), bytes.NewReader(input), &out, nil); err != nil {
		t.Fatal(err)
	}
	var result struct{ ID string }
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != "�" {
		t.Fatal(result.ID)
	}
}

func FuzzStdio(f *testing.F) {
	for _, s := range []string{"\n", "{}", "null\n", "\xff", `{"jsonrpc":"2.0","method":"unknown"}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		_ = ServeStdio(context.Background(), strings.NewReader(raw), io.Discard, nil)
	})
}

func TestPythonStdioParity(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-stdio.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input    []byte `json:"input_bytes"`
		Expected json.RawMessage
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	handler := func(ctx context.Context, params map[string]any) (any, *RPCError, error) {
		return map[string]any{"transport": Context(ctx).Transport, "params": params}, nil, nil
	}
	for _, c := range cases {
		var out bytes.Buffer
		if err = ServeStdio(context.Background(), bytes.NewReader(c.Input), &out, map[string]Handler{"initialize": handler}); err != nil {
			t.Fatal(err)
		}
		var replies = []any{}
		decoder := json.NewDecoder(&out)
		decoder.UseNumber()
		for {
			var reply any
			e := decoder.Decode(&reply)
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			replies = append(replies, reply)
		}
		want, _ := decode(c.Expected)
		if !reflect.DeepEqual(replies, want) {
			t.Errorf("%q: got %#v want %#v", c.Input, replies, want)
		}
	}
	// The reference has no Scanner's implicit 64 KiB token limit.
	var output bytes.Buffer
	text := strings.Repeat("x", 70000)
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"text":"` + text + `"}}`
	if err = ServeStdio(context.Background(), strings.NewReader(input), &output, map[string]Handler{"initialize": handler}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), text) {
		t.Fatal("large line truncated")
	}
}
