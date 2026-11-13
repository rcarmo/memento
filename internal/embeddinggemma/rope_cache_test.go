package embeddinggemma

import (
	"math"
	"testing"
)

func TestRopeTileMatchesDirectRope(t *testing.T) {
	for _, global := range []bool{false, true} {
		name := "local"
		dimension := ropeLocalDimension
		if global {
			name = "global"
			dimension = ropeGlobalDimension
		}
		t.Run(name, func(t *testing.T) {
			for _, pos := range []int{0, 63, 64, 511, 512, MaxTokens - ropeTilePositions, MaxTokens - 1} {
				t.Run("position", func(t *testing.T) {
					base := pos / ropeTilePositions * ropeTilePositions
					count := min(ropeTilePositions, MaxTokens-base)
					var cache ropeTile
					cache.prepare(base, count, global)

					got := ropeFixture(dimension)
					want := append([]float32(nil), got...)
					cache.apply(got, pos)
					directRope(want, pos, global)
					for i := range got {
						if got[i] != want[i] {
							t.Fatalf("position=%d index=%d got=%g want=%g", pos, i, got[i], want[i])
						}
					}
				})
			}
		})
	}
}

func TestRopeTilePreservesNorm(t *testing.T) {
	for _, global := range []bool{false, true} {
		dimension := ropeLocalDimension
		if global {
			dimension = ropeGlobalDimension
		}
		for _, pos := range []int{0, 511, MaxTokens - 1} {
			base := pos / ropeTilePositions * ropeTilePositions
			var cache ropeTile
			cache.prepare(base, min(ropeTilePositions, MaxTokens-base), global)
			x := ropeFixture(dimension)
			before := squaredNorm(x)
			cache.apply(x, pos)
			if got := squaredNorm(x); math.Abs(got-before) > 2e-5*before {
				t.Fatalf("global=%t position=%d norm=%g want=%g", global, pos, got, before)
			}
		}
	}
}

func ropeFixture(dimension int) []float32 {
	x := make([]float32, dimension)
	for i := range x {
		x[i] = float32((i*37)%101-50) / 17
	}
	return x
}

func directRope(x []float32, pos int, global bool) {
	base := 10000.0
	if global {
		base = 1e6
	}
	half := len(x) / 2
	for i := 0; i < half; i++ {
		angle := float64(pos) * math.Pow(base, -float64(2*i)/float64(len(x)))
		s, c := math.Sincos(angle)
		a, b := x[i], x[i+half]
		x[i] = a*float32(c) - b*float32(s)
		x[i+half] = b*float32(c) + a*float32(s)
	}
}

func squaredNorm(x []float32) float64 {
	var sum float64
	for _, value := range x {
		sum += float64(value) * float64(value)
	}
	return sum
}
