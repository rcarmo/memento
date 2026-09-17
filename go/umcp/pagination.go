package umcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf16"
)

// DiscoveryItem separates the wire object from its ordered identity fields.
// Discovery builders supply validated string identities (name or URI), then
// paginate in their already-sorted source order.
type DiscoveryItem struct {
	Identity []string
	Value    any
}

// ListPage mirrors uMCP list pagination. params use the JSON decoder's Number
// type. Cursors encode principal, label and visible identities; they are not
// signatures and must never be used in place of authorisation.
func ListPage(ctx context.Context, label, resultKey string, items []DiscoveryItem, params map[string]any, defaultSize int) (map[string]any, *RPCError) {
	invalid := func(message string) (map[string]any, *RPCError) {
		return nil, &RPCError{Code: -32602, Message: message}
	}
	var size *big.Int
	if value := params["pageSize"]; value != nil {
		n, ok := value.(json.Number)
		if !ok || !integer.MatchString(n.String()) {
			return invalid("Invalid params: 'pageSize' must be a positive integer")
		}
		size, _ = new(big.Int).SetString(n.String(), 10)
		if size.Sign() <= 0 {
			return invalid("Invalid params: 'pageSize' must be a positive integer")
		}
	}
	cursorValue := params["cursor"]
	cursor, hasCursor := cursorValue.(string)
	if cursorValue != nil && !hasCursor {
		return invalid("Invalid params: 'cursor' must be a string")
	}
	start := 0
	end := len(items)
	fingerprint := ""
	if hasCursor || size != nil {
		if size == nil {
			if defaultSize == 0 {
				defaultSize = 50
			}
			size = big.NewInt(int64(defaultSize))
		}
		basis := []string{Context(ctx).Principal, label}
		for _, item := range items {
			basis = append(basis, strings.Join(item.Identity, "\x1f"))
		}
		fingerprint = base64.RawURLEncoding.EncodeToString([]byte(strings.Join(basis, "\n")))
		if hasCursor {
			offset, ok := decodeCursor(cursor, label, fingerprint)
			if !ok {
				return invalid("Invalid cursor")
			}
			if offset.Cmp(big.NewInt(int64(len(items)))) >= 0 {
				start = len(items)
			} else {
				start = int(offset.Int64())
			}
		}
		remaining := len(items) - start
		if size.Sign() < 0 {
			// Preserve Python slicing for a subclass's negative default size.
			bound := new(big.Int).Add(big.NewInt(int64(start)), size)
			if bound.Sign() < 0 {
				bound.Add(bound, big.NewInt(int64(len(items))))
			}
			if bound.Sign() < 0 {
				end = 0
			} else if bound.Cmp(big.NewInt(int64(len(items)))) < 0 {
				end = int(bound.Int64())
			}
			if end < start {
				end = start
			}
		} else if size.Cmp(big.NewInt(int64(remaining))) < 0 {
			end = start + int(size.Int64())
		}
	}
	values := make([]any, 0, end-start)
	for _, item := range items[start:end] {
		values = append(values, item.Value)
	}
	result := map[string]any{resultKey: values}
	if (hasCursor || size != nil) && end < len(items) {
		payload := fmt.Sprintf(`{"v":1,"l":%s,"o":%d,"f":%s}`, quoteASCII(label), end, quoteASCII(fingerprint))
		result["nextCursor"] = base64.RawURLEncoding.EncodeToString([]byte(payload))
	}
	return result, nil
}

func decodeCursor(cursor, label, fingerprint string) (*big.Int, bool) {
	// Python pads before permissively filtering non-base64 alphabet characters.
	padded := cursor + strings.Repeat("=", (4-len([]rune(cursor))%4)%4)
	var b strings.Builder
	for _, r := range padded {
		if r > 127 {
			return nil, false
		}
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '=' || r == '+' || r == '/' || r == '-' || r == '_' {
			switch r {
			case '-':
				r = '+'
			case '_':
				r = '/'
			}
			b.WriteRune(r)
		}
	}
	decoded, valid := decodePythonBase64(b.String())
	if !valid {
		return nil, false
	}
	value, ok := decode(decoded)
	if !ok {
		return nil, false
	}
	payload, ok := value.(map[string]any)
	if !ok || payload["l"] != label || payload["f"] != fingerprint {
		return nil, false
	}
	version := payload["v"]
	// Python equality accepts True and 1.0 for v; o accepts integers/bools only.
	if version != true {
		n, ok := version.(json.Number)
		if !ok {
			return nil, false
		}
		r, ok := new(big.Rat).SetString(n.String())
		if !ok || r.Cmp(big.NewRat(1, 1)) != 0 {
			return nil, false
		}
	}
	if boolean, ok := payload["o"].(bool); ok {
		if boolean {
			return big.NewInt(1), true
		}
		return big.NewInt(0), true
	}
	n, ok := payload["o"].(json.Number)
	if !ok || !integer.MatchString(n.String()) {
		return nil, false
	}
	offset, _ := new(big.Int).SetString(n.String(), 10)
	return offset, offset.Sign() >= 0
}

// decodePythonBase64 reproduces binascii's non-strict padding behaviour: extra
// trailing padding and leading/incomplete padding are ignored, but incomplete
// data quanta at EOF still fail. Input has already been alphabet-filtered.
func decodePythonBase64(value string) ([]byte, bool) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	out := make([]byte, 0, len(value)*3/4)
	position, padding := 0, 0
	var bits uint32
	for _, r := range value {
		if r == '=' {
			padding++
			if position >= 2 && position+padding >= 4 {
				return out, true
			}
			continue
		}
		padding = 0
		n := strings.IndexRune(alphabet, r)
		bits = bits<<6 | uint32(n)
		position++
		switch position {
		case 2:
			out = append(out, byte(bits>>4))
		case 3:
			out = append(out, byte(bits>>2))
		case 4:
			out = append(out, byte(bits))
			position = 0
			bits = 0
		}
	}
	return out, position == 0
}

// Python json.dumps ensure_ascii=True is required for exact opaque cursor bytes.
func quoteASCII(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
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
					a, z := utf16.EncodeRune(r)
					fmt.Fprintf(&b, `\u%04x\u%04x`, a, z)
				} else {
					fmt.Fprintf(&b, `\u%04x`, r)
				}
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
