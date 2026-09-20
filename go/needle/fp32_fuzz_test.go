package needle

import "testing"

func FuzzFP32Header(f *testing.F) {
	f.Add([]byte(fp32Magic), int64(fp32FixedSize))
	f.Add(make([]byte, fp32FixedSize), int64(fp32FixedSize))
	f.Fuzz(func(t *testing.T, raw []byte, size int64) {
		if len(raw) > fp32MaxHeader || size < 0 || size > fp32MaxFileSize+1 {
			return
		}
		_, _, _ = parseFP32Header(raw, size)
	})
}
