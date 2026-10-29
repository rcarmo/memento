//go:build amd64

package embeddinggemma

import (
	"encoding/binary"
	"math"
	"math/rand"
	"testing"

	"github.com/rcarmo/memento/internal/simd"
)

// Exercise multi-block accumulation and all packed scale/min bits independently
// of scaleK and the production half decoder. The model uses up to 2048 columns.
func TestQ4RowAVX2(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	rng := rand.New(rand.NewSource(42))
	for _, blocks := range []int{1, 2, 4, 8, 16} {
		raw := make([]byte, blocks*144)
		rng.Read(raw)
		for b := 0; b < blocks; b++ {
			binary.LittleEndian.PutUint16(raw[b*144:], 0x2000+uint16(b))
			binary.LittleEndian.PutUint16(raw[b*144+2:], 0x1800+uint16(b))
		}
		x := make([]float32, blocks*256)
		for i := range x {
			x[i] = float32(rng.NormFloat64())
		}
		want := 0.0
		decoded := referenceRow(raw, Q4_K, len(x))
		for i, a := range x {
			want += decoded[i] * float64(a)
		}
		for _, backend := range []simd.Backend{simd.SSE2, simd.AVX2} {
			if _, err := simd.New(string(backend)); err != nil {
				continue
			}
			got, ok := fusedQ4Dot(raw, x, backend)
			if !ok || math.Abs(float64(got)-want) > 2e-5*math.Max(1, math.Abs(want)) {
				t.Fatal(blocks, backend, got, want)
			}
		}
	}
	for _, backend := range []simd.Backend{simd.AVX2, simd.SSE2} {
		if _, err := simd.New(string(backend)); err != nil {
			continue
		}
		if got, ok := fusedQ4Dot(nil, nil, backend); !ok || got != 0 {
			t.Fatal(got, ok)
		}
		if _, ok := fusedQ4Dot(make([]byte, 143), make([]float32, 256), backend); ok {
			t.Fatal("raw bounds")
		}
		if _, ok := fusedQ4Dot(make([]byte, 144), make([]float32, 255), backend); ok {
			t.Fatal("activation bounds")
		}
	}
}

func TestQuantHalfLookup(t *testing.T) {
	for i, got := range quantHalfTable {
		want := referenceHalf(uint16(i))
		if math.IsNaN(float64(want)) {
			if !math.IsNaN(float64(got)) {
				t.Fatal(i)
			}
		} else if math.Float32bits(got) != math.Float32bits(float32(want)) {
			t.Fatal(i, got, want)
		}
	}
}
