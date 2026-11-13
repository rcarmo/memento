package embeddinggemma

import "math"

// A small, read-only-after-init Hermite table approximates tanh, not GELU:
// rounding tanh to float32 before the final operations is part of the model's
// numerical contract, particularly for negative inputs near saturation.
// Step 1/256 makes interpolation error far smaller than a float32 ULP.
const fastGELUTanhScale = 256
const fastGELUTanhEnd = 9.125

// Precompute cubic coefficients to avoid recomputing adjacent differences on
// every call. The fixed 73 KiB table is shared by all workspaces.
var fastGELUTanhTable = func() [2336]struct{ a, b, c, d float64 } {
	var table [2336]struct{ a, b, c, d float64 }
	t0 := math.Tanh(0)
	m0 := (1 - t0*t0) / fastGELUTanhScale
	for i := range table {
		t1 := math.Tanh(float64(i+1) / fastGELUTanhScale)
		m1 := (1 - t1*t1) / fastGELUTanhScale
		delta := t1 - t0
		table[i].a = t0
		table[i].b = m0
		table[i].c = 3*delta - 2*m0 - m1
		table[i].d = m0 + m1 - 2*delta
		t0, m0 = t1, m1
	}
	return table
}()

// fastGELU is a candidate for the tanh GELU used by the model. It allocates
// nothing per call. Keep the float32 operations in the same order as gelu.
// The table is never written after package initialization.
func fastGELU(x float32) float32 {
	if !(x > -6 && x < 6) {
		if x >= 6 {
			return .5 * x * 2
		}
		if x <= -6 {
			return .5 * x * 0 // Includes -Inf -> NaN and finite -> negative zero.
		}
		return x // NaN, never converted to a table index.
	}
	y := float64(x)
	u := math.Abs(.7978845608028654 * y * (1 + .044715*y*y))
	t := float64(1)
	if u < fastGELUTanhEnd {
		position := u * fastGELUTanhScale
		i := int(position)
		f := position - float64(i)
		p := &fastGELUTanhTable[i]
		t = p.a + f*(p.b+f*(p.c+f*p.d))
	}
	return .5 * x * (1 + float32(math.Copysign(t, y)))
}
