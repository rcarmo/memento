package main

import (
	"strings"
	"testing"
)

func FuzzBenchmarkCLIArguments(f *testing.F) {
	for _, raw := range []string{"", "/tmp\x001\x001", "/tmp\x000\x001", "/tmp\x001e3\x00-1", "x\x001\x001\x00extra"} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		args := []string{}
		if raw != "" {
			args = strings.Split(raw, "\x00")
		}
		root, rebuilds, searches, err := parseBenchmarkArgs(args)
		if err == nil && (len(args) != 3 || root != args[0] || rebuilds < 1 || searches < 1) {
			t.Fatalf("accepted invalid arguments: %#v", args)
		}
	})
}
