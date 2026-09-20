package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
)

const benchmarkConcept = "---\nid: a\ntype: concept\ntitle: Shared benchmark\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: benchmark\n---\nshared benchmark\n"

func benchmarkRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte(benchmarkConcept), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestBenchmarkRun(t *testing.T) {
	result, err := run(benchmarkRoot(t), 1, 2)
	if err != nil || len(result.RebuildNS) != 1 || len(result.SearchNS) != 2 || result.ResultCount != 1 {
		t.Fatal(result, err)
	}
}

type badWriter struct{}

func (badWriter) Write([]byte) (int, error) { return 0, errors.New("write") }
func TestBenchmarkInjectedFailures(t *testing.T) {
	boom := errors.New("boom")
	ops := defaultBenchmarkOps()
	ops.rebuild = func(*derived.Index, context.Context, string) error { return boom }
	if _, err := runWithOps("x", 1, 1, ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ops = defaultBenchmarkOps()
	ops.rebuild = func(*derived.Index, context.Context, string) error { return nil }
	ops.search = func(*derived.Index, context.Context, access.EffectivePolicy) (derived.SearchPage, error) {
		return derived.SearchPage{}, boom
	}
	if _, err := runWithOps("x", 1, 1, ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
func TestBenchmarkCLI(t *testing.T) {
	root := benchmarkRoot(t)
	var out bytes.Buffer
	if code := runCLI([]string{root, "1", "1"}, &out, io.Discard); code != 0 || !json.Valid(out.Bytes()) {
		t.Fatal(code, out.String())
	}
	for _, args := range [][]string{{}, {root, "x", "1"}, {root, "1", "x"}, {root, "0", "1"}, {root, "1", "0"}} {
		if code := runCLI(args, io.Discard, io.Discard); code == 0 {
			t.Fatal(args)
		}
	}
	if code := runCLIWith([]string{root, "1", "1"}, io.Discard, io.Discard, func(string, int, int) (result, error) { return result{}, errors.New("run") }); code != 1 {
		t.Fatal(code)
	}
	if code := runCLI([]string{root, "1", "1"}, badWriter{}, io.Discard); code != 1 {
		t.Fatal(code)
	}
	oldArgs, oldExit := os.Args, exit
	defer func() { os.Args = oldArgs; exit = oldExit }()
	os.Args = []string{"benchmark"}
	called := false
	exit = func(code int) {
		called = true
		if code != 2 {
			t.Fatal(code)
		}
	}
	main()
	if !called {
		t.Fatal("exit")
	}
}
