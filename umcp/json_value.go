package umcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Member is one insertion-ordered JSON object entry. OrderedObject avoids Go map
// sorting when Python's json.dumps text appears inside a tool result.
type Member struct {
	Name  string
	Value any
}

// OrderedObject is the JSON/Python-dict subset used by result formatting.
type OrderedObject []Member

// ParseValue preserves dict insertion order, last-value-wins duplicates and
// arbitrary-precision integers. It intentionally accepts only valid JSON input.
func ParseValue(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := parseValue(d)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return v, nil
}
func parseValue(d *json.Decoder) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		object := OrderedObject{}
		positions := map[string]int{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, err
			}
			value, err := parseValue(d)
			if err != nil {
				return nil, err
			}
			name := key.(string)
			if i, ok := positions[name]; ok {
				object[i].Value = value
			} else {
				positions[name] = len(object)
				object = append(object, Member{name, value})
			}
		}
		if _, err = d.Token(); err != nil {
			return nil, err
		}
		return object, nil
	case '[':
		list := []any{}
		for d.More() {
			v, err := parseValue(d)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		if _, err = d.Token(); err != nil {
			return nil, err
		}
		return list, nil
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

// MarshalJSON retains insertion order for nested structuredContent objects.
func (o OrderedObject) MarshalJSON() ([]byte, error) { return marshalObject(o, false) }
func marshalObject(o OrderedObject, spaced bool) ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	separator := ","
	colon := ":"
	if spaced {
		separator = ", "
		colon = ": "
	}
	for i, m := range o {
		if i > 0 {
			b.WriteString(separator)
		}
		name, _ := json.Marshal(m.Name)
		b.Write(name)
		b.WriteString(colon)
		var value []byte
		var err error
		if spaced {
			value, err = pythonJSON(m.Value)
		} else {
			value, err = json.Marshal(m.Value)
		}
		if err != nil {
			return nil, err
		}
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
func jsonString(s string) []byte {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(s)
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'})
}
func pythonJSON(value any) ([]byte, error) {
	switch v := value.(type) {
	case OrderedObject:
		return marshalObject(v, true)
	case []any:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			part, err := pythonJSON(item)
			if err != nil {
				return nil, err
			}
			b.Write(part)
		}
		b.WriteByte(']')
		return b.Bytes(), nil
	case string:
		return jsonString(v), nil
	case nil:
		return []byte("null"), nil
	case bool:
		if v {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case json.Number:
		if integer.MatchString(v.String()) {
			n, _ := new(big.Int).SetString(v.String(), 10)
			return []byte(n.String()), nil
		}
		number, err := strconv.ParseFloat(v.String(), 64)
		if err != nil && !math.IsInf(number, 0) {
			return nil, err
		}
		return []byte(pythonFloat(number)), nil
	default:
		return nil, fmt.Errorf("unsupported structured value %T", value)
	}
}
func pythonFloat(value float64) string {
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
	_, exponent, _ := strings.Cut(scientific, "e")
	n, _ := strconv.Atoi(exponent)
	if n < -4 || n >= 16 {
		return scientific
	}
	fixed := strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.Contains(fixed, ".") {
		fixed += ".0"
	}
	return fixed
}

func numberRat(value any) (*big.Rat, bool) {
	switch v := value.(type) {
	case bool:
		if v {
			return big.NewRat(1, 1), true
		}
		return big.NewRat(0, 1), true
	case json.Number:
		if integer.MatchString(v.String()) {
			n, _ := new(big.Int).SetString(v.String(), 10)
			return new(big.Rat).SetInt(n), true
		}
		f, err := strconv.ParseFloat(v.String(), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, false
		}
		return new(big.Rat).SetFloat64(f), true
	default:
		return nil, false
	}
}
func pythonEqual(a, b any) bool {
	if x, ok := numberRat(a); ok {
		if y, ok := numberRat(b); ok {
			return x.Cmp(y) == 0
		}
		return false
	}
	switch x := a.(type) {
	case nil:
		return b == nil
	case string:
		y, ok := b.(string)
		return ok && x == y
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !pythonEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	case OrderedObject:
		y, ok := b.(OrderedObject)
		if !ok || len(x) != len(y) {
			return false
		}
		for _, m := range x {
			v, ok := lookup(y, m.Name)
			if !ok || !pythonEqual(m.Value, v) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
func lookup(o OrderedObject, key string) (any, bool) {
	for _, member := range o {
		if member.Name == key {
			return member.Value, true
		}
	}
	return nil, false
}
func pythonValueRepr(value any) string {
	switch v := value.(type) {
	case nil:
		return "None"
	case bool:
		if v {
			return "True"
		}
		return "False"
	case string:
		return pythonRepr(v)
	case []any:
		out := make([]string, len(v))
		for i, x := range v {
			out[i] = pythonValueRepr(x)
		}
		return "[" + strings.Join(out, ", ") + "]"
	case OrderedObject:
		out := make([]string, len(v))
		for i, m := range v {
			out[i] = pythonRepr(m.Name) + ": " + pythonValueRepr(m.Value)
		}
		return "{" + strings.Join(out, ", ") + "}"
	case json.Number:
		b, err := pythonJSON(v)
		if err == nil {
			return string(b)
		}
	}
	return "<unsupported>"
}
