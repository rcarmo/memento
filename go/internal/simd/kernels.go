// Package simd provides opt-in architecture-specific float32 kernels.
// Scalar remains the default and correctness oracle because SIMD reduction and
// FMA change operation order relative to Memento's exact scalar path.
package simd

import (
	"fmt"
	"os"
	"strings"
)

type Backend string

const (
	Scalar Backend = "scalar"
	SSE2   Backend = "sse2"
	AVX2   Backend = "avx2"
	NEON   Backend = "neon"
)

type Engine struct{ backend Backend }

func New(value string) (Engine, error) { backend, err := Select(value); return Engine{backend}, err }
func NewFromEnvironment() (Engine, error) {
	backend, err := FromEnvironment()
	return Engine{backend}, err
}
func (e Engine) Backend() Backend { return e.backend }
func (e Engine) Dot(left, right []float32) (float32, error) {
	if len(left) != len(right) {
		return 0, fmt.Errorf("vector dimension mismatch: %d vs %d", len(left), len(right))
	}
	switch e.backend {
	case Scalar:
		return dotScalar(left, right), nil
	case SSE2, AVX2, NEON:
		return dotNative(e.backend, left, right), nil
	default:
		return 0, fmt.Errorf("unknown SIMD backend %q", e.backend)
	}
}
func (e Engine) AXPY(alpha float32, values, output []float32) error {
	if len(values) != len(output) {
		return fmt.Errorf("vector dimension mismatch: %d vs %d", len(values), len(output))
	}
	switch e.backend {
	case Scalar:
		axpyScalar(alpha, values, output)
		return nil
	case SSE2, AVX2, NEON:
		axpyNative(e.backend, alpha, values, output)
		return nil
	default:
		return fmt.Errorf("unknown SIMD backend %q", e.backend)
	}
}

type Capabilities struct {
	Architecture        string `json:"architecture"`
	SSE2, AVX2FMA, NEON bool
	Available           []Backend `json:"available"`
}

func Detect() Capabilities                 { return detectArch() }
func Select(value string) (Backend, error) { return selectBackend(value, Detect()) }
func selectBackend(value string, caps Capabilities) (Backend, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = "auto"
	}
	if value == "auto" {
		if caps.AVX2FMA {
			return AVX2, nil
		}
		if caps.SSE2 {
			return SSE2, nil
		}
		if caps.NEON {
			return NEON, nil
		}
		return Scalar, nil
	}
	backend := Backend(value)
	for _, candidate := range caps.Available {
		if candidate == backend {
			return backend, nil
		}
	}
	return Scalar, fmt.Errorf("SIMD backend %q is unavailable on %s", value, caps.Architecture)
}
func FromEnvironment() (Backend, error) { return Select(os.Getenv("MEMENTO_SIMD")) }
func Dot(backend Backend, left, right []float32) (float32, error) {
	engine, err := New(string(backend))
	if err != nil {
		return 0, err
	}
	return engine.Dot(left, right)
}
func AXPY(backend Backend, alpha float32, values, output []float32) error {
	engine, err := New(string(backend))
	if err != nil {
		return err
	}
	return engine.AXPY(alpha, values, output)
}
func dotScalar(a, b []float32) float32 {
	var sum float32
	for i, v := range a {
		sum = float32(sum + float32(v*b[i]))
	}
	return sum
}
func axpyScalar(alpha float32, x, y []float32) {
	for i, v := range x {
		y[i] = float32(y[i] + float32(alpha*v))
	}
}
