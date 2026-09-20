package derived

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rcarmo/memento/internal/access"
)

func TestSearchReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/derived-search.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Files   map[string]string
		Queries []struct{ Query, Syntax, Expected, Error string }
		Pages   []struct {
			Policy  access.EffectivePolicy
			Options struct {
				Query          string
				Syntax         string  `json:"query_syntax"`
				ConceptType    *string `json:"concept_type"`
				Status, Cursor *string
				PathPrefix     *string `json:"path_prefix"`
				Tags           []string
				Limit          *int
			}
			Expected SearchPage
			Error    string
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, q := range fixture.Queries {
		got, err := LexicalQuery(q.Query, q.Syntax)
		if q.Error != "" {
			if err == nil || err.Error() != q.Error {
				t.Fatal(q, got, err)
			}
		} else if err != nil || got != q.Expected {
			t.Fatal(q, got, err)
		}
	}
	root := t.TempDir()
	for path, text := range fixture.Files {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := testStore(t)
	ctx := context.Background()
	if err = s.Rebuild(ctx, root, "main"); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixture.Pages {
		opts := SearchOptions{Query: c.Options.Query, Syntax: c.Options.Syntax, ConceptType: c.Options.ConceptType, Status: c.Options.Status, PathPrefix: c.Options.PathPrefix, Cursor: c.Options.Cursor, Tags: c.Options.Tags, Limit: 20}
		if c.Options.Limit != nil {
			opts.Limit = *c.Options.Limit
		}
		got, err := s.SearchLexical(ctx, c.Policy, opts)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(i, got, err, c.Error)
			}
			continue
		}
		if err != nil || len(got.Results) != len(c.Expected.Results) {
			t.Fatal(i, got, c.Expected, err)
		}
		for j := range got.Results {
			if got.Results[j].Score != c.Expected.Results[j].Score {
				t.Fatal(i, got.Results[j], c.Expected.Results[j])
			}
		}
		if !reflect.DeepEqual(got, c.Expected) {
			t.Fatal(i, got, c.Expected)
		}
	}
}
func TestSearchErrorsAndBounds(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	root := t.TempDir()
	installConcept(t, root)
	if err := s.Rebuild(ctx, root, "main"); err != nil {
		t.Fatal(err)
	}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	opts := SearchOptions{Query: "self", Syntax: "plain", Limit: 1}
	for _, value := range []string{"bad", "offset:bad", "offset:9223372036854775807"} {
		opts.Cursor = &value
		if _, err := s.SearchLexical(ctx, policy, opts); err == nil {
			t.Fatal(value)
		}
	}
	opts.Cursor = nil
	if _, err := s.DB.Exec("UPDATE index_state SET value='quarantined' WHERE key='status'"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SearchLexical(ctx, policy, opts); err == nil {
		t.Fatal("quarantined")
	}
	if _, err := s.DB.Exec("DELETE FROM index_state WHERE key='status'"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("DELETE FROM index_state WHERE key='repo_revision'"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SearchLexical(ctx, policy, opts); err == nil {
		t.Fatal("missing state")
	}
	if got := boundedSnippet(strings.Repeat("😀\x1c", 300)); len([]rune(got)) != 240 {
		t.Fatal(got)
	}
	for _, message := range []string{"unterminated string", "malformed match expression", "fts5: syntax error", "no such column"} {
		if err := searchQueryError(errors.New(message)); err.Error() != "invalid FTS query" {
			t.Fatal(err)
		}
	}
	if err := searchQueryError(io.ErrClosedPipe); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	for _, scan := range []bool{false, true} {
		r := &badRows{}
		if scan {
			r.scan = io.ErrClosedPipe
		} else {
			r.iteration = io.ErrClosedPipe
		}
		if _, err := readSearchRows(r); !errors.Is(err, io.ErrClosedPipe) || !r.closed {
			t.Fatal(err)
		}
	}
	for _, tags := range []string{"{", "null"} {
		rows, err := s.DB.Query(`SELECT 'id','/a.md','Title','concept','active',?,NULL,NULL`, tags)
		if err != nil {
			t.Fatal(err)
		}
		got, err := readSearchRows(rows)
		if tags == "{" {
			if err == nil {
				t.Fatal("bad JSON")
			}
		} else if err != nil || got[0].Snippet != "Title" || got[0].Score != 0 || len(got[0].Tags) != 0 {
			t.Fatal(got, err)
		}
	}
}
func TestSearchEverySQLFailure(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	for fail := 1; fail <= 4; fail++ {
		s, f := faultStore(t)
		if err := s.Rebuild(ctx, root, "main"); err != nil {
			t.Fatal(err)
		}
		f.remaining = fail
		if _, err := s.SearchLexical(ctx, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, SearchOptions{Query: "self", Syntax: "plain", Limit: 20}); err == nil {
			t.Fatal("failure", fail)
		}
	}
}
func FuzzLexicalQuery(f *testing.F) {
	f.Add("what is an alpha?", "plain")
	f.Add("é Ⅵ ² á", "plain")
	f.Add("alpha", "fts5")
	f.Fuzz(func(t *testing.T, text, syntax string) {
		if len(text) > 4096 {
			return
		}
		query, err := LexicalQuery(text, syntax)
		if err != nil {
			return
		}
		if query == "" {
			t.Fatal("empty accepted query")
		}
		if syntax == "fts5" && query != text {
			t.Fatal("FTS query changed")
		}
		if len([]rune(boundedSnippet(text))) > 240 {
			t.Fatal("snippet overflow")
		}
	})
}

func TestSearchConcurrentUpdateIsolation(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	root := t.TempDir()
	installConcept(t, root)
	if err := s.Rebuild(ctx, root, "r0"); err != nil {
		t.Fatal(err)
	}
	private := strings.Replace(testConcept, "'id'", "'private'", 1)
	if err := os.Mkdir(filepath.Join(root, "private"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private/p.md"), []byte(private), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePaths(ctx, root, "r1", []string{"/private/p.md"}); err != nil {
		t.Fatal(err)
	}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 10 {
			if err := s.UpdatePaths(ctx, root, fmt.Sprintf("r%d", i+2), []string{"/a.md"}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 12 {
				page, err := s.SearchLexical(ctx, policy, SearchOptions{Query: "self", Syntax: "plain", Limit: 20})
				if err != nil {
					t.Error(err)
					return
				}
				for _, result := range page.Results {
					if strings.HasPrefix(result.Path, "/private/") {
						t.Error("protected path leaked")
					}
				}
			}
		}()
	}
	wg.Wait()
}

func FuzzLexicalFTS(f *testing.F) {
	ctx := context.Background()
	s := testStore(f)
	root := f.TempDir()
	installConcept(f, root)
	if err := s.Rebuild(ctx, root, "r"); err != nil {
		f.Fatal(err)
	}
	for _, text := range []string{"alpha", "self OR external", "what is [self]?", "é ² Ⅵ 中文", `\"unterminated`} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 512 {
			return
		}
		query, err := LexicalQuery(text, "plain")
		if err != nil {
			return
		}
		if _, err = s.SearchLexical(ctx, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, SearchOptions{Query: query, Syntax: "fts5", Limit: 10}); err != nil {
			t.Fatal(query, err)
		}
	})
}
