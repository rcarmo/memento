package main

import (
	"bytes"
	"math"
	"strings"
	"testing"

	msimd "github.com/rcarmo/memento/go/internal/simd"
)

type fuzzEngine struct{ dot, tail float32 }

func (e fuzzEngine) Dot([]float32, []float32) (float32, error) { return e.dot, nil }
func (e fuzzEngine) AXPY(_ float32, _ []float32, output []float32) error {
	output[len(output)-1] = e.tail
	return nil
}

func FuzzSIMDCheckOutput(f *testing.F) {
	f.Add(uint32(0x3f800000), uint32(0x40000000), "amd64")
	f.Add(uint32(0x7fc00000), uint32(0x7f800000), "arm64")
	f.Fuzz(func(t *testing.T, dotBits, tailBits uint32, architecture string) {
		if len(architecture) > 1024 {
			t.Skip()
		}
		engine := fuzzEngine{math.Float32frombits(dotBits), math.Float32frombits(tailBits)}
		ops := checkOps{
			detect: func() msimd.Capabilities {
				return msimd.Capabilities{Architecture: architecture, Available: []msimd.Backend{msimd.Scalar}}
			},
			selectBackend: func(string) (msimd.Backend, error) { return msimd.Scalar, nil },
			newEngine:     func(string) (checkEngine, error) { return engine, nil },
		}
		var out bytes.Buffer
		if err := runWithOps(&out, ops); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "backend=scalar") || !strings.HasSuffix(out.String(), "\n") {
			t.Fatalf("invalid diagnostic output %q", out.String())
		}
	})
}
