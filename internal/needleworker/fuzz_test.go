package needleworker

import (
	"bytes"
	"testing"
)

func FuzzFrameParsing(f *testing.F) {
	f.Add([]byte{2, 0, 0, 0, '{', '}'})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > DefaultMaxFrameBytes+4 {
			return
		}
		var request Request
		_ = ReadFrame(bytes.NewReader(raw), &request, DefaultMaxFrameBytes)
	})
}
