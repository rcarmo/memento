package repository

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestFrontmatterBoundariesAndIO(t *testing.T) {
	for _, text := range []string{"\xff", "{\n}\nx\n}\n"} {
		_, _ = ParseConceptText(text)
	}
	if _, err := ParseConceptFile(filepath.Join(t.TempDir(), "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "concept.md")
	if err := os.WriteFile(path, []byte("---\rid: x\r---\rbody"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseConceptFile(path); errorText(err) != "frontmatter validation failed" {
		t.Fatal(err)
	}
	// A JSON handler can close with a line containing either brace. Additional
	// closing syntax inside metadata must produce a parser failure.
	if _, err := ParseConceptText("{\n\"x\":1} {}\n}\nbody"); errorText(err) != "invalid frontmatter" {
		t.Fatal(err)
	}
	_, _, _, _ = splitConceptText("{\nnot closed")
	if _, err := parseYAMLMetadata("a: 1\n---\nb: 2"); err == nil {
		t.Fatal("multiple docs")
	}
	if _, err := yamlValue(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "x"}, map[*yaml.Node]bool{}, 101); err == nil {
		t.Fatal("depth")
	}
	if _, err := yamlValue(&yaml.Node{}, map[*yaml.Node]bool{}, 0); err == nil {
		t.Fatal("unknown node")
	}
	for _, node := range []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!int", Value: "bad"},
		{Kind: yaml.ScalarNode, Tag: "!!float", Value: "bad", Style: yaml.TaggedStyle},
		{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: "bad", Style: yaml.TaggedStyle},
		{Kind: yaml.ScalarNode, Tag: "!!binary", Value: "a", Style: yaml.TaggedStyle},
		{Kind: yaml.MappingNode, Tag: "!!set", Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!bad", Value: "x"}, {Kind: yaml.ScalarNode, Tag: "!!null"}}},
		{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!merge", Value: "<<"}, {Kind: yaml.ScalarNode, Tag: "!bad", Value: "x"}}},
	} {
		_, _ = yamlValue(node, map[*yaml.Node]bool{}, 0)
	}
	for _, text := range []string{"[!!timestamp bad]", "<<: [!bad scalar]"} {
		if _, err := parseYAMLMetadata(text); err == nil {
			t.Fatal(text)
		}
	}
}

func TestYAMLQuotedControls(t *testing.T) {
	for _, text := range []string{"a: 'quoted'", "a\x01b", "a\ufeffb", "x\uffff", "\a\b\v\f\r\t\x1b\\\"\u00a0\u0085\u2028\u2029"} {
		out := yamlString(text, 2)
		if out == text {
			t.Fatal("not quoted", text)
		}
	}
	if got := yamlDoubleQuoted("\x01\ufeff"); got != `"\x01\uFEFF"` {
		t.Fatal(got)
	}
}

func FuzzConceptFrontmatter(f *testing.F) {
	for _, text := range []string{"", "---\nid: x\n---", "---\na: &x [*x]\n---", "{\n}\nbody", "\xff"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 65536 {
			return
		}
		document, err := ParseConceptText(text)
		if err == nil {
			_, _ = SerializeConcept(document)
		}
		_ = yamlString(strings.ToValidUTF8(text, "�"), 2)
	})
}

func TestYAMLAliasExpansionLimit(t *testing.T) {
	remaining := 0
	if _, err := yamlValueBounded(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "x"}, map[*yaml.Node]bool{}, 0, &remaining); err == nil {
		t.Fatal("budget ignored")
	}
}
