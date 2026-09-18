// Package pyjson provides the Python json.dumps(sort_keys=True) subset used
// by durable control records. JSON bytes are part of request/patch hashes.
package pyjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func Parse(raw string) (any, error) {
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	return value, nil
}
func Dumps(value any) (string, error) { return dumps(value, false) }

// DumpsCompact matches json.dumps(sort_keys=True, separators=(",", ":")).
func DumpsCompact(value any) (string, error) { return dumps(value, true) }

func dumps(value any, compact bool) (string, error) {
	var b bytes.Buffer
	if err := encode(&b, value, 0, compact); err != nil {
		return "", err
	}
	return b.String(), nil
}

// DumpsIndent retains Python's ASCII escaping and sorted keys while using
// two-space JSON layout. Empty objects/lists remain on one line.
func DumpsIndent(value any) (string, error) {
	raw, err := Dumps(value)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err = json.Indent(&out, []byte(raw), "", "  "); err != nil {
		return "", err
	}
	return out.String(), nil
}
func encode(b *bytes.Buffer, value any, depth int, compact bool) error {
	if depth > 100 {
		return fmt.Errorf("JSON nesting limit")
	}
	switch v := value.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		if !utf8.ValidString(v) {
			return fmt.Errorf("invalid UTF-8 JSON string")
		}
		quote(b, v)
	case json.Number:
		if !strings.ContainsAny(string(v), ".eE") {
			n, ok := new(big.Int).SetString(string(v), 10)
			if !ok {
				return fmt.Errorf("invalid JSON integer")
			}
			b.WriteString(n.String())
		} else {
			n, err := strconv.ParseFloat(string(v), 64)
			if err != nil {
				return err
			}
			b.WriteString(floatString(n))
		}
	case float64:
		b.WriteString(floatString(v))
	case int:
		b.WriteString(strconv.Itoa(v))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case []string:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = item
		}
		return encode(b, items, depth, compact)
	case []any:
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				if compact {
					b.WriteByte(',')
				} else {
					b.WriteString(", ")
				}
			}
			if err := encode(b, item, depth+1, compact); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				if compact {
					b.WriteByte(',')
				} else {
					b.WriteString(", ")
				}
			}
			if !utf8.ValidString(k) {
				return fmt.Errorf("invalid UTF-8 JSON key")
			}
			quote(b, k)
			if compact {
				b.WriteByte(':')
			} else {
				b.WriteString(": ")
			}
			if err := encode(b, v[k], depth+1, compact); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("unsupported JSON value: %T", value)
	}
	return nil
}
func quote(b *bytes.Buffer, text string) {
	b.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 32 || r >= 127 {
				if r > 0xffff {
					a, c := utf16.EncodeRune(r)
					fmt.Fprintf(b, `\u%04x\u%04x`, a, c)
				} else {
					fmt.Fprintf(b, `\u%04x`, r)
				}
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
func floatString(value float64) string {
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 1) {
		return "Infinity"
	}
	if math.IsInf(value, -1) {
		return "-Infinity"
	}
	scientific := strconv.FormatFloat(value, 'e', -1, 64)
	parts := strings.Split(scientific, "e")
	exponent, _ := strconv.Atoi(parts[1])
	if exponent < -4 || exponent >= 16 {
		return scientific
	}
	result := strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.Contains(result, ".") {
		result += ".0"
	}
	return result
}
