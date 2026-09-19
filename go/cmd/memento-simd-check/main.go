package main

import (
	"fmt"
	msimd "github.com/rcarmo/memento/go/internal/simd"
	"io"
	"os"
)

type checkEngine interface {
	Dot([]float32, []float32) (float32, error)
	AXPY(float32, []float32, []float32) error
}
type checkOps struct {
	detect        func() msimd.Capabilities
	selectBackend func(string) (msimd.Backend, error)
	newEngine     func(string) (checkEngine, error)
}

func defaultCheckOps() checkOps {
	return checkOps{msimd.Detect, msimd.Select, func(value string) (checkEngine, error) { engine, err := msimd.New(value); return engine, err }}
}
func run(out io.Writer) error { return runWithOps(out, defaultCheckOps()) }
func runWithOps(out io.Writer, ops checkOps) error {
	caps := ops.detect()
	backend, err := ops.selectBackend("auto")
	if err != nil {
		return err
	}
	engine, err := ops.newEngine(string(backend))
	if err != nil {
		return err
	}
	left, right := make([]float32, 17), make([]float32, 17)
	for i := range left {
		left[i] = float32(i + 1)
		right[i] = float32(17 - i)
	}
	dot, err := engine.Dot(left, right)
	if err != nil {
		return err
	}
	values := append([]float32{}, right...)
	if err = engine.AXPY(.5, left, values); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "arch=%s backend=%s sse2=%t avx2_fma=%t neon=%t dot=%g axpy_tail=%g\n", caps.Architecture, backend, caps.SSE2, caps.AVX2FMA, caps.NEON, dot, values[len(values)-1])
	return err
}

var exit = os.Exit
var stdout, stderr io.Writer = os.Stdout, os.Stderr

func main() {
	if err := run(stdout); err != nil {
		fmt.Fprintln(stderr, err)
		exit(1)
	}
}
