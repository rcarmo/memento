package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestRunInvalid(t *testing.T) {
	for _, args := range [][]string{nil, {"-bad"}, {"-model", "missing"}, {"-model", "missing", "-role", "invalid"}, {"-model", "missing", "-dimensions", "17"}, {"-model", "missing", "extra"}} {
		var out, err bytes.Buffer
		if run(context.Background(), args, &out, &err) == 0 || out.Len() != 0 || err.Len() == 0 {
			t.Fatal(args, out.String(), err.String())
		}
	}
}
func TestRunReal(t *testing.T) {
	path := os.Getenv("EMBEDDINGGEMMA_MODEL")
	if path == "" {
		t.Skip("no model")
	}
	var out, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-model", path, "-text", "hello", "-dimensions", "128", "-role", "query"}, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var result struct {
		Tokens    []int
		Embedding []float32
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || len(result.Embedding) != 128 || len(result.Tokens) < 3 {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := run(ctx, []string{"-model", path}, io.Discard, &stderr); code != 1 {
		t.Fatal(code)
	}
}
