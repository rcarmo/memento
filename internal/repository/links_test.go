package repository

import (
	"encoding/json"
	"github.com/yuin/goldmark/ast"
	"os"
	"reflect"
	"testing"
)

func TestPythonStructuralLinks(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/repository-links.json")
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
	for _, tc := range []struct {
		source, href, want string
		internal           bool
	}{
		{"/folder/source.md", "target.md", "/folder/target.md", true},
		{"/folder/source.md", "../target.md#part", "/target.md", true},
		{"/folder/source.md", "#part", "/folder/source.md", true},
		{"/folder/source.md", "/target.md", "/target.md", true},
		{"/folder/source.md", "https://example.org", "", false},
		{"/folder/source.md", "//example.org", "", false},
	} {
		got, internal := ResolveLinkPath(tc.source, tc.href)
		if got != tc.want || internal != tc.internal {
			t.Fatal(tc, got, internal)
		}
	}
	for _, tc := range []struct {
		href, want string
		valid      bool
	}{
		{"references/guide.md#part", "references/guide.md", true},
		{"./scripts/run.sh", "scripts/run.sh", true},
		{"../guide.md", "", false},
		{"/guide.md", "", false},
		{"#part", "", false},
		{"https://example.org", "", false},
	} {
		got, valid := ResolveAssetLinkPath(tc.href)
		if got != tc.want || valid != tc.valid {
			t.Fatal(tc, got, valid)
		}
	}
	for _, tc := range []struct {
		content, source, oldPath, newPath, want string
	}{
		{"[same](target.md#part)", "/folder/source.md", "/folder/target.md", "/folder/renamed.md", "[same](renamed.md#part)"},
		{"[cross](target.md)", "/folder/source.md", "/folder/target.md", "/other/renamed.md", "[cross](../other/renamed.md)"},
		{"[self](#part)", "/folder/source.md", "/folder/source.md", "/other/source.md", "[self](#part)"},
		{"[angle](<target.md#space here>)", "/folder/source.md", "/folder/target.md", "/other/renamed.md", "[angle](<../other/renamed.md#space here>)"},
	} {
		got := RewriteLinksForRenameFrom(tc.content, tc.source, tc.oldPath, tc.newPath)
		if !got.Changed && got.Content != tc.content || got.Content != tc.want {
			t.Fatal(tc, got)
		}
	}
	rebased := RebaseRelativeLinks("[same](target.md) [up](../root.md#x) [absolute](/fixed.md)", "/folder/source.md", "/other/source.md")
	if !rebased.Changed || rebased.Content != "[same](../folder/target.md) [up](../root.md#x) [absolute](/fixed.md)" {
		t.Fatal(rebased)
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
