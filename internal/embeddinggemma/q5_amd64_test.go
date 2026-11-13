//go:build amd64

package embeddinggemma

import (
	"math"
	"testing"

	"github.com/rcarmo/memento/internal/simd"
)

// q5ReferenceDot deliberately reads the Q5K bit planes directly. It does
// not use production half/scale/decode helpers, so the high-plane layout is
// checked independently of the fused kernels.
func q5ReferenceDot(raw []byte, x []float32) float32 {
	var sum float32
	for g := 0; g < 8; g++ {
		var scale uint8
		if g < 4 {
			scale = raw[4+g] & 63
		} else {
			j := g - 4
			scale = (raw[8+g] & 15) | ((raw[4+j] >> 6) << 4)
		}
		for i := 0; i < 32; i++ {
			q := raw[48+(g/2)*32+i]
			v := q & 15
			if g&1 != 0 {
				v = q >> 4
			}
			if raw[16+i]&(1<<g) != 0 {
				v |= 16
			}
			// The fixture uses d=1 and zero minimum, both exact in fp32.
			sum += float32(scale) * float32(v) * x[g*32+i]
		}
	}
	return sum
}

func TestQ5FusedDotHighPlanes(t *testing.T) {
	raw := make([]byte, 176)
	// d=1, dmin=0. The independent reference above intentionally relies on
	// these exact values rather than production half conversion.
	raw[0], raw[1] = 0, 0x3c
	for g := 0; g < 4; g++ {
		raw[4+g] = 1  // direct scale for groups 0..3
		raw[12+g] = 1 // low four bits of scale for groups 4..7
	}
	for i := 0; i < 32; i++ {
		// Every group sees a low value of 3; odd lanes have its selected
		// high-plane bit set, checking zero and set bits in all eight groups.
		raw[16+i] = 0xaa
	}
	for i := 0; i < 128; i++ {
		raw[48+i] = 0x33
	}
	x := make([]float32, 256)
	for i := range x {
		x[i] = float32(i%9-4) / 4 // signed, binary-exact activations
	}
	want := q5ReferenceDot(raw, x)
	for _, name := range []string{"sse2", "avx2"} {
		e, err := simd.New(name)
		if err != nil {
			continue
		}
		got, ok := fusedQ5Dot(raw, x, e.Backend())
		if !ok {
			t.Fatalf("%s did not fuse", name)
		}
		if got != want {
			t.Fatalf("%s: got %g want %g", name, got, want)
		}
	}
}

func TestQ5FusedDotZero(t *testing.T) {
	raw := make([]byte, 176)
	raw[0], raw[1] = 0, 0x3c // d=1
	x := make([]float32, 256)
	for i := range x {
		x[i] = float32(i%7 - 3)
	}
	for _, name := range []string{"sse2", "avx2"} {
		e, err := simd.New(name)
		if err != nil {
			continue
		}
		got, ok := fusedQ5Dot(raw, x, e.Backend())
		if !ok || got != 0 || math.Signbit(float64(got)) {
			t.Fatalf("%s: got %g fused=%t", name, got, ok)
		}
	}
}
