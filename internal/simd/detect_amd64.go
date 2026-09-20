//go:build amd64

package simd

import (
	"golang.org/x/sys/cpu"
	"runtime"
)

func detectArch() Capabilities {
	c := Capabilities{Architecture: runtime.GOARCH, SSE2: cpu.X86.HasSSE2, AVX2FMA: cpu.X86.HasAVX2 && cpu.X86.HasFMA, Available: []Backend{Scalar}}
	if c.SSE2 {
		c.Available = append(c.Available, SSE2)
	}
	if c.AVX2FMA {
		c.Available = append(c.Available, AVX2)
	}
	return c
}
