package simd

import (
	"encoding/binary"
	"math"
	"testing"
)

func fuzzFloats(raw []byte) []float32 {
	n := min(len(raw)/4, 1024)
	out := make([]float32, n)
	for i := range out {
		bits := binary.LittleEndian.Uint32(raw[i*4:])
		out[i] = float32(int32(bits)) / float32(math.MaxInt32)
	}
	return out
}

func FuzzKernels(f *testing.F) {
	f.Add([]byte{}, []byte{}, uint32(0x3f000000))
	f.Add([]byte{0, 0, 128, 63}, []byte{0, 0, 0, 64}, uint32(0x3f800000))
	f.Fuzz(func(t *testing.T, leftRaw, rightRaw []byte, alphaBits uint32) {
		left, right := fuzzFloats(leftRaw), fuzzFloats(rightRaw)
		if len(left) != len(right) {
			if _, err := (Engine{backend: Scalar}).Dot(left, right); err == nil {
				t.Fatal("dot accepted mismatched dimensions")
			}
			if err := (Engine{backend: Scalar}).AXPY(1, left, right); err == nil {
				t.Fatal("axpy accepted mismatched dimensions")
			}
			return
		}
		wantDot := dotScalar(left, right)
		alpha := float32(int32(alphaBits)) / float32(math.MaxInt32)
		for _, backend := range Detect().Available {
			engine := Engine{backend: backend}
			gotDot, err := engine.Dot(left, right)
			if err != nil {
				t.Fatal(err)
			}
			tolerance := 1e-4 * math.Max(1, math.Abs(float64(wantDot)))
			if math.Abs(float64(gotDot-wantDot)) > tolerance {
				t.Fatalf("dot mismatch for %s: want=%g got=%g", backend, wantDot, gotDot)
			}
			got, want := append([]float32{}, right...), append([]float32{}, right...)
			if err = engine.AXPY(alpha, left, got); err != nil {
				t.Fatal(err)
			}
			axpyScalar(alpha, left, want)
			for i := range want {
				if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
					t.Fatalf("axpy mismatch for %s at %d: want=%g got=%g", backend, i, want[i], got[i])
				}
			}
		}
	})
}

func FuzzBackendSelection(f *testing.F) {
	for _, value := range []string{"", "auto", "scalar", " SSE2 ", "avx2", "neon", "bad", "\x00"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		backend, err := Select(value)
		if err != nil {
			return
		}
		for _, available := range Detect().Available {
			if backend == available {
				return
			}
		}
		t.Fatalf("selected unavailable backend %q", backend)
	})
}
