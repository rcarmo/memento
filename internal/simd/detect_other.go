//go:build !amd64 && !arm64

package simd

import "runtime"

func detectArch() Capabilities {
	return Capabilities{Architecture: runtime.GOARCH, Available: []Backend{Scalar}}
}
