package access

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/control"
)

func TestAuthorizationReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/access-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Policy              EffectivePolicy
			Path, Action, Error string
			Expected            AuthorizedNamespace
		}
		Managed []struct {
			Name                 string
			Roles, Reads, Writes []string
			Error                string
			Expected             [][]string
		}
		Config   AuthorizationConfig
		Resolved []struct {
			Name     string
			Roles    []string
			Error    string
			Expected EffectivePolicy
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixture.Cases {
		got, err := AuthorizePath(c.Policy, c.Path, c.Action)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(i, err, c.Error)
			}
		} else if err != nil || got != c.Expected {
			t.Fatal(i, got, c.Expected, err)
		}
	}
	for _, c := range fixture.Managed {
		roles, reads, writes, err := validatePolicy(c.Name, c.Roles, c.Reads, c.Writes)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(err, c.Error)
			}
		} else if err != nil || !reflect.DeepEqual([][]string{roles, reads, writes}, c.Expected) {
			t.Fatal(roles, reads, writes, c.Expected, err)
		}
	}
	config, err := ValidateConfig(fixture.Config)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Resolved {
		got, err := ResolvePolicy(config, Principal{Name: c.Name, Roles: c.Roles})
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(err, c.Error)
			}
		} else if err != nil || !reflect.DeepEqual(got, c.Expected) {
			t.Fatal(got, c.Expected, err)
		}
	}
}
func TestAuthorizationHelpers(t *testing.T) {
	p := EffectivePolicy{Principal: "p", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{}, ProtectedReadPrefixes: []string{"/private/"}}
	if err := RequireRole(p, "reader"); err != nil {
		t.Fatal(err)
	}
	if err := RequireRole(p, "admin"); err == nil {
		t.Fatal("missing role")
	}
	if warning := BroadReadGrantWarning(p.Roles, p.ReadPrefixes, p.ProtectedReadPrefixes); warning == nil {
		t.Fatal("missing warning")
	}
	if warning := BroadReadGrantWarning([]string{"admin"}, p.ReadPrefixes, p.ProtectedReadPrefixes); warning != nil {
		t.Fatal(*warning)
	}
	if warning := BroadReadGrantWarning(p.Roles, []string{"/", "/private/"}, p.ProtectedReadPrefixes); warning != nil {
		t.Fatal(*warning)
	}
	paths := []string{"/a", "/private/a", "/trash/a", "relative", "/a"}
	if got := FilterAuthorizedPaths(p, paths, "read"); !reflect.DeepEqual(got, []string{"/a", "/trash/a", "/a"}) {
		t.Fatal(got)
	}
	if !PathMatchesPrefix("x", "") {
		t.Fatal("source empty prefix")
	}
	for _, c := range []AuthorizationConfig{{Principals: map[string]NamespacePolicy{"p": {}}}, {Principals: map[string]NamespacePolicy{"p": {Roles: []string{"r"}, TokenEnv: "TOKEN", ReadPrefixes: []string{"bad"}}}}, {ProtectedReadPrefixes: []string{"/"}}} {
		if _, err := ValidateConfig(c); err == nil {
			t.Fatal(c)
		}
	}
}

type sequentialRandom struct{ position int }

func (r *sequentialRandom) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r.position + i)
	}
	r.position += len(p)
	return len(p), nil
}
func accessDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sqlite")
	db, err := control.Connect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = control.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db, path
}
func syntheticStore(t *testing.T) (*Store, string) {
	t.Helper()
	db, path := accessDB(t)
	s, err := openStore(context.Background(), db, "synthetic-master", &sequentialRandom{}, func() time.Time { return time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}
func normal(value any) any {
	raw, _ := json.Marshal(value)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}
func TestManagedAccessReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/access-store.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		InitialWrap string `json:"initial_wrap"`
		FinalWrap   string `json:"final_wrap"`
		Cases       []struct {
			Input struct {
				Action, Name, Token  string
				NewName              string `json:"new_name"`
				Roles, Reads, Writes []string
				Key                  *string
			}
			Result       json.RawMessage
			Token, Error string
		}
		Principals []ManagedPrincipal
		Audit      []AuditEntry
		Rows       map[string][]map[string]any
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	s, path := syntheticStore(t)
	ctx := context.Background()
	bootstrap := []ConfiguredPrincipal{{Name: "piclaw-workspace", Policy: NamespacePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}}}
	for _, token := range []string{"synthetic-admin", "ignored-replacement"} {
		if err = s.Bootstrap(ctx, bootstrap, map[string]string{"piclaw-workspace": token}); err != nil {
			t.Fatal(err)
		}
	}
	var wrapped string
	_ = s.db.QueryRow("SELECT value FROM access_meta WHERE key='verifier_key'").Scan(&wrapped)
	if wrapped != fixture.InitialWrap {
		t.Fatal("AES-GCM/scrypt bytes differ")
	}
	last := ""
	for i, c := range fixture.Cases {
		var result any
		var token string
		var err error
		switch c.Input.Action {
		case "authenticate":
			result, err = s.Authenticate(ctx, strings.ReplaceAll(c.Input.Token, "{{LAST}}", last))
		case "policy":
			result, err = s.Policy(ctx, c.Input.Name)
		case "create":
			result, token, err = s.Create(ctx, "admin", c.Input.Name, c.Input.Roles, c.Input.Reads, c.Input.Writes, c.Input.Key)
		case "rotate":
			token, err = s.Rotate(ctx, "admin", c.Input.Name, c.Input.Key)
		case "update":
			result, err = s.Update(ctx, "admin", c.Input.Name, c.Input.Roles, c.Input.Reads, c.Input.Writes)
		case "rename":
			result, err = s.Rename(ctx, "admin", c.Input.Name, c.Input.NewName)
		case "disable", "enable":
			result, err = s.SetEnabled(ctx, "admin", c.Input.Name, c.Input.Action == "enable")
		case "revoke":
			result, err = s.Revoke(ctx, "admin", c.Input.Name)
		case "delete":
			result, err = s.Delete(ctx, "admin", c.Input.Name)
		}
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatalf("case %d: %v != %s", i, err, c.Error)
			}
			continue
		}
		if err != nil {
			t.Fatal(i, err)
		}
		if c.Token != "" {
			if token != c.Token {
				t.Fatal(i, "token mismatch")
			}
			last = token
		}
		if len(c.Result) > 0 {
			var expected any
			_ = json.Unmarshal(c.Result, &expected)
			if !reflect.DeepEqual(normal(result), expected) {
				t.Fatal(i, normal(result), expected)
			}
		}
	}
	if err = s.RotateMasterKey(ctx, "synthetic-master", "synthetic-new-master"); err != nil {
		t.Fatal(err)
	}
	_ = s.db.QueryRow("SELECT value FROM access_meta WHERE key='verifier_key'").Scan(&wrapped)
	if wrapped != fixture.FinalWrap {
		t.Fatal("key rotation bytes differ")
	}
	principals, err := s.List(ctx)
	if err != nil || !reflect.DeepEqual(principals, fixture.Principals) {
		t.Fatal(principals, err)
	}
	audit, err := s.Audit(ctx, 100)
	if err != nil || !reflect.DeepEqual(normal(audit), normal(fixture.Audit)) {
		t.Fatal(audit, err)
	}
	for table, want := range fixture.Rows {
		rows, err := s.db.Query("SELECT * FROM " + table + " ORDER BY 1")
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		got := []map[string]any{}
		for rows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err = rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			row := map[string]any{}
			for i, col := range cols {
				row[col] = values[i]
			}
			got = append(got, row)
		}
		rows.Close()
		if !reflect.DeepEqual(normal(got), normal(want)) {
			t.Fatal(table, got, want)
		}
	}
	s.db.Close()
	db, err := control.Connect(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = OpenStore(ctx, db, "synthetic-new-master"); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenStore(ctx, db, "synthetic-master"); err == nil {
		t.Fatal("old key accepted")
	}
}
func TestAccessCryptoErrors(t *testing.T) {
	if _, err := masterKey(""); err == nil {
		t.Fatal("empty master")
	}
	for _, c := range []struct{ Payload, Key string }{{"bad", "key"}, {base64.URLEncoding.EncodeToString(make([]byte, 30)), ""}, {base64.URLEncoding.EncodeToString(make([]byte, 30)), "wrong"}} {
		if _, err := openKey(c.Payload, c.Key); err == nil || err.Error() != "access master key is invalid" {
			t.Fatal(err)
		}
	}
	if _, err := sealKey(nil, "x", strings.NewReader("")); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	if _, err := sealKey(nil, "", bytes.NewReader(make([]byte, 12))); err == nil {
		t.Fatal("empty sealing key")
	}
	if _, err := newToken(strings.NewReader("")); err == nil {
		t.Fatal("empty randomness")
	}
}

func FuzzAuthorizationPath(f *testing.F) {
	for _, path := range []string{"/", "/public/a", "/private/x", "/trash/private/x", "/a/../b"} {
		f.Add(path, "read")
	}
	f.Fuzz(func(t *testing.T, path, action string) {
		policy := EffectivePolicy{Principal: "fuzz", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/public/"}, ProtectedReadPrefixes: []string{"/private/"}}
		_, _ = AuthorizePath(policy, path, action)
	})
}
func TestAccessRevocationConcurrentReads(t *testing.T) {
	s, _ := syntheticStore(t)
	ctx := context.Background()
	_, token, err := s.Create(ctx, "admin", "reader", []string{"reader"}, []string{"/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 20 {
				if _, err := s.Authenticate(ctx, token); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	if _, err = s.Revoke(ctx, "admin", "reader"); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	if p, err := s.Authenticate(ctx, token); err != nil || p != nil {
		t.Fatal("revoked token accepted", p, err)
	}
}
