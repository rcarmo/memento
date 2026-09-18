package pyjson

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestPythonJSONEncoding(t *testing.T) {
	cases := []struct {
		Value any
		Want  string
	}{{map[string]any{"z": "日本 😀\x7f", "a": 1.0}, `{"a": 1.0, "z": "\u65e5\u672c \ud83d\ude00\u007f"}`}, {[]any{nil, true, false, int(1), int64(2), json.Number("123456789012345678901234"), json.Number("1.0"), 1e-5, 1e16}, `[null, true, false, 1, 2, 123456789012345678901234, 1.0, 1e-05, 1e+16]`}, {[]string{"a", "b"}, `["a", "b"]`}, {"\b\f\n\r\t\x00\"\\", `"\b\f\n\r\t\u0000\"\\"`}, {[]any{math.NaN(), math.Inf(1), math.Inf(-1)}, `[NaN, Infinity, -Infinity]`}}
	for _, c := range cases {
		got, err := Dumps(c.Value)
		if err != nil || got != c.Want {
			t.Fatal(got, c.Want, err)
		}
	}
	for _, v := range []any{make(chan int), json.Number("bad"), json.Number("1e999"), "\xff", map[string]any{"\xff": 1}, []any{make(chan int)}, map[string]any{"x": make(chan int)}} {
		if _, err := Dumps(v); err == nil {
			t.Fatal(v)
		}
	}
	var nested any = nil
	for range 102 {
		nested = []any{nested}
	}
	if _, err := Dumps(nested); err == nil {
		t.Fatal("depth")
	}
	for _, raw := range []string{"{", "{} {}", "{} garbage"} {
		if _, err := Parse(raw); err == nil {
			t.Fatal(raw)
		}
	}
	value, err := Parse(`{"b":2,"a":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Dumps(value); err != nil || got != `{"a": 1, "b": 2}` {
		t.Fatal(got, err)
	}
}
func FuzzPythonJSON(f *testing.F) {
	for _, s := range []string{"{}", "[]", "null", `{"a":"é"}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 65536 {
			return
		}
		v, err := Parse(s)
		if err != nil {
			return
		}
		out, err := Dumps(v)
		if err == nil && !strings.Contains(out, "Infinity") && !strings.Contains(out, "NaN") {
			if _, err = Parse(out); err != nil {
				t.Fatal(err)
			}
		}
	})
}
