package repository

import (
	"encoding/json"
	"github.com/yuin/goldmark/ast"
	"os"
	"reflect"
	"testing"
)

func TestPythonStructuralLinks(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/repository-links.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Cases []struct {
			Input  string
			Links  []MarkdownLink
			Rename RenameRewriteResult
		}
		Rewrites []struct {
			Input, Old, New string
			Expected        RenameRewriteResult
		}
		External []struct {
			Input    string
			Expected bool
		}
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixtures.Cases {
		links := ExtractStructuralLinks(c.Input)
		if !reflect.DeepEqual(links, c.Links) {
			t.Errorf("case %d links %+v != %+v input %q", i, links, c.Links, c.Input)
		}
		renamed := RewriteLinksForRename(c.Input, "/old.md", "/new.md")
		if renamed != c.Rename {
			t.Errorf("case %d rename %+v != %+v", i, renamed, c.Rename)
		}
	}
	for _, c := range fixtures.Rewrites {
		if got := RewriteLinksForRename(c.Input, c.Old, c.New); got != c.Expected {
			t.Fatal(c, got)
		}
	}
	for _, c := range fixtures.External {
		if got := IsExternalLink(c.Input); got != c.Expected {
			t.Fatal(c, got)
		}
	}
}
func FuzzMarkdownLinks(f *testing.F) {
	for _, input := range []string{"", "[x](/old.md)", "![x](/old.md)", "[x](javascript:bad)", "[x][a]\n\n[a]: /old.md"} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 8192 {
			return
		}
		_ = ExtractStructuralLinks(input)
		_ = RewriteLinksForRename(input, "/old.md", "/new.md")
	})
}

func TestMarkdownLinkHelpers(t *testing.T) {
	if got := RewriteLinksForRename("content", "", "new"); got.Changed || got.Content != "content" {
		t.Fatal(got)
	}
	node := ast.NewLink()
	node.AppendChild(node, ast.NewString([]byte("a &amp; b")))
	if got := linkText(node, nil); got != "a & b" {
		t.Fatal(got)
	}
	if linkLine(node, nil) != 0 {
		t.Fatal("detached node line")
	}
	if got := recodeLinkHost("https://xn--99999999/a", true); got != "https://xn--99999999/a" {
		t.Fatal(got)
	}
}
