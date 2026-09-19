package main

import (
	"bytes"
	"errors"
	msimd "github.com/rcarmo/memento/go/internal/simd"
	"io"
	"strings"
	"testing"
)

type stubEngine struct{ dotErr, axpyErr error }

func (s stubEngine) Dot([]float32, []float32) (float32, error) { return 1, s.dotErr }
func (s stubEngine) AXPY(float32, []float32, []float32) error  { return s.axpyErr }

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write") }
func TestRun(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out); err != nil || !strings.Contains(out.String(), "backend=") || !strings.Contains(out.String(), "dot=") {
		t.Fatal(out.String(), err)
	}
	boom := errors.New("boom")
	base := defaultCheckOps()
	for i, mutate := range []func(*checkOps){func(o *checkOps) { o.selectBackend = func(string) (msimd.Backend, error) { return msimd.Scalar, boom } }, func(o *checkOps) { o.newEngine = func(string) (checkEngine, error) { return nil, boom } }, func(o *checkOps) {
		o.newEngine = func(string) (checkEngine, error) { return stubEngine{dotErr: boom}, nil }
	}, func(o *checkOps) {
		o.newEngine = func(string) (checkEngine, error) { return stubEngine{axpyErr: boom}, nil }
	}} {
		ops := base
		mutate(&ops)
		if err := runWithOps(io.Discard, ops); !errors.Is(err, boom) {
			t.Fatal(i, err)
		}
	}
	if err := runWithOps(failWriter{}, base); err == nil {
		t.Fatal("write")
	}
}
func TestMain(t *testing.T) {
	oldExit, oldOut, oldErr := exit, stdout, stderr
	defer func() { exit, stdout, stderr = oldExit, oldOut, oldErr }()
	called := false
	exit = func(int) { called = true }
	stdout = failWriter{}
	stderr = io.Discard
	main()
	if !called {
		t.Fatal("exit")
	}
}
