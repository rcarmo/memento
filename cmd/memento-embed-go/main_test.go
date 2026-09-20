package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/gte"
)

func TestRun(t *testing.T) {
	var output, stderr bytes.Buffer
	load := func(path string) (*gte.Model, error) { return nil, nil }
	if code := run(nil, strings.NewReader(""), &output, &stderr, load); code != 2 {
		t.Fatal(code)
	}
	if code := run([]string{"a", "b"}, strings.NewReader(""), &output, &stderr, load); code != 2 {
		t.Fatal(code)
	}
	boom := errors.New("load failed")
	if code := run([]string{"model"}, strings.NewReader(""), &output, &stderr, func(string) (*gte.Model, error) { return nil, boom }); code != 1 || !strings.Contains(stderr.String(), "load failed") {
		t.Fatal(code, stderr.String())
	}
	if code := run([]string{"model"}, strings.NewReader(""), &output, &stderr, load); code != 0 {
		t.Fatal(code)
	}
	if code := run([]string{"model"}, badReader{}, &output, &stderr, load); code != 1 {
		t.Fatal(code)
	}
}

type badReader struct{}

func (badReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestMain(t *testing.T) {
	oldArgs, oldExit := os.Args, exit
	t.Cleanup(func() { os.Args = oldArgs; exit = oldExit })
	os.Args = []string{"memento-embed-go"}
	called := false
	exit = func(code int) {
		called = true
		if code != 2 {
			t.Fatal(code)
		}
	}
	main()
	if !called {
		t.Fatal("exit not called")
	}
}
