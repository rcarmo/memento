package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/rcarmo/memento/go/internal/pyjson"
	"github.com/rcarmo/memento/go/umcp"
)

func encodeAuditCursor(key [3]string, revision string, filters map[string]any) (string, error) {
	raw, err := pyjson.Dumps(map[string]any{"v": 1, "revision": revision, "filters": filters, "after": []string{key[0], key[1], key[2]}})
	if err != nil {
		return "", err
	}
	var compact bytes.Buffer
	_ = json.Compact(&compact, []byte(raw))
	return base64.RawURLEncoding.EncodeToString(compact.Bytes()), nil
}
func decodeAuditCursor(cursor *string, revision string, filters map[string]any) (*[3]string, error) {
	if cursor == nil {
		return nil, nil
	}
	failure := func() (*[3]string, error) { return nil, &Error{"validation_error", "invalid or stale audit cursor"} }
	// Python's b64decode discards non-alphabet bytes and accepts the standard
	// alphabet too. Padding is added before decoding from the original length.
	text := *cursor + strings.Repeat("=", (4-len([]rune(*cursor))%4)%4)
	clean := strings.Builder{}
	for _, r := range text {
		if r > 127 {
			return failure()
		}
		switch {
		case r == '-':
			clean.WriteByte('+')
		case r == '_':
			clean.WriteByte('/')
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '/' || r == '=':
			clean.WriteByte(byte(r))
		}
	}
	raw, valid := umcp.DecodePythonBase64(clean.String())
	if !valid {
		return failure()
	}
	value, err := pyjson.Parse(string(raw))
	if err != nil {
		return failure()
	}
	object, ok := value.(map[string]any)
	if !ok {
		return failure()
	}
	versionOK := object["v"] == true
	if n, ok := object["v"].(json.Number); ok {
		// json.loads uses a binary float for decimal/exponent forms. A
		// bounded float conversion also avoids allocating giant rationals for
		// attacker-controlled exponents in an otherwise tiny cursor.
		number, err := n.Float64()
		versionOK = err == nil && number == 1
	} // Python numeric equality
	if !versionOK || object["revision"] != revision || !reflect.DeepEqual(object["filters"], filters) {
		return failure()
	}
	values, ok := object["after"].([]any)
	if !ok || len(values) != 3 {
		return failure()
	}
	key := [3]string{}
	for i, value := range values {
		text, ok := value.(string)
		if !ok {
			return failure()
		}
		key[i] = text
	}
	return &key, nil
}
