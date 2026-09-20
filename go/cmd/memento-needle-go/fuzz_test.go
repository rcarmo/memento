package main

import (
	"io"
	"strings"
	"testing"
)

func FuzzNeedleCommandArguments(f *testing.F) {
	f.Add("")
	f.Add("model\x00tokenizer")
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		args := []string{}
		if raw != "" {
			args = strings.Split(raw, "\x00")
		}
		code := run(args, strings.NewReader(""), io.Discard, io.Discard)
		if code != 1 && code != 2 {
			t.Fatalf("unexpected exit code %d", code)
		}
	})
}
