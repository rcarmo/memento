package main

import (
	"io"
	"strings"
	"testing"
)

func FuzzSkillImportArguments(f *testing.F) {
	for _, raw := range []string{"", "--bad", "--name\x00demo", "--workspace\x00.\x00--name\x00demo\x00--version\x001\x00--skill-md\x00missing\x00--zip\x00missing"} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 8192 {
			t.Skip()
		}
		args := []string{}
		if raw != "" {
			args = strings.Split(raw, "\x00")
		}
		options, err := parseImportArgs(args, io.Discard)
		if err == nil && (options.name == "" || options.version == "" || options.skillPath == "" || options.zipPath == "") {
			t.Fatalf("accepted incomplete arguments: %#v", args)
		}
	})
}
