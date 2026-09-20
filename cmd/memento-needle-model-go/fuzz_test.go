package main

import (
	"io"
	"strings"
	"testing"
)

func FuzzNeedleModelCommandArguments(f *testing.F) {
	f.Add("")
	f.Add("source\x00destination")
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		args := []string{}
		if raw != "" {
			args = strings.Split(raw, "\x00")
		}
		code := run(args, io.Discard, io.Discard)
		if code != 1 && code != 2 {
			t.Fatalf("unexpected exit code %d", code)
		}
	})
}
