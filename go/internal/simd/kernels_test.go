package simd

import (
	"math"
	"os"
	"testing"
)

func TestSelectAndCapabilities(t *testing.T) {
	caps := Detect()
	if caps.Architecture == "" || caps.Available[0] != Scalar {
		t.Fatal(caps)
	}
	for _, value := range []string{"", "scalar", " SCALAR "} {
		backend, err := Select(value)
		if err != nil || backend != Scalar {
			t.Fatal(value, backend, err)
		}
	}
	if _, err := Select("bad"); err == nil {
		t.Fatal("bad")
	}
	for _, fixture := range []struct {
		caps Capabilities
		want Backend
	}{{Capabilities{Architecture: "amd64", SSE2: true, Available: []Backend{Scalar, SSE2}}, SSE2}, {Capabilities{Architecture: "amd64", SSE2: true, AVX2FMA: true, Available: []Backend{Scalar, SSE2, AVX2}}, AVX2}, {Capabilities{Architecture: "arm64", NEON: true, Available: []Backend{Scalar, NEON}}, NEON}, {Capabilities{Architecture: "other", Available: []Backend{Scalar}}, Scalar}} {
		got, selectErr := selectBackend("auto", fixture.caps)
		if selectErr != nil || got != fixture.want {
			t.Fatal(fixture, got, selectErr)
		}
	}
	if _, err := selectBackend("avx2", Capabilities{Architecture: "amd64", Available: []Backend{Scalar}}); err == nil {
		t.Fatal("unavailable")
	}
	old := os.Getenv("MEMENTO_SIMD")
	defer os.Setenv("MEMENTO_SIMD", old)
	if err := os.Setenv("MEMENTO_SIMD", "scalar"); err != nil {
		t.Fatal(err)
	}
	if backend, err := FromEnvironment(); err != nil || backend != Scalar {
		t.Fatal(backend, err)
	}
	engine, err := NewFromEnvironment()
	if err != nil || engine.Backend() != Scalar {
		t.Fatal(engine, err)
	}
	auto, err := Select("auto")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range caps.Available {
		found = found || candidate == auto
	}
	if !found {
		t.Fatal(auto, caps)
	}
}
func TestKernelsAgainstScalar(t *testing.T) {
	caps := Detect()
	for _, n := range []int{0, 1, 3, 4, 7, 8, 15, 16, 31, 32, 257} {
		a, b := make([]float32, n), make([]float32, n)
		for i := range a {
			a[i] = float32(i%13-6) / 7
			b[i] = float32(i%11-5) / 9
		}
		want := dotScalar(a, b)
		for _, backend := range caps.Available {
			got, err := Dot(backend, a, b)
			if err != nil || math.Abs(float64(got-want)) > 1e-4*math.Max(1, math.Abs(float64(want))) {
				t.Fatal(n, backend, want, got, err)
			}
			output := append([]float32{}, b...)
			expected := append([]float32{}, b...)
			axpyScalar(.25, a, expected)
			if err = AXPY(backend, .25, a, output); err != nil {
				t.Fatal(err)
			}
			for i := range output {
				if math.Float32bits(output[i]) != math.Float32bits(expected[i]) {
					t.Fatal(n, backend, i, expected[i], output[i])
				}
			}
		}
	}
}
func TestEngineKernels(t *testing.T) {
	engine, err := New("scalar")
	if err != nil {
		t.Fatal(err)
	}
	if value, err := engine.Dot([]float32{2}, []float32{3}); err != nil || value != 6 {
		t.Fatal(value, err)
	}
	out := []float32{1}
	if err = engine.AXPY(2, []float32{3}, out); err != nil || out[0] != 7 {
		t.Fatal(out, err)
	}
	if _, err = New("bad"); err == nil {
		t.Fatal("new")
	}
	bad := Engine{backend: Backend("bad")}
	if _, err = bad.Dot(nil, nil); err == nil {
		t.Fatal("dot")
	}
	if err = bad.AXPY(1, nil, nil); err == nil {
		t.Fatal("axpy")
	}
	native := Engine{backend: Detect().Available[len(Detect().Available)-1]}
	if native.backend != Scalar {
		if _, err = native.Dot([]float32{1}, []float32{2}); err != nil {
			t.Fatal(err)
		}
		values := []float32{1}
		if err = native.AXPY(1, []float32{2}, values); err != nil || values[0] != 3 {
			t.Fatal(values, err)
		}
	}
	if _, err = engine.Dot([]float32{1}, nil); err == nil {
		t.Fatal("dot dimensions")
	}
	if err = engine.AXPY(1, []float32{1}, nil); err == nil {
		t.Fatal("axpy dimensions")
	}
}
func TestKernelFailures(t *testing.T) {
	if _, err := Dot(Scalar, []float32{1}, nil); err == nil {
		t.Fatal("dot dimensions")
	}
	if err := AXPY(Scalar, 1, []float32{1}, nil); err == nil {
		t.Fatal("axpy dimensions")
	}
	for _, backend := range []Backend{SSE2, AVX2, NEON} {
		available := false
		for _, candidate := range Detect().Available {
			available = available || candidate == backend
		}
		if !available {
			if _, err := Dot(backend, nil, nil); err == nil {
				t.Fatal("unavailable dot", backend)
			}
			if err := AXPY(backend, 1, nil, nil); err == nil {
				t.Fatal("unavailable axpy", backend)
			}
		}
	}
	if _, err := Dot(Backend("bad"), nil, nil); err == nil {
		t.Fatal("dot backend")
	}
	if err := AXPY(Backend("bad"), 1, nil, nil); err == nil {
		t.Fatal("axpy backend")
	}
}
func BenchmarkDotScalar(b *testing.B) {
	x, y := make([]float32, 384), make([]float32, 384)
	b.ResetTimer()
	for range b.N {
		_ = dotScalar(x, y)
	}
}
func BenchmarkDotSSE2(b *testing.B) { benchmarkDotBackend(b, SSE2) }
func BenchmarkDotAVX2(b *testing.B) { benchmarkDotBackend(b, AVX2) }
func benchmarkDotBackend(b *testing.B, backend Backend) {
	engine, err := New(string(backend))
	if err != nil {
		b.Skip(err)
	}
	x, y := make([]float32, 384), make([]float32, 384)
	b.ResetTimer()
	for range b.N {
		_, _ = engine.Dot(x, y)
	}
}
func BenchmarkDotSelected(b *testing.B) {
	backend, _ := Select("auto")
	x, y := make([]float32, 384), make([]float32, 384)
	b.ResetTimer()
	engine, _ := New(string(backend))
	b.ResetTimer()
	for range b.N {
		_, _ = engine.Dot(x, y)
	}
}
