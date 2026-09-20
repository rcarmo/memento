package needle

import (
	"encoding/json"
	"testing"
)

func FuzzNeedleJSONAndConstraints(f *testing.F) {
	for _, seed := range [][2]string{
		{`[]`, `{"name":"`},
		{`[{"name":"memory_status","parameters":{"path":{"type":"string"}}}]`, `{"name":"memory_status","arguments":{"path":"x"}}`},
		{`null`, "\xff"},
		{`{}`, `[{"name":"x"}]`},
	} {
		f.Add(seed[0], seed[1])
	}
	tokenizer, err := TokenizerFromBytes(syntheticTokenizer(true))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, tools, generated string) {
		if len(tools)+len(generated) > 1<<16 {
			t.Skip()
		}
		normalized, names := NormalizeToolsJSON(tools)
		if value, ok := parseJSON(tools); ok {
			if _, list := value.([]any); list && !json.Valid([]byte(normalized)) {
				t.Fatalf("normalization produced invalid JSON: %q", normalized)
			}
		}
		_ = RestoreToolNames(generated, names)
		constraints := newConstraints(normalized, tokenizer)
		for _, ch := range []byte(generated) {
			constraints.machine.feedRune(rune(ch))
			allowed := constraints.allowed()
			for _, id := range allowed {
				if id < 0 || id >= tokenizer.VocabSize() {
					t.Fatalf("invalid allowed token %d", id)
				}
			}
		}
	})
}

func FuzzSnakeCase(f *testing.F) {
	for _, value := range []string{"memoryStatus", "HTTPServer", "two words", "éclair", "", "a__b"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 1<<20 {
			t.Skip()
		}
		got := SnakeCase(value)
		if SnakeCase(got) != got {
			t.Fatalf("not idempotent: %q -> %q", value, got)
		}
	})
}
