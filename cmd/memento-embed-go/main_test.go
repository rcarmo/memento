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

func TestParseRunConfig(t *testing.T) {
	config, err := parseRunConfig([]string{"model.gtemodel"}, nil)
	if err != nil || config.ModelPath != "model.gtemodel" || config.Nice != 15 {
		t.Fatal(config, err)
	}
	config, err = parseRunConfig([]string{"--nice", "19", "model.gtemodel"}, nil)
	if err != nil || config.ModelPath != "model.gtemodel" || config.Nice != 19 {
		t.Fatal(config, err)
	}
	config, err = parseRunConfig([]string{"--nice=12", "model.gtemodel"}, func(string) (string, bool) { return "18", true })
	if err != nil || config.Nice != 12 {
		t.Fatal(config, err)
	}
	config, err = parseRunConfig([]string{"model.gtemodel"}, func(string) (string, bool) { return " 7 ", true })
	if err != nil || config.Nice != 7 {
		t.Fatal(config, err)
	}
	for _, args := range [][]string{{}, {"a", "b"}, {"--nice"}, {"--nice=20", "model.gtemodel"}, {"--nice", "bad", "model.gtemodel"}} {
		if _, err := parseRunConfig(args, nil); err == nil {
			t.Fatal(args)
		}
	}
	if _, err = parseRunConfig([]string{"model.gtemodel"}, func(string) (string, bool) { return "bad", true }); err == nil {
		t.Fatal("invalid env")
	}
}

func TestRun(t *testing.T) {
	var output, stderr bytes.Buffer
	load := func(path string) (*gte.Model, error) { return nil, nil }
	applyCalls := 0
	apply := func(priority int) error {
		applyCalls++
		if priority != 15 {
			t.Fatal(priority)
		}
		return nil
	}
	if code := runWith(nil, strings.NewReader(""), &output, &stderr, nil, apply, load); code != 2 {
		t.Fatal(code)
	}
	if applyCalls != 0 || !strings.Contains(stderr.String(), usage) {
		t.Fatal(applyCalls, stderr.String())
	}
	stderr.Reset()
	if code := runWith([]string{"a", "b"}, strings.NewReader(""), &output, &stderr, nil, apply, load); code != 2 {
		t.Fatal(code)
	}
	boom := errors.New("load failed")
	stderr.Reset()
	if code := runWith([]string{"model"}, strings.NewReader(""), &output, &stderr, nil, apply, func(string) (*gte.Model, error) { return nil, boom }); code != 1 || !strings.Contains(stderr.String(), "load failed") {
		t.Fatal(code, stderr.String())
	}
	stderr.Reset()
	if code := runWith([]string{"model"}, strings.NewReader(""), &output, &stderr, nil, apply, load); code != 0 {
		t.Fatal(code)
	}
	stderr.Reset()
	if code := runWith([]string{"model"}, badReader{}, &output, &stderr, nil, apply, load); code != 1 {
		t.Fatal(code)
	}
}

func TestRunNiceFailures(t *testing.T) {
	var output, stderr bytes.Buffer
	applyErr := errors.New("nice")
	if code := runWith([]string{"model"}, strings.NewReader(""), &output, &stderr, nil, func(int) error { return applyErr }, func(string) (*gte.Model, error) { return nil, nil }); code != 1 {
		t.Fatal(code)
	}
	if !strings.Contains(stderr.String(), "failed to set embedding process nice") || !strings.Contains(stderr.String(), "nice") {
		t.Fatal(stderr.String())
	}
	stderr.Reset()
	if code := runWith([]string{"model"}, strings.NewReader(""), &output, &stderr, func(string) (string, bool) { return "bad", true }, func(int) error { t.Fatal("apply"); return nil }, func(string) (*gte.Model, error) { return nil, nil }); code != 2 {
		t.Fatal(code)
	}
	if !strings.Contains(stderr.String(), usage) || !strings.Contains(stderr.String(), "invalid") {
		t.Fatal(stderr.String())
	}
	stderr.Reset()
	called := 0
	if code := runWith([]string{"--nice", "19", "model"}, strings.NewReader(""), &output, &stderr, func(string) (string, bool) { return "7", true }, func(priority int) error { called = priority; return nil }, func(string) (*gte.Model, error) { return nil, nil }); code != 0 || called != 19 {
		t.Fatal(code, called)
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
