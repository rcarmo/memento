package vector

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestRustParity(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Method      string
		Bytes       []byte
		Dimensions  *int
		Left, Right []float32
		Alpha       float32
		Value       *float32
		Output      []float32
		Error       *string
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		var gotErr error
		switch c.Method {
		case "validate":
			var n int
			n, gotErr = ValidateF32LE(c.Bytes)
			if gotErr == nil && n != *c.Dimensions {
				t.Error(n, c.Dimensions)
			}
		case "dot", "cosine":
			var n float32
			if c.Method == "dot" {
				n, gotErr = Dot(c.Left, c.Right)
			} else {
				n, gotErr = Cosine(c.Left, c.Right)
			}
			if gotErr == nil && math.Abs(float64(n-*c.Value)) > 1e-6 {
				t.Errorf("%s got %v want %v", c.Method, n, *c.Value)
			}
		case "axpy":
			out := append([]float32{}, c.Right...)
			gotErr = AXPY(c.Alpha, c.Left, out)
			if !reflect.DeepEqual(out, c.Output) {
				t.Errorf("axpy got %v want %v", out, c.Output)
			}
		default:
			t.Fatal("unknown fixture method", c.Method)
		}
		if c.Error == nil && gotErr != nil {
			t.Error(gotErr)
		}
		if c.Error != nil && (gotErr == nil || gotErr.Error() != *c.Error) {
			t.Errorf("%s error got %v want %q", c.Method, gotErr, *c.Error)
		}
	}
}

func TestEncoding(t *testing.T) {
	for _, values := range [][]float32{{}, {0, float32(math.Copysign(0, -1)), 1, -3.5, math.SmallestNonzeroFloat32, math.MaxFloat32}} {
		blob, err := EncodeF32LE(values)
		if err != nil {
			t.Fatal(err)
		}
		out, err := DecodeF32LE(blob)
		if err != nil {
			t.Fatal(err)
		}
		for i := range values {
			if math.Float32bits(values[i]) != math.Float32bits(out[i]) {
				t.Fatal("roundtrip changed bits")
			}
		}
	}
	for _, v := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		if _, err := EncodeF32LE([]float32{0, v}); err == nil {
			t.Fatal("accepted nonfinite")
		}
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, math.Float32bits(v))
		if _, err := DecodeF32LE(b); err == nil {
			t.Fatal("decoded nonfinite")
		}
	}
	if _, err := DecodeF32LE([]byte{1, 2, 3}); err == nil {
		t.Fatal("decoded partial float")
	}
}

func FuzzF32LE(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 0, 128, 63})
	f.Add([]byte{255, 255, 255, 127})
	f.Fuzz(func(t *testing.T, blob []byte) {
		values, err := DecodeF32LE(blob)
		if err != nil {
			return
		}
		again, err := EncodeF32LE(values)
		if err != nil || !reflect.DeepEqual(blob, again) {
			t.Fatalf("roundtrip failed: %v", err)
		}
	})
}
