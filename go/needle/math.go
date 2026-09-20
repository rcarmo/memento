package needle

import (
	msimd "github.com/rcarmo/memento/go/internal/simd"
	"math"
)

func sqrt32(x float32) float32 { return float32(math.Sqrt(float64(x))) }

type rope struct{ cos, sin []float32 }

func precomputeRope(dim, length int, theta float32) rope {
	half := dim / 2
	r := rope{make([]float32, length*half), make([]float32, length*half)}
	for t := 0; t < length; t++ {
		for i := 0; i < half; i++ {
			exponent := float32(2*i) / float32(dim)
			freq := float32(1) / float32(math.Pow(float64(theta), float64(exponent)))
			angle := float32(float32(t) * freq)
			r.cos[t*half+i] = float32(math.Cos(float64(angle)))
			r.sin[t*half+i] = float32(math.Sin(float64(angle)))
		}
	}
	return r
}
func applyRope(x []float32, heads, dim int, r rope, pos int) {
	half := dim / 2
	for h := 0; h < heads; h++ {
		base := h * dim
		for i := 0; i < half; i++ {
			c, s := r.cos[pos*half+i], r.sin[pos*half+i]
			a, b := x[base+i], x[base+half+i]
			x[base+i] = float32(float32(a*c) - float32(b*s))
			x[base+half+i] = float32(float32(b*c) + float32(a*s))
		}
	}
}
func ropeRows(x []float32, heads, dim int, r rope) {
	for row := 0; row < len(x)/(heads*dim); row++ {
		applyRope(x[row*heads*dim:(row+1)*heads*dim], heads, dim, r, row)
	}
}
func resizeFloat32(buffer []float32, size int) []float32 {
	if cap(buffer) < size {
		return make([]float32, size)
	}
	return buffer[:size]
}
func normVectorInto(out, x, scale []float32) []float32 {
	out = resizeFloat32(out, len(x))
	copy(out, x)
	headNorm(out, 1, len(x), scale)
	return out
}
func normRowsInto(out, x []float32, width int, scale []float32) []float32 {
	out = resizeFloat32(out, len(x))
	copy(out, x)
	headNorm(out, len(x)/width, width, scale)
	return out
}
func headNorm(x []float32, heads, dim int, scale []float32) {
	for h := 0; h < heads; h++ {
		row := x[h*dim : (h+1)*dim]
		var sum float32
		for _, v := range row {
			sum = float32(sum + float32(v*v))
		}
		rms := float32(math.Sqrt(float64(float32(sum/float32(dim) + float32(1e-6)))))
		for i, v := range row {
			row[i] = float32(float32(float32(1+scale[i])*v) / rms)
		}
	}
}
func project(input []float32, in, out int, kernel []float32) []float32 {
	return projectWithEngine(input, in, out, kernel, nil)
}
func projectWithEngine(input []float32, in, out int, kernel []float32, engine *msimd.Engine) []float32 {
	return projectInto(nil, input, in, out, kernel, engine)
}
func projectInto(result, input []float32, in, out int, kernel []float32, engine *msimd.Engine) []float32 {
	rows := len(input) / in
	result = resizeFloat32(result, rows*out)
	clear(result)
	for r := 0; r < rows; r++ {
		row := result[r*out : (r+1)*out]
		coefficients := input[r*in : (r+1)*in]
		if engine != nil {
			_ = engine.AXPYRows(coefficients, kernel, row)
			continue
		}
		for i, value := range coefficients {
			weights := kernel[i*out : (i+1)*out]
			for j := 0; j < out; j++ {
				row[j] = float32(row[j] + float32(value*weights[j]))
			}
		}
	}
	return result
}
func dot(a, b []float32) float32 {
	var sum float32
	for i, v := range a {
		sum = float32(sum + float32(v*b[i]))
	}
	return sum
}
func attendWithEngine(q, k, v []float32, heads, kvHeads, dim int, engine *msimd.Engine) []float32 {
	return attendInto(nil, nil, q, k, v, heads, kvHeads, dim, engine)
}
func attendInto(out, scores, q, k, v []float32, heads, kvHeads, dim int, engine *msimd.Engine) []float32 {
	tokens := len(k) / (kvHeads * dim)
	repeats := heads / kvHeads
	out = resizeFloat32(out, heads*dim)
	clear(out)
	scores = resizeFloat32(scores, tokens)
	scale := float32(math.Sqrt(float64(float32(dim))))
	for h := 0; h < heads; h++ {
		kh := h / repeats
		maxScore := float32(math.Inf(-1))
		for token := range scores {
			base := token*kvHeads*dim + kh*dim
			var product float32
			if engine != nil {
				product, _ = engine.Dot(q[h*dim:(h+1)*dim], k[base:base+dim])
			} else {
				product = dot(q[h*dim:(h+1)*dim], k[base:base+dim])
			}
			score := float32(product / scale)
			scores[token] = score
			if score > maxScore {
				maxScore = score
			}
		}
		var sum float32
		for i, s := range scores {
			scores[i] = float32(math.Exp(float64(float32(s - maxScore))))
			sum = float32(sum + scores[i])
		}
		if sum == 0 {
			continue
		}
		for token, score := range scores {
			weight := float32(score / sum)
			base := token*kvHeads*dim + kh*dim
			for i := 0; i < dim; i++ {
				out[h*dim+i] = float32(out[h*dim+i] + float32(weight*v[base+i]))
			}
		}
	}
	return out
}
func sigmoid(x float32) float32 { return float32(1) / float32(1+float32(math.Exp(float64(-x)))) }
func gatedResidual(x, other []float32, gate float32) {
	g := sigmoid(gate)
	for i := range x {
		x[i] = float32(x[i] + float32(g*other[i]))
	}
}
func argmaxWithEngine(hidden, embedding []float32, allowed []int, engine *msimd.Engine) int {
	dim := len(hidden)
	best := 0
	maximum := float32(math.Inf(-1))
	if len(allowed) == 0 {
		for id := 0; id < len(embedding)/dim; id++ {
			var score float32
			if engine != nil {
				score, _ = engine.Dot(hidden, embedding[id*dim:(id+1)*dim])
			} else {
				score = dot(hidden, embedding[id*dim:(id+1)*dim])
			}
			if score > maximum {
				maximum = score
				best = id
			}
		}
	} else {
		for _, id := range allowed {
			var score float32
			if engine != nil {
				score, _ = engine.Dot(hidden, embedding[id*dim:(id+1)*dim])
			} else {
				score = dot(hidden, embedding[id*dim:(id+1)*dim])
			}
			if score > maximum {
				maximum = score
				best = id
			}
		}
	}
	return best
}
