package needle

import (
	"reflect"
	"strings"
	"testing"
)

func TestToolNameNormalization(t *testing.T) {
	for _, c := range [][2]string{{"HTTPServer", "h_t_t_p_server"}, {"getURL", "get_u_r_l"}, {"already_snake", "already_snake"}, {" a--b__ ", "a_b"}, {"中文", ""}, {"a2B", "a2_b"}} {
		if got := SnakeCase(c[0]); got != c[1] {
			t.Error(c, got)
		}
	}
	raw := `[{"parameters":{"q":{}},"name":"GetNotes"},{"name":"get_notes"},{"name":1},null,3]`
	normalized, names := NormalizeToolsJSON(raw)
	if !strings.Contains(normalized, `"name":"get_notes"`) || names["get_notes"] != "get_notes" {
		t.Fatal(normalized, names)
	}
	invalid, names := NormalizeToolsJSON("not json")
	if invalid != "not json" || len(names) != 0 {
		t.Fatal(invalid, names)
	}
	object, names := NormalizeToolsJSON(` {"x":1} `)
	if object != `{"x":1}` || len(names) != 0 {
		t.Fatal(object, names)
	}
	nameMap := map[string]string{"get_notes": "GetNotes", "get": "Get"}
	for _, c := range [][2]string{{`[{"name":"get_notes"},{"name":"absent"},3]`, `[{"name":"GetNotes"},{"name":"absent"},3]`}, {`{"name":"get_notes","x":1}`, `{"name":"GetNotes","x":1}`}, {`{"name":1}`, `{"name":1}`}, {`bad get_notes get`, `bad GetNotes Get`}, {`null`, `null`}} {
		if got := RestoreToolNames(c[0], nameMap); got != c[1] {
			t.Fatal(c, got)
		}
	}
	if got := RestoreToolNames(" unchanged ", nil); got != " unchanged " {
		t.Fatal(got)
	}
	if _, ok := parseJSON(`{} {}`); ok {
		t.Fatal("extra JSON accepted")
	}
	_ = RestoreToolNames("foo bar", map[string]string{"foo": "Foo", "bar": "Bar"})
}

func TestConstraintTrieAndMachine(t *testing.T) {
	tree := trie{}
	tree.insert("alpha")
	tree.insert("beta")
	if tree.node("missing") != nil || tree.node("al") == nil {
		t.Fatal("prefix")
	}
	if !tokenValid("alpha\"suffix", &tree) || tokenValid("al\"", &tree) || tokenValid("no", &tree) || !tokenValid("al", &tree) {
		t.Fatal("token validation")
	}
	tok, err := TokenizerFromBytes(syntheticTokenizer(true))
	if err != nil {
		t.Fatal(err)
	}
	c := newConstraints(`[{"name":"a","parameters":{"a":{},"ignored":"string"}},{"name":4},null]`, tok)
	if c.allowed() != nil {
		t.Fatal("free state constrained")
	}
	c.machine.feed(`{"name":"`)
	if c.machine.state != inName {
		t.Fatal(c.machine)
	}
	allowed := c.allowed()
	if len(allowed) == 0 {
		t.Fatal("no prefix candidates")
	}
	c.update(-1)
	c.update(999)
	c.update(6)
	c.machine.feed(`","arguments":{"`)
	if c.machine.state != inArgKey || c.machine.currentFunction != "a" {
		t.Fatal(c.machine)
	}
	if !reflect.DeepEqual(c.allowed(), []int{6}) {
		t.Fatal(c.allowed())
	}
	c.machine.feed(`a":"value\\\"still","b":{},"c":[]}}`)
	if c.machine.inArguments {
		t.Fatal("arguments never closed")
	}
	c.machine = stateMachine{state: inArgKey, currentFunction: "missing"}
	if c.allowed() != nil {
		t.Fatal("unknown params trie")
	}
	c.machine = stateMachine{state: inName, constrained: "missing"}
	if c.allowed() != nil {
		t.Fatal("unknown prefix")
	}
	c.machine = stateMachine{state: inName, constrained: "a"}
	c.template.strings = append(c.template.strings, `"done`)
	if ids := c.allowed(); len(ids) != 1 || ids[0] != len(c.template.strings)-1 {
		t.Fatal(ids)
	}
	for _, raw := range []string{"bad", `{}`, `[{"name":"x"}]`} {
		_ = newConstraints(raw, tok)
	}
	machine := stateMachine{}
	machine.feed(`]}`)
	machine.feed(`{"arguments":{"x":[{"nested":"text"}]},"name":"a"}`)
	machine = stateMachine{}
	machine.feed(`{"value": "a\\b\"c"}`)
	if machine.inString {
		t.Fatal(machine)
	}
}
