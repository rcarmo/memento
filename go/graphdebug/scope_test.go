package graphdebug

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/access"
	_ "modernc.org/sqlite"
)

func TestPathScopeSQL(t *testing.T) {
	if query, args := pathScopeSQL("path", nil); query != "" || args != nil {
		t.Fatal(query, args)
	}
	cases := []struct {
		policy   access.EffectivePolicy
		contains string
		argc     int
	}{{access.EffectivePolicy{Roles: []string{"admin"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}, "CASE WHEN", 2}, {access.EffectivePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}, "AND NOT", 4}, {access.EffectivePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/", "/private/sub/"}, ProtectedReadPrefixes: []string{"/private/"}}, "OR", 8}}
	for _, tc := range cases {
		query, args := pathScopeSQL("c.path", &tc.policy)
		if len(args) != tc.argc || !contains(query, tc.contains) {
			t.Fatal(query, args, tc)
		}
	}
}
func TestPathScopeQuery(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("CREATE TABLE paths(path TEXT); INSERT INTO paths VALUES('/public/a.md'),('/private/a.md'),('/private/sub/a.md'),('/trash/public/a.md'),('/trash/private/a.md')"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		policy access.EffectivePolicy
		want   []string
	}{{access.EffectivePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}, []string{"/public/a.md", "/trash/public/a.md"}}, {access.EffectivePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/", "/private/sub/"}, ProtectedReadPrefixes: []string{"/private/"}}, []string{"/private/sub/a.md", "/public/a.md", "/trash/public/a.md"}}, {access.EffectivePolicy{Roles: []string{"admin"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}, []string{"/private/a.md", "/private/sub/a.md", "/public/a.md", "/trash/private/a.md", "/trash/public/a.md"}}}
	for _, tc := range cases {
		query, args := pathScopeSQL("path", &tc.policy)
		rows, err := db.QueryContext(context.Background(), "SELECT path FROM paths WHERE "+query+" ORDER BY path", args...)
		if err != nil {
			t.Fatal(query, args, err)
		}
		got := []string{}
		for rows.Next() {
			var path string
			if err = rows.Scan(&path); err != nil {
				t.Fatal(err)
			}
			got = append(got, path)
		}
		rows.Close()
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatal(got, tc.want)
		}
	}
}
func TestPrefixScopeEscaping(t *testing.T) {
	query, args := prefixScopeSQL("path", []string{"/a_b%\\/"})
	if query != "((path = ? OR path LIKE ? ESCAPE '\\'))" || !reflect.DeepEqual(args, []any{"/a_b%\\", "/a\\_b\\%\\\\/%"}) {
		t.Fatal(query, args)
	}
}
func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
