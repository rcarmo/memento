package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func FuzzGreetingPayload(f *testing.F) {
	for _, name := range []string{"world", "", "Rui", "世界", "\x00"} {
		f.Add(name)
	}
	f.Fuzz(func(t *testing.T, name string) {
		if len(name) > 1<<20 {
			t.Skip()
		}
		payload := map[string]any{"greeting": fmt.Sprintf("Hello, %s!", name)}
		raw, err := json.Marshal(payload)
		if err != nil || !json.Valid(raw) {
			t.Fatalf("invalid greeting payload: %q %v", raw, err)
		}
	})
}
