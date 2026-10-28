package main

import (
	"bytes"
	"os"
	"testing"
)

func TestRunErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"missing"}, {"a", "b"}} {
		var out, err bytes.Buffer
		if run(args, &out, &err) == 0 || err.Len() == 0 {
			t.Fatal(args)
		}
	}
}
func TestRunReal(t *testing.T) {
	p := os.Getenv("EMBEDDINGGEMMA_MODEL")
	if p == "" {
		t.Skip("real checkpoint opt-in")
	}
	var out, err bytes.Buffer
	if code := run([]string{p}, &out, &err); code != 0 || !bytes.Contains(out.Bytes(), []byte("gemma-embedding2")) {
		t.Fatal(code, err.String())
	}
}
func FuzzArguments(f *testing.F) {
	f.Add("missing")
	f.Fuzz(func(t *testing.T, arg string) { var out, err bytes.Buffer; _ = run([]string{arg, "extra"}, &out, &err) })
}
