package embeddinggemma

import (
	"math"
	"math/rand"
	"testing"
)

// Standalone reference: do not use the candidate table or model.gelu.
func fastGELUReference(x float32) float32 {
	return .5 * x * (1 + float32(math.Tanh(.7978845608028654*float64(x)*(1+.044715*float64(x)*float64(x)))))
}

func fastGELUULP(a, b float32) uint32 {
	ordered := func(x float32) uint32 {
		v := math.Float32bits(x)
		if v>>31 != 0 {
			return ^v
		}
		return v | 1<<31
	}
	x, y := ordered(a), ordered(b)
	if x < y {
		return y - x
	}
	return x - y
}

func TestFastGELUAccuracy(t *testing.T) {
	var maxError, sumSquares float64
	var maxULP uint32
	var worst, worstULPX float32
	count := 0
	check := func(x float32) {
		a, b := fastGELU(x), fastGELUReference(x)
		e := math.Abs(float64(a) - float64(b))
		if e > maxError {
			maxError, worst = e, x
		}
		if u := fastGELUULP(a, b); u > maxULP {
			maxULP, worstULPX = u, x
		}
		sumSquares += e * e
		count++
		// The absolute gate handles cancellation near the negative tail;
		// ULP distance is reported, not substituted for this bound.
		if math.IsNaN(float64(a)) || math.IsInf(float64(a), 0) || e > 1e-7 {
			t.Fatalf("x=%g got=%.9g want=%.9g abs=%g ULP=%d", x, a, b, e, fastGELUULP(a, b))
		}
	}
	for i := 0; i <= 1_000_000; i++ {
		check(float32(-8 + 16*float64(i)/1_000_000))
	}
	rng := rand.New(rand.NewSource(7541))
	for i := 0; i < 1_000_000; i++ {
		check(float32(-8 + 16*rng.Float64()))
	}
	// Concentrate on table knots, cancellation and saturation boundaries.
	for _, center := range []float32{-6, -5.2, -5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5, 5.2, 6} {
		lo, hi := center, center
		for i := 0; i < 4096; i++ {
			check(lo)
			check(hi)
			lo = math.Nextafter32(lo, float32(math.Inf(-1)))
			hi = math.Nextafter32(hi, float32(math.Inf(1)))
		}
	}
	// Invert the transform to probe actual interpolation knots on both sides.
	for i := 1; i <= 2336; i++ {
		u := float64(i) / 256
		x := u / .7978845608028654
		for j := 0; j < 12; j++ {
			x -= (.7978845608028654*x*(1+.044715*x*x) - u) / (.7978845608028654 * (1 + 3*.044715*x*x))
		}
		lo, hi := float32(x), float32(x)
		for j := 0; j < 16; j++ {
			check(lo)
			check(-lo)
			check(hi)
			check(-hi)
			lo = math.Nextafter32(lo, float32(math.Inf(-1)))
			hi = math.Nextafter32(hi, float32(math.Inf(1)))
		}
	}
	rms := math.Sqrt(sumSquares / float64(count))
	if rms > 1e-8 {
		t.Fatal("GELU RMS gate", rms)
	}
	t.Logf("samples=%d maxAbs=%g RMS=%g maxULP=%d worstX=%.9g worstULPX=%.9g", count, maxError, rms, maxULP, worst, worstULPX)
}

func TestFastGELUExceptions(t *testing.T) {
	for _, x := range []float32{0, math.Float32frombits(1 << 31), math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32, math.MaxFloat32, -math.MaxFloat32, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())} {
		got, want := fastGELU(x), fastGELUReference(x)
		if math.IsNaN(float64(want)) {
			if !math.IsNaN(float64(got)) {
				t.Fatalf("x=%g got=%g want=NaN", x, got)
			}
		} else if got != want || math.Signbit(float64(got)) != math.Signbit(float64(want)) {
			t.Fatalf("x=%g got=%g want=%g (including sign)", x, got, want)
		}
	}
}

var fastGELUBenchmarkSink float32

func BenchmarkFastGELU(b *testing.B) {
	var values [4096]float32
	rng := rand.New(rand.NewSource(7541))
	for i := range values {
		values[i] = float32(rng.NormFloat64() * 2)
	}
	b.Run("Reference", func(b *testing.B) {
		var result float32
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			result = fastGELUReference(values[i&4095])
		}
		fastGELUBenchmarkSink = result
	})
	b.Run("Candidate", func(b *testing.B) {
		var result float32
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			result = fastGELU(values[i&4095])
		}
		fastGELUBenchmarkSink = result
	})
}

// Broad bit-pattern coverage includes tiny inputs and finite saturated tails.
func FuzzFastGELU(f *testing.F) {
	for _, x := range []float32{0, -0.5, 1.53015995, -1.53015995, -6, 6, math.MaxFloat32, -math.MaxFloat32} {
		f.Add(math.Float32bits(x))
	}
	f.Fuzz(func(t *testing.T, bits uint32) {
		x := math.Float32frombits(bits)
		got, want := fastGELU(x), fastGELUReference(x)
		if math.IsNaN(float64(want)) {
			if !math.IsNaN(float64(got)) {
				t.Fatalf("x=%g got=%g want=NaN", x, got)
			}
			return
		}
		if math.IsInf(float64(want), 0) {
			if got != want {
				t.Fatalf("x=%g got=%g want=%g", x, got, want)
			}
			return
		}
		err := math.Abs(float64(got) - float64(want))
		if math.IsNaN(float64(got)) || math.IsInf(float64(got), 0) || err > 1e-7 {
			t.Fatalf("x=%g got=%g want=%g abs=%g ULP=%d", x, got, want, err, fastGELUULP(got, want))
		}
		if want == 0 && (got != 0 || math.Signbit(float64(got)) != math.Signbit(float64(want))) {
			t.Fatalf("x=%g signed-zero mismatch", x)
		}
	})
}
