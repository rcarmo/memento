//go:build amd64

package embeddinggemma

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/rcarmo/memento/internal/simd"
)

// q8ReferenceDot deliberately uses the test-only IEEE binary16 reference and
// scalar FP32 operations. It does not share the production half lookup or a
// production dequantizer with the fused kernels.
func q8ReferenceDot(raw []byte, x []float32) float32 {
	var sum float32
	for b := 0; b < len(x)/32; b++ {
		r := raw[b*34:]
		scale := float32(referenceHalf(binary.LittleEndian.Uint16(r)))
		for i := 0; i < 32; i++ {
			sum += scale * float32(int8(r[i+2])) * x[b*32+i]
		}
	}
	return sum
}

func requireQ8Close(t *testing.T, got, want float32) {
	t.Helper()
	if math.IsNaN(float64(want)) {
		if !math.IsNaN(float64(got)) {
			t.Fatalf("got %g want NaN", got)
		}
		return
	}
	if math.IsNaN(float64(got)) {
		t.Fatalf("got NaN want %g", want)
	}
	if math.IsInf(float64(want), 0) {
		if got != want {
			t.Fatalf("got %g want %g", got, want)
		}
		return
	}
	// SIMD reductions use a different, but valid, FP32 accumulation order.
	// The bound is deliberately relative to the independently decoded sum.
	limit := float32(3e-5) * max(float32(1), abs32(want))
	if abs32(got-want) > limit {
		t.Fatalf("got %.9g want %.9g (error %.9g > %.9g)", got, want, abs32(got-want), limit)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func TestQ8FusedDot(t *testing.T) {
	const blocks = 7
	raw := make([]byte, blocks*34)
	// Include normal, negative, subnormal and largest finite binary16 scales.
	scales := []uint16{0x3c00, 0x3800, 0xbc00, 0x0001, 0x3555, 0x7bff, 0x0400}
	for b, scale := range scales {
		binary.LittleEndian.PutUint16(raw[b*34:], scale)
		for i := 0; i < 32; i++ {
			raw[b*34+2+i] = byte(int8((b*37+i*19)%255 - 127))
		}
	}
	x := make([]float32, blocks*32)
	for i := range x {
		x[i] = float32(math.Sin(float64(i)*0.071) * math.Cos(float64(i)*0.013))
	}
	want := q8ReferenceDot(raw, x)
	for _, name := range []string{"sse2", "avx2"} {
		engine, err := simd.New(name)
		if err != nil {
			continue
		}
		got, ok := fusedQ8Dot(raw, x, engine.Backend())
		if !ok {
			t.Fatalf("%s did not fuse", name)
		}
		requireQ8Close(t, got, want)
	}
}

func TestQ8ExceptionalScales(t *testing.T) {
	for _, backend := range []string{"sse2", "avx2"} {
		engine, err := simd.New(backend)
		if err != nil {
			continue
		}
		for _, scale := range []uint16{0, 0x8000, 0x7c00, 0xfc00, 0x7e01} {
			raw := bytes.Repeat([]byte{1}, 34)
			binary.LittleEndian.PutUint16(raw, scale)
			x := make([]float32, 32)
			for i := range x {
				x[i] = 1
			}
			got, ok := fusedQ8Dot(raw, x, engine.Backend())
			if !ok {
				t.Fatal(backend)
			}
			requireQ8Close(t, got, q8ReferenceDot(raw, x))
		}
	}
}

func TestQ8FusedDotBoundsAndBackends(t *testing.T) {
	x := make([]float32, 32)
	raw := make([]byte, 34)
	binary.LittleEndian.PutUint16(raw, 0x3c00)
	for _, backend := range []simd.Backend{simd.Scalar, simd.NEON, simd.Backend("invalid")} {
		if _, ok := fusedQ8Dot(raw, x, backend); ok {
			t.Fatalf("unsupported backend %q fused", backend)
		}
	}
	if _, ok := fusedQ8Dot(raw, x[:31], simd.SSE2); ok {
		t.Fatal("partial activation block fused")
	}
	if _, ok := fusedQ8Dot(raw[:33], x, simd.SSE2); ok {
		t.Fatal("partial encoded block fused")
	}
	if _, ok := fusedQ8Dot(raw, append(x, 0), simd.SSE2); ok {
		t.Fatal("mismatched block count fused")
	}
	for _, name := range []string{"sse2", "avx2"} {
		engine, err := simd.New(name)
		if err != nil {
			continue
		}
		got, ok := fusedQ8Dot(nil, nil, engine.Backend())
		if !ok || got != 0 || math.Signbit(float64(got)) {
			t.Fatalf("%s empty: got %g fused=%t", name, got, ok)
		}
	}
}
