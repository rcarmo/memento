package simd

import (
	"math"
	"os"
	"reflect"
	"testing"
)

func TestSelectAndCapabilities(t *testing.T) {
	caps := Detect()
	if caps.Architecture == "" || caps.Available[0] != Scalar {
		t.Fatal(caps)
	}
	autoExpected, _ := Select("auto")
	if backend, err := Select(""); err != nil || backend != autoExpected {
		t.Fatal(backend, err)
	}
	for _, value := range []string{"scalar", " SCALAR "} {
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
func TestKernelsDeterministicRandomAndAliasing(t *testing.T) {
	caps := Detect()
	seed := uint32(0x9e3779b9)
	next := func() float32 { seed = seed*1664525 + 1013904223; return float32(int32(seed)) / float32(math.MaxInt32) }
	for _, n := range []int{4, 5, 127, 384, 385} {
		a, b := make([]float32, n), make([]float32, n)
		for i := range a {
			a[i], b[i] = next(), next()
		}
		want := dotScalar(a, b)
		for _, backend := range caps.Available {
			got, err := Dot(backend, a, b)
			if err != nil || math.Abs(float64(got-want)) > 1e-4*math.Max(1, math.Abs(float64(want))) {
				t.Fatalf("dot n=%d backend=%s want=%g got=%g err=%v", n, backend, want, got, err)
			}
			alias := append([]float32{}, a...)
			expected := append([]float32{}, a...)
			axpyScalar(.37, expected, expected)
			if err = AXPY(backend, .37, alias, alias); err != nil {
				t.Fatal(err)
			}
			for i := range alias {
				if math.Float32bits(alias[i]) != math.Float32bits(expected[i]) {
					t.Fatalf("alias n=%d backend=%s i=%d want=%g got=%g", n, backend, i, expected[i], alias[i])
				}
			}
		}
	}
}

func TestFusedKernelsMatchPrimitiveComposition(t *testing.T) {
	seed := uint32(0x9e3779b9)
	next := func() float32 { seed = seed*1664525 + 1013904223; return float32(int32(seed)) / float32(math.MaxInt32) }
	for _, rows := range []int{0, 1, 2, 3, 4, 5, 9} {
		for _, columns := range []int{0, 1, 3, 4, 7, 8, 15, 16, 31, 32, 257, 384, 385} {
			input, coefficients := make([]float32, columns), make([]float32, rows)
			matrix := make([]float32, rows*columns)
			for i := range input {
				input[i] = next()
			}
			for i := range coefficients {
				coefficients[i] = next()
			}
			for i := range matrix {
				matrix[i] = next()
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
						t.Fatalf("dot rows=%d cols=%d backend=%s row=%d want=%g got=%g", rows, columns, backend, i, wantDots[i], gotDots[i])
					}
				}
				wantAXPY, gotAXPY := make([]float32, columns), make([]float32, columns)
				for i := range wantAXPY {
					wantAXPY[i] = next()
				}
				copy(gotAXPY, wantAXPY)
				for row, coefficient := range coefficients {
					_ = engine.AXPY(coefficient, matrix[row*columns:(row+1)*columns], wantAXPY)
				}
				if err := engine.AXPYRows(coefficients, matrix, gotAXPY); err != nil {
					t.Fatal(err)
				}
				for i := range wantAXPY {
					if math.Float32bits(wantAXPY[i]) != math.Float32bits(gotAXPY[i]) {
						t.Fatalf("axpy rows=%d cols=%d backend=%s col=%d want=%g got=%g", rows, columns, backend, i, wantAXPY[i], gotAXPY[i])
					}
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
	dots := make([]float32, 2)
	if err = engine.DotRows([]float32{2}, []float32{3, 4}, dots); err != nil || !reflect.DeepEqual(dots, []float32{6, 8}) {
		t.Fatal(dots, err)
	}
	if err = engine.AXPYRows([]float32{2, 3}, []float32{4, 5}, out); err != nil || out[0] != 30 {
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
	if err = bad.DotRows(nil, nil, nil); err == nil {
		t.Fatal("dot rows")
	}
	if err = bad.AXPYRows(nil, nil, nil); err == nil {
		t.Fatal("axpy rows")
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
	if err := DotRows(Scalar, []float32{1}, []float32{1}, make([]float32, 2)); err == nil {
		t.Fatal("dot rows dimensions")
	}
	if err := AXPYRows(Scalar, []float32{1}, []float32{1}, make([]float32, 2)); err == nil {
		t.Fatal("axpy rows dimensions")
	}
	if err := DotRows(Backend("bad"), nil, nil, nil); err == nil {
		t.Fatal("dot rows backend")
	}
	if err := AXPYRows(Backend("bad"), nil, nil, nil); err == nil {
		t.Fatal("axpy rows backend")
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
func BenchmarkDotRowsSelected(b *testing.B) {
	engine, _ := New("auto")
	input, matrix, output := make([]float32, 384), make([]float32, 384*384), make([]float32, 384)
	for i := range input {
		input[i] = float32(i%17-8) / 9
	}
	for i := range matrix {
		matrix[i] = float32(i%23-11) / 13
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = engine.DotRows(input, matrix, output)
	}
}
func BenchmarkAXPYRowsSelected(b *testing.B) {
	engine, _ := New("auto")
	coefficients, matrix, output := make([]float32, 384), make([]float32, 384*384), make([]float32, 384)
	for i := range coefficients {
		coefficients[i] = float32(i%17-8) / 9
	}
	for i := range matrix {
		matrix[i] = float32(i%23-11) / 13
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		clear(output)
		_ = engine.AXPYRows(coefficients, matrix, output)
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
