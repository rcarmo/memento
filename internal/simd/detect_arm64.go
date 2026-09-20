//go:build arm64

package simd

import (
	"golang.org/x/sys/cpu"
	"runtime"
)

func detectArch() Capabilities {
	c := Capabilities{Architecture: runtime.GOARCH, NEON: cpu.ARM64.HasASIMD, Available: []Backend{Scalar}}
	if c.NEON {
		c.Available = append(c.Available, NEON)
	}
	return c
}
