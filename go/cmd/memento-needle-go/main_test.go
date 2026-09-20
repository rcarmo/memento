package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/needleworker"
)

func TestRun(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run(nil, &bytes.Buffer{}, &out, &stderr); code != 2 {
		t.Fatal(code)
	}
	if code := run([]string{"missing", "tokenizer"}, &bytes.Buffer{}, &out, &stderr); code != 1 {
		t.Fatal(code)
	}
	boom := errors.New("boom")
	mapped := &needle.MappedRouter{Router: &needle.Router{}}
	if code := runWith([]string{"model", "tokenizer"}, &bytes.Buffer{}, &out, &stderr, func(string) (*needle.MappedRouter, error) { return mapped, nil }, func(string) (*needle.Tokenizer, error) { return nil, boom }, needleworker.Serve); code != 1 {
		t.Fatal(code)
	}
	if code := runWith([]string{"model", "tokenizer"}, &bytes.Buffer{}, &out, &stderr, func(string) (*needle.MappedRouter, error) { return mapped, nil }, func(string) (*needle.Tokenizer, error) { return &needle.Tokenizer{}, nil }, func(io.Reader, io.Writer, needleworker.GenerateFunc, uint32) error { return boom }); code != 1 {
		t.Fatal(code)
	}
	if code := runWith([]string{"model", "tokenizer"}, &bytes.Buffer{}, &out, &stderr, func(string) (*needle.MappedRouter, error) { return mapped, nil }, func(string) (*needle.Tokenizer, error) { return &needle.Tokenizer{}, nil }, func(io.Reader, io.Writer, needleworker.GenerateFunc, uint32) error { return nil }); code != 0 {
		t.Fatal(code)
	}
	if _, err := mappedGenerate(mapped, &needle.Tokenizer{})("q", "[]", needle.GenerationOptions{MaxEncoded: -1}); err == nil {
		t.Fatal("generate")
	}
}
func TestMain(t *testing.T) {
	oldArgs, oldExit := os.Args, exit
	t.Cleanup(func() { os.Args, exit = oldArgs, oldExit })
	os.Args = []string{"memento-needle-go"}
	called := false
	exit = func(code int) { called = code == 2 }
	main()
	if !called {
		t.Fatal("exit")
	}
}
