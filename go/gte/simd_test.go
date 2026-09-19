package gte

import (
	msimd "github.com/rcarmo/memento/go/internal/simd"
	"math"
	"testing"
)

func TestLoadedModelDefaultsToAutoSIMD(t *testing.T) {
	t.Setenv("MEMENTO_SIMD", "")
	model, err := FromBytes(syntheticModel(0))
	if err != nil {
		t.Fatal(err)
	}
	want, err := msimd.Select("auto")
	if err != nil || model.SIMDBackend() != want {
		t.Fatal(model.SIMDBackend(), want, err)
	}
	t.Setenv("MEMENTO_SIMD", "scalar")
	model, err = FromBytes(syntheticModel(0))
	if err != nil || model.SIMDBackend() != msimd.Scalar {
		t.Fatal(model, err)
	}
	t.Setenv("MEMENTO_SIMD", "bad")
	if _, err = FromBytes(syntheticModel(0)); err == nil {
		t.Fatal("bad")
	}
}
func TestSIMDConfiguration(t *testing.T) {
	model := &Model{}
	if model.SIMDBackend() != msimd.Scalar {
		t.Fatal(model.SIMDBackend())
	}
	if err := model.SetSIMD("scalar"); err != nil || model.simd != nil {
		t.Fatal(err)
	}
	if err := model.SetSIMD("bad"); err == nil {
		t.Fatal("bad")
	}
	backend, err := msimd.Select("auto")
	if err != nil {
		t.Fatal(err)
	}
	if err = model.SetSIMD(string(backend)); err != nil || model.SIMDBackend() != backend {
		t.Fatal(model.SIMDBackend(), err)
	}
}
func TestLinearSIMDAgainstScalar(t *testing.T) {
	backend, err := msimd.Select("auto")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := msimd.New(string(backend))
	if err != nil {
		t.Fatal(err)
	}
	for _, shape := range [][3]int{{1, 1, 1}, {2, 3, 5}, {3, 17, 9}, {2, 384, 64}} {
		rows, in, out := shape[0], shape[1], shape[2]
		x, w, b := make([]float32, rows*in), make([]float32, out*in), make([]float32, out)
		for i := range x {
			x[i] = float32(i%11-5) / 7
		}
		for i := range w {
			w[i] = float32(i%13-6) / 9
		}
		for i := range b {
			b[i] = float32(i%5-2) / 11
		}
		scalar := linear(x, w, b, rows, in, out)
		fast := linearWithEngine(x, w, b, rows, in, out, &engine)
		for i := range scalar {
			if math.Abs(float64(scalar[i]-fast[i])) > 1e-4*math.Max(1, math.Abs(float64(scalar[i]))) {
				t.Fatal(shape, i, scalar[i], fast[i], backend)
			}
		}
	}
}
func BenchmarkLinearScalar(b *testing.B) { benchmarkLinear(b, false) }
func BenchmarkLinearSIMD(b *testing.B)   { benchmarkLinear(b, true) }
func benchmarkLinear(b *testing.B, fast bool) {
	const rows, in, out = 32, 384, 384
	x, w, bias := make([]float32, rows*in), make([]float32, out*in), make([]float32, out)
	var engine *msimd.Engine
	if fast {
		backend, _ := msimd.Select("auto")
		value, _ := msimd.New(string(backend))
		engine = &value
	}
	b.ResetTimer()
	for range b.N {
		_ = linearWithEngine(x, w, bias, rows, in, out, engine)
	}
}
