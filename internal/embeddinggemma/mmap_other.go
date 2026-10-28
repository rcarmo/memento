//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package embeddinggemma

import (
	"errors"
	"os"
)

func mapReadOnly(_ *os.File, _ int) ([]byte, error) {
	return nil, errors.New("embeddinggemma: read-only model mapping unsupported on this platform")
}
func unmap(_ []byte) error { return nil }
