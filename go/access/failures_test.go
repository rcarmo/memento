package access

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAccessOpeningFailures(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"random", "master", "insert", "closed"} {
		db, _ := accessDB(t)
		random := io.Reader(&sequentialRandom{})
		key := "master"
		switch kind {
		case "random":
			random = strings.NewReader("")
		case "master":
			key = ""
		case "insert":
			_, _ = db.Exec(`CREATE TRIGGER blocked BEFORE INSERT ON access_meta BEGIN SELECT RAISE(ABORT,'test'); END`)
		case "closed":
			db.Close()
		}
		if _, err := openStore(ctx, db, key, random, time.Now); err == nil {
			t.Fatal(kind)
		}
	}
	s, _ := syntheticStore(t)
	s.db.Close()
	for _, run := range []func() error{
		func() error { _, err := s.Authenticate(ctx, "x"); return err }, func() error { _, err := s.Policy(ctx, "x"); return err }, func() error { _, err := s.List(ctx); return err }, func() error { _, err := s.require(ctx, "x"); return err }, func() error { return s.otherAdmin(ctx, "x") }, func() error { _, err := s.Audit(ctx, 1); return err }, func() error { return s.RotateMasterKey(ctx, "a", "b") }, func() error { return s.Bootstrap(ctx, nil, nil) },
	} {
		if err := run(); err == nil {
			t.Fatal("closed access store")
		}
	}
}
func TestAccessBootstrapAndRows(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"policy", "principal-insert", "credential-insert", "old-query"} {
		s, _ := syntheticStore(t)
		principals := []ConfiguredPrincipal{{Name: "one", Policy: NamespacePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}}}
		tokens := map[string]string{"one": "token"}
		switch kind {
		case "policy":
			principals[0].Name = "INVALID"
		case "principal-insert":
			_, _ = s.db.Exec(`CREATE TRIGGER blocked BEFORE INSERT ON access_principals BEGIN SELECT RAISE(ABORT,'test'); END`)
		case "credential-insert":
			_, _ = s.db.Exec(`CREATE TRIGGER blocked BEFORE INSERT ON access_credentials BEGIN SELECT RAISE(ABORT,'test'); END`)
		case "old-query":
			_, _ = s.db.Exec("DROP TABLE access_credentials")
			_, _ = s.db.Exec("DROP TABLE access_principals")
			principals = nil
		}
		if err := s.Bootstrap(ctx, principals, tokens); err == nil {
			t.Fatal(kind)
		}
	}
	s, _ := syntheticStore(t)
	_, err := s.db.Exec(`INSERT INTO access_principals VALUES('piclaw-workspace','["reader"]','["/"]','[]',1,0,0,'old','old')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Bootstrap(ctx, nil, nil); err != nil {
		t.Fatal(err)
	}
	item, err := s.require(ctx, "sandbox")
	if err != nil || item.Name != "sandbox" {
		t.Fatal(item, err)
	}
	// No admin is added by the legacy rename branch.
	if contains(item.Roles, "admin") {
		t.Fatal("legacy rename changed roles")
	}
	for _, field := range []string{"roles_json", "read_prefixes_json", "write_prefixes_json"} {
		s, _ := syntheticStore(t)
		_, token, err := s.Create(ctx, "admin", "one", []string{"reader"}, []string{"/"}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = s.db.Exec("UPDATE access_principals SET " + field + "='{' WHERE name='one'")
		if _, err = s.List(ctx); err == nil {
			t.Fatal(field)
		}
		if _, err = s.Policy(ctx, "one"); err == nil {
			t.Fatal(field)
		}
		if field == "roles_json" {
			if _, err = s.Authenticate(ctx, token); err == nil {
				t.Fatal("invalid auth roles")
			}
		}
	}
	s, _ = syntheticStore(t)
	_, _, _ = s.Create(ctx, "admin", "one", []string{"reader"}, []string{"/"}, nil, nil)
	_, _ = s.db.Exec(`UPDATE access_principals SET read_prefixes_json='[]'`)
	if _, err = s.Policy(ctx, "one"); err == nil {
		t.Fatal("invalid stored policy")
	}
}
func TestAccessMutationValidationAndRandomness(t *testing.T) {
	s, _ := syntheticStore(t)
	ctx := context.Background()
	if _, _, err := s.Create(ctx, "a", "INVALID", []string{"reader"}, []string{"/"}, nil, nil); err == nil {
		t.Fatal("create policy")
	}
	if _, err := s.Update(ctx, "a", "INVALID", []string{"reader"}, []string{"/"}, nil); err == nil {
		t.Fatal("update policy")
	}
	for _, run := range []func() error{func() error {
		_, err := s.Update(ctx, "a", "missing", []string{"reader"}, []string{"/"}, nil)
		return err
	}, func() error { _, err := s.Rename(ctx, "a", "missing", "new"); return err }, func() error { _, err := s.SetEnabled(ctx, "a", "missing", true); return err }, func() error { _, err := s.Revoke(ctx, "a", "missing"); return err }, func() error { _, err := s.Delete(ctx, "a", "missing"); return err }} {
		if err := run(); err == nil {
			t.Fatal("missing principal")
		}
	}
	_, _, err := s.Create(ctx, "a", "one", []string{"reader"}, []string{"/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Rename(ctx, "a", "one", "INVALID"); err == nil {
		t.Fatal("rename validation")
	}
	s.random = strings.NewReader("")
	key := "burn-on-failure"
	if _, _, err = s.Create(ctx, "a", "two", []string{"reader"}, []string{"/"}, nil, &key); err == nil {
		t.Fatal("random fail")
	}
	s.random = &sequentialRandom{}
	if _, _, err = s.Create(ctx, "a", "two", []string{"reader"}, []string{"/"}, nil, &key); err == nil {
		t.Fatal("one-time claim rolled back")
	}
	s.random = strings.NewReader("")
	if _, err = s.Rotate(ctx, "a", "one", nil); err == nil {
		t.Fatal("rotate entropy")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.audit(ctx, tx, "a", "x", "one", map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("audit encoding")
	}
	_ = tx.Rollback()
}
func TestAccessMutationRollback(t *testing.T) {
	ctx := context.Background()
	for _, action := range []string{"create", "update", "rename", "enable", "rotate", "revoke", "delete"} {
		for _, table := range []string{"access_principals", "access_credentials", "access_audit"} {
			t.Run(action+"/"+table, func(t *testing.T) {
				s, _ := syntheticStore(t)
				_, token, err := s.Create(ctx, "admin", "one", []string{"reader"}, []string{"/"}, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if action == "delete" {
					if _, err = s.Revoke(ctx, "admin", "one"); err != nil {
						t.Fatal(err)
					}
				}
				before, _ := s.List(ctx)
				auditBefore, _ := s.Audit(ctx, 100)
				verb := "UPDATE"
				if action == "create" || table == "access_audit" {
					verb = "INSERT"
				}
				_, err = s.db.Exec("CREATE TRIGGER blocked BEFORE " + verb + " ON " + table + " BEGIN SELECT RAISE(ABORT,'test'); END")
				if err != nil {
					t.Fatal(err)
				}
				switch action {
				case "create":
					_, _, err = s.Create(ctx, "admin", "new", []string{"reader"}, []string{"/"}, nil, nil)
				case "update":
					_, err = s.Update(ctx, "admin", "one", []string{"proposer"}, []string{"/"}, nil)
				case "rename":
					_, err = s.Rename(ctx, "admin", "one", "renamed")
				case "enable":
					_, err = s.SetEnabled(ctx, "admin", "one", false)
				case "rotate":
					_, err = s.Rotate(ctx, "admin", "one", nil)
				case "revoke":
					_, err = s.Revoke(ctx, "admin", "one")
				case "delete":
					_, err = s.Delete(ctx, "admin", "one")
				}
				affected := table != "access_credentials" || action == "create" || action == "rotate" || action == "revoke" || action == "rename"
				if affected {
					if err == nil {
						t.Fatal("trigger ignored")
					}
					after, _ := s.List(ctx)
					auditAfter, _ := s.Audit(ctx, 100)
					if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(auditBefore, auditAfter) {
						t.Fatal("partial mutation")
					}
					if action != "delete" {
						if p, err := s.Authenticate(ctx, token); err != nil || p == nil {
							t.Fatal("credential changed on rollback", err)
						}
					}
				}
			})
		}
	}
}
func TestMasterRotationFailures(t *testing.T) {
	ctx := context.Background()
	s, _ := syntheticStore(t)
	if err := s.RotateMasterKey(ctx, "wrong", "new"); err == nil {
		t.Fatal("wrong old key")
	}
	if err := s.RotateMasterKey(ctx, "synthetic-master", ""); err == nil {
		t.Fatal("empty new key")
	}
	_, _ = s.db.Exec("DELETE FROM access_meta")
	if err := s.RotateMasterKey(ctx, "x", "y"); err == nil {
		t.Fatal("missing verifier")
	}
}

type failRows struct {
	done    bool
	raw     string
	scanErr error
	audit   bool
}

func (r *failRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}
func (r *failRows) Err() error { return nil }
func (r *failRows) Scan(values ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.audit {
		*(values[3].(*string)) = r.raw
	}
	return nil
}
func TestAccessRowErrors(t *testing.T) {
	if _, err := scanPrincipals(&failRows{scanErr: io.ErrClosedPipe}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if _, err := scanAudit(&failRows{scanErr: io.ErrClosedPipe}); err == nil {
		t.Fatal("scan audit")
	}
	for _, raw := range []string{"{", "[]"} {
		if _, err := scanAudit(&failRows{raw: raw, audit: true}); err == nil {
			t.Fatal(raw)
		}
	}
}

func TestAccessDoesNotReturnCredentialOnPostWriteFailure(t *testing.T) {
	s, _ := syntheticStore(t)
	ctx := context.Background()
	_, err := s.db.Exec(`CREATE TRIGGER remove_created AFTER INSERT ON access_audit BEGIN DELETE FROM access_credentials WHERE principal_name=NEW.target; DELETE FROM access_principals WHERE name=NEW.target; END`)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := s.Create(ctx, "admin", "one", []string{"reader"}, []string{"/"}, nil, nil)
	if err == nil || token != "" {
		t.Fatal("credential returned with error")
	}
	s, _ = syntheticStore(t)
	_, token, err = s.Create(ctx, "admin", "one", []string{"reader"}, []string{"/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = s.db.Exec(`UPDATE access_principals SET roles_json='[]' WHERE name='one'`)
	if p, err := s.Authenticate(ctx, token); err == nil || p != nil {
		t.Fatal("invalid principal authenticated")
	}
}
