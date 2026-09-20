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

func FuzzFusedKernels(f *testing.F) {
	f.Add([]byte{0, 0, 128, 63}, []byte{0, 0, 0, 64}, uint8(1), uint8(1))
	f.Add([]byte{}, []byte{}, uint8(0), uint8(0))
	f.Fuzz(func(t *testing.T, inputRaw, matrixRaw []byte, rowsRaw, columnsRaw uint8) {
		rows, columns := int(rowsRaw%9), int(columnsRaw%33)
		if rows*columns > len(matrixRaw)/4 || columns > len(inputRaw)/4 {
			return
		}
		input := fuzzFloats(inputRaw)[:columns]
		matrix := fuzzFloats(matrixRaw)[:rows*columns]
		coefficients := append([]float32{}, input[:min(rows, len(input))]...)
		for len(coefficients) < rows {
			coefficients = append(coefficients, float32(len(coefficients)+1)/7)
		}
		for _, backend := range Detect().Available {
			engine := Engine{backend: backend}
			wantDots, gotDots := make([]float32, rows), make([]float32, rows)
			for row := range wantDots {
				wantDots[row], _ = engine.Dot(input, matrix[row*columns:(row+1)*columns])
			}
			if err := engine.DotRows(input, matrix, gotDots); err != nil {
				t.Fatal(err)
			}
			for i := range wantDots {
				if math.Float32bits(wantDots[i]) != math.Float32bits(gotDots[i]) {
					t.Fatalf("dot rows mismatch for %s", backend)
				}
			}
			wantAXPY, gotAXPY := make([]float32, columns), make([]float32, columns)
			for row, coefficient := range coefficients {
				_ = engine.AXPY(coefficient, matrix[row*columns:(row+1)*columns], wantAXPY)
			}
			if err := engine.AXPYRows(coefficients, matrix, gotAXPY); err != nil {
				t.Fatal(err)
			}
			for i := range wantAXPY {
				if math.Float32bits(wantAXPY[i]) != math.Float32bits(gotAXPY[i]) {
					t.Fatalf("axpy rows mismatch for %s", backend)
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
