//go:build !unix

package needle

import (
	"errors"
)

func mapReadOnly(string) ([]byte, error) {
	return nil, errors.New("read-only model mapping is unsupported on this platform")
}
func unmapReadOnly([]byte) error { return nil }
