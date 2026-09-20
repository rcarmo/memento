package needle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// GenerationOptions mirrors the source's explicit generation bounds/mode.
type GenerationOptions struct {
	MaxGenerated int  `json:"max_generated"`
	MaxEncoded   int  `json:"max_encoded"`
	Constrained  bool `json:"constrained"`
}

// DefaultGenerationOptions returns the pinned runtime's values.
func DefaultGenerationOptions() GenerationOptions { return GenerationOptions{512, 1024, true} }

func encoderInput(t *Tokenizer, query, tools string, maxLength int) ([]int, error) {
	q, err := t.Encode(query)
	if err != nil {
		return nil, err
	}
	tool, err := t.Encode(tools)
	if err != nil {
		return nil, err
	}
	q = q[:min(len(q), max(0, maxLength-2))]
	tool = tool[:min(len(tool), max(0, maxLength-len(q)-1))]
	return append(append(q, TokenTools), tool...), nil
}

// Generate runs scalar Needle inference with deterministic greedy decoding and
// the source tool/argument trie constraints. No external inference runtime.
func (r *Router) Generate(t *Tokenizer, query, tools string, options GenerationOptions, cp Checkpoint) (string, error) {
	if options.MaxEncoded < 0 || options.MaxGenerated < 0 {
		return "", invalid("generation lengths must be nonnegative")
	}
	rawTools := tools
	key := struct {
		tools     string
		tokenizer *Tokenizer
	}{rawTools, t}
	cached, ok := r.constraintTemplates.Load(key)
	if !ok {
		normalized, names := NormalizeToolsJSON(rawTools)
		cached, _ = r.constraintTemplates.LoadOrStore(key, &constraintCache{
			normalized: normalized,
			names:      names,
			template:   newConstraintTemplate(normalized, t),
		})
	}
	entry := cached.(*constraintCache)
	tools = entry.normalized
	tokens, err := encoderInput(t, query, tools, options.MaxEncoded)
	if err != nil {
		return "", err
	}
	if err = poll(cp, "tokenized"); err != nil {
		return "", err
	}
	encoded, err := r.encode(tokens, cp)
	if err != nil {
		return "", err
	}
	if err = poll(cp, "encoded"); err != nil {
		return "", err
	}
	decoder := &constraints{template: entry.template}
	state, err := r.decoderStateFor(encoded, options.MaxGenerated, cp)
	if err != nil {
		return "", err
	}
	token := TokenEOS
	generated := make([]int, 0, min(options.MaxGenerated, int(r.config.MaxSequence)))
	allowed := make([]int, 0, 64)
	for i := 0; i < options.MaxGenerated; i++ {
		hidden, err := r.decodeStep(token, len(generated), state, cp)
		if err != nil {
			return "", err
		}
		allowed = allowed[:0]
		if options.Constrained {
			allowed = decoder.allowedInto(allowed)
		}
		next := argmaxWithEngine(hidden, r.embedding, allowed, r.simd)
		if options.Constrained {
			decoder.update(next)
		}
		if next == TokenEOS {
			break
		}
		generated = append(generated, next)
		token = next
	}
	if len(generated) == options.MaxGenerated {
		return "", fmt.Errorf("generation exceeded max length %d", options.MaxGenerated)
	}
	text, err := t.Decode(generated)
	if err != nil {
		return "", err
	}
	text = strings.TrimPrefix(text, "<tool_call>")
	return RestoreToolNames(text, entry.names), nil
}

// SnakeCase intentionally preserves the reference's uppercase-run behaviour.
func SnakeCase(name string) string {
	var out []rune
	lower, upper := false, false
	for _, ch := range name {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' {
			if ch >= 'A' && ch <= 'Z' {
				if len(out) > 0 && (lower || upper && out[len(out)-1] >= 'a' && out[len(out)-1] <= 'z') {
					out = append(out, '_')
				}
				out = append(out, ch+32)
				lower = false
				upper = true
			} else {
				out = append(out, ch)
				lower = ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9'
				upper = false
			}
		} else if len(out) == 0 || out[len(out)-1] != '_' {
			out = append(out, '_')
			lower = false
			upper = false
		}
	}
	return strings.Join(strings.FieldsFunc(string(out), func(r rune) bool { return r == '_' }), "_")
}
func parseJSON(text string) (any, bool) {
	d := json.NewDecoder(strings.NewReader(text))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return nil, false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, false
	}
	return v, true
}
func compactJSON(value any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(value)
	return strings.TrimSuffix(b.String(), "\n")
}

// NormalizeToolsJSON uses the original deterministic JSON object ordering and
// records last-wins collisions so generated function names can be restored.
func NormalizeToolsJSON(text string) (string, map[string]string) {
	names := map[string]string{}
	value, ok := parseJSON(text)
	if !ok {
		return text, names
	}
	if items, ok := value.([]any); ok {
		for _, item := range items {
			if object, ok := item.(map[string]any); ok {
				if name, ok := object["name"].(string); ok {
					snake := SnakeCase(name)
					names[snake] = name
					object["name"] = snake
				}
			}
		}
	}
	return compactJSON(value), names
}

// RestoreToolNames reverses renamed top-level JSON calls, with the source's
// longest-key-first text fallback when generated output is not valid JSON.
func RestoreToolNames(text string, names map[string]string) string {
	if len(names) == 0 {
		return text
	}
	value, ok := parseJSON(text)
	if ok {
		restore := func(v any) {
			if object, ok := v.(map[string]any); ok {
				if name, ok := object["name"].(string); ok {
					if original, found := names[name]; found {
						object["name"] = original
					}
				}
			}
		}
		if items, ok := value.([]any); ok {
			for _, item := range items {
				restore(item)
			}
		} else {
			restore(value)
		}
		return compactJSON(value)
	}
	keys := make([]string, 0, len(names))
	for key := range names {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})
	for _, key := range keys {
		text = strings.ReplaceAll(text, key, names[key])
	}
	return text
}
