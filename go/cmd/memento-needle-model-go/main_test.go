package main

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/rcarmo/memento/go/needle"
)

func TestRun(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run(nil, &out, &stderr); code != 2 {
		t.Fatal(code)
	}
	if code := run([]string{"missing", t.TempDir() + "/out"}, &out, &stderr); code != 1 {
		t.Fatal(code)
	}
	boom := errors.New("boom")
	ok := func(string, string) error { return nil }
	if code := runWith([]string{"a", "b"}, &out, &stderr, ok, func(string) (needle.FP32Info, error) { return needle.FP32Info{}, boom }, func(string) error { return nil }); code != 1 {
		t.Fatal(code)
	}
	if code := runWith([]string{"a", "b"}, &out, &stderr, ok, func(string) (needle.FP32Info, error) { return needle.FP32Info{TensorCount: 1, FileSize: 2}, nil }, func(string) error { return boom }); code != 1 {
		t.Fatal(code)
	}
	if code := runWith([]string{"a", "b"}, &out, &stderr, ok, func(string) (needle.FP32Info, error) { return needle.FP32Info{TensorCount: 1, FileSize: 2}, nil }, func(string) error { return nil }); code != 0 {
		t.Fatal(code)
	}
}
func TestMain(t *testing.T) {
	oldArgs, oldExit := os.Args, exit
	t.Cleanup(func() { os.Args, exit = oldArgs, oldExit })
	os.Args = []string{"memento-needle-model-go"}
	called := false
	exit = func(code int) { called = code == 2 }
	main()
	if !called {
		t.Fatal("exit")
	}
}
