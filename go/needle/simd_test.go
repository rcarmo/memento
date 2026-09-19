package needle

import (
	msimd "github.com/rcarmo/memento/go/internal/simd"
	"math"
	"testing"
)

func TestRouterSIMDConfiguration(t *testing.T) {
	router := &Router{}
	if router.SIMDBackend() != msimd.Scalar {
		t.Fatal(router.SIMDBackend())
	}
	if err := router.SetSIMD("scalar"); err != nil || router.simd != nil {
		t.Fatal(err)
	}
	if err := router.SetSIMD("bad"); err == nil {
		t.Fatal("bad")
	}
	backend, err := msimd.Select("auto")
	if err != nil {
		t.Fatal(err)
	}
	if err = router.SetSIMD(string(backend)); err != nil || router.SIMDBackend() != backend {
		t.Fatal(router.SIMDBackend(), err)
	}
}
func TestNeedleSIMDMath(t *testing.T) {
	backend, err := msimd.Select("auto")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := msimd.New(string(backend))
	if err != nil {
		t.Fatal(err)
	}
	q := []float32{.1, .2, .3, .4, .5, .6, .7, .8}
	k := []float32{.2, .3, .4, .5, .6, .7, .8, .9, .3, .2, .1, 0, .9, .8, .7, .6}
	v := append([]float32{}, k...)
	scalar := attendWithEngine(q, k, v, 2, 2, 4, nil)
	fast := attendWithEngine(q, k, v, 2, 2, 4, &engine)
	for i := range scalar {
		if math.Abs(float64(scalar[i]-fast[i])) > 1e-5 {
			t.Fatal(i, scalar[i], fast[i], backend)
		}
	}
	embedding := []float32{1, 0, 0, 1, -1, 0}
	hidden := []float32{.9, .1}
	if got, want := argmaxWithEngine(hidden, embedding, nil, &engine), argmaxWithEngine(hidden, embedding, nil, nil); got != want {
		t.Fatal(got, want)
	}
	if got, want := argmaxWithEngine(hidden, embedding, []int{1, 2}, &engine), argmaxWithEngine(hidden, embedding, []int{1, 2}, nil); got != want {
		t.Fatal(got, want)
	}
}
func BenchmarkArgmaxScalar(b *testing.B) { benchmarkArgmax(b, false) }
func BenchmarkArgmaxSIMD(b *testing.B)   { benchmarkArgmax(b, true) }
func benchmarkArgmax(b *testing.B, fast bool) {
	hidden, embedding := make([]float32, 384), make([]float32, 8192*384)
	var engine *msimd.Engine
	if fast {
		backend, _ := msimd.Select("auto")
		value, _ := msimd.New(string(backend))
		engine = &value
	}
	b.ResetTimer()
	for range b.N {
		_ = argmaxWithEngine(hidden, embedding, nil, engine)
	}
}
