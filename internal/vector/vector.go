// Package vector provides scalar float32 reference kernels for the Go port.
// It intentionally has no SIMD, assembly or concurrent execution.
package vector

import (
	"encoding/binary"
	"fmt"
	"math"
)

// ValidateF32LE checks length and finite values, preserving Rust error text.
func ValidateF32LE(blob []byte) (int, error) {
	if len(blob)%4 != 0 {
		return 0, fmt.Errorf("f32le blob length %d is not a multiple of 4", len(blob))
	}
	for i := 0; i < len(blob)/4; i++ {
		value := math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return 0, fmt.Errorf("vector contains non-finite value at index %d", i)
		}
	}
	return len(blob) / 4, nil
}

// DecodeF32LE decodes a validated vector in the existing storage format.
func DecodeF32LE(blob []byte) ([]float32, error) {
	n, err := ValidateF32LE(blob)
	if err != nil {
		return nil, err
	}
	out := make([]float32, n)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return out, nil
}

// EncodeF32LE encodes finite values without changing the model/index format.
func EncodeF32LE(values []float32) ([]byte, error) {
	out := make([]byte, len(values)*4)
	for i, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("vector contains non-finite value at index %d", i)
		}
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(value))
	}
	return out, nil
}

// Dot computes the scalar dot product, explicitly rounding each float32 step.
func Dot(left, right []float32) (float32, error) {
	if len(left) != len(right) {
		return 0, fmt.Errorf("vector dimension mismatch: %d vs %d", len(left), len(right))
	}
	var sum float32
	for i, value := range left {
		sum = float32(sum + float32(value*right[i]))
	}
	return sum, nil
}

// AXPY adds alpha*values to output, preserving the scalar source operation order.
func AXPY(alpha float32, values, output []float32) error {
	if len(values) != len(output) {
		return fmt.Errorf("vector dimension mismatch: %d vs %d", len(values), len(output))
	}
	for i, value := range values {
		output[i] = float32(output[i] + float32(alpha*value))
	}
	return nil
}

// Cosine mirrors the source zero-norm and dimension checks.
func Cosine(left, right []float32) (float32, error) {
	if len(left) != len(right) {
		return 0, fmt.Errorf("vector dimension mismatch: %d vs %d", len(left), len(right))
	}
	ll, _ := Dot(left, left)
	rr, _ := Dot(right, right)
	if ll <= 0 || rr <= 0 {
		return 0, fmt.Errorf("zero-norm vector")
	}
	product, _ := Dot(left, right)
	denom := float32(float32(math.Sqrt(float64(ll))) * float32(math.Sqrt(float64(rr))))
	return float32(product / denom), nil
}
