package umcp

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPythonToolResultParity(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-tool-results.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Value    string `json:"value_json"`
			Schema   string `json:"schema_json"`
			Expected json.RawMessage
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Cases {
		value, err := ParseValue([]byte(c.Value))
		if err != nil {
			t.Fatal(err)
		}
		schema, err := ParseSchema([]byte(c.Schema))
		if err != nil {
			t.Fatal(err)
		}
		result, err := FormatToolResult(value, schema)
		var message any
		if err != nil {
			message = err.Error()
		}
		actual, e := json.Marshal(map[string]any{"result": result, "error": message})
		if e != nil {
			t.Fatal(e)
		}
		got, _ := decode(actual)
		want, _ := decode(c.Expected)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("value %s schema %s\ngot %s\nwant %s", c.Value, c.Schema, actual, c.Expected)
		}
	}
}

func TestOrderedJSONDuplicatesAndErrors(t *testing.T) {
	value, err := ParseValue([]byte(`{"z":1,"a":2,"z":3}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil || string(encoded) != `{"z":3,"a":2}` {
		t.Fatal(string(encoded), err)
	}
	for _, text := range []string{"", "{", `{"a":`, `[`, `[1,`, `{"a":1`, `[1`, `{} {}`, `{} trailing`} {
		if _, err = ParseValue([]byte(text)); err == nil {
			t.Fatal(text)
		}
	}
	if _, err = FormatToolResult(make(chan int), nil); err == nil {
		t.Fatal("native object accepted")
	}
	if _, err = json.Marshal(OrderedObject{{"bad", make(chan int)}}); err == nil {
		t.Fatal("invalid member accepted")
	}
	if _, err = pythonJSON([]any{make(chan int)}); err == nil {
		t.Fatal("invalid array accepted")
	}
	if _, err = pythonJSON(OrderedObject{{"bad", make(chan int)}}); err == nil {
		t.Fatal("invalid dict accepted")
	}
	if _, err = pythonJSON(json.Number("bad")); err == nil {
		t.Fatal("bad numeric accepted")
	}
}

func FuzzToolResults(f *testing.F) {
	for _, s := range []string{"null", `{"b":1,"a":2}`, `[1,true]`, `"hello"`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		value, err := ParseValue([]byte(s))
		if err != nil {
			return
		}
		_, _ = FormatToolResult(value, nil)
	})
}

func TestStructuredValueEdges(t *testing.T) {
	if _, err := ParseValue([]byte(`{"bad`)); err == nil {
		t.Fatal("partial key accepted")
	}
	decoder := json.NewDecoder(strings.NewReader(`[]`))
	_, _ = decoder.Token()
	if _, err := parseValue(decoder); err == nil {
		t.Fatal("closing delimiter as value")
	}
	for _, c := range []struct {
		V    float64
		Want string
	}{{math.NaN(), "NaN"}, {math.Inf(1), "Infinity"}, {math.Inf(-1), "-Infinity"}} {
		if pythonFloat(c.V) != c.Want {
			t.Fatal(c)
		}
	}
	for _, s := range []string{"bad", "1e999"} {
		if _, ok := numberRat(json.Number(s)); ok {
			t.Fatal(s)
		}
	}
	for _, c := range [][2]any{{[]any{}, []any{nil}}, {[]any{"a"}, []any{"b"}}, {OrderedObject{}, OrderedObject{{"a", nil}}}, {OrderedObject{{"a", "x"}}, OrderedObject{{"b", "x"}}}, {OrderedObject{{"a", "x"}}, OrderedObject{{"a", "y"}}}, {make(chan int), nil}} {
		if pythonEqual(c[0], c[1]) {
			t.Fatal("unequal compared equal")
		}
	}
	if pythonValueRepr(false) != "False" || pythonValueRepr(make(chan int)) != "<unsupported>" || pythonValueRepr(json.Number("bad")) != "<unsupported>" {
		t.Fatal("repr")
	}
	if _, err := ParseSchema([]byte("{")); err == nil {
		t.Fatal("bad schema")
	}
	if _, err := ParseSchema([]byte("[]")); err == nil {
		t.Fatal("array schema")
	}
	input := make(chan int)
	if orderedSchemaValue(input) != input {
		t.Fatal("unsupported schema conversion")
	}
	for _, schema := range []map[string]any{
		{"oneOf": []any{true}}, {"type": []any{}}, {"required": []any{true}},
		{"properties": map[string]any{"a": true}},
		{"properties": map[string]any{"a": map[string]any{"type": "integer"}}},
	} {
		if err := ValidateSchemaSubset(OrderedObject{{"a", "bad"}}, schema, "$"); err == nil {
			t.Fatal(schema)
		}
	}
}
