package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/gte"
)

func FuzzEmbedCommandArguments(f *testing.F) {
	for _, raw := range []string{"", "model", "a\x00b", "\x00"} {
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
		code := run(args, strings.NewReader(""), io.Discard, io.Discard, func(string) (*gte.Model, error) {
			return nil, errors.New("load")
		})
		if code != 1 && code != 2 {
			t.Fatalf("unexpected exit code %d", code)
		}
	})
}
