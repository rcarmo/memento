package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"
)

func TestAuditFailurePaths(t *testing.T) {
	ctx := context.Background()
	c, actor, _ := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer"}
	repo := fakeRepo(nil)
	fail := func(string, func(string) bool) (repository.RepositoryAudit, error) {
		return repository.RepositoryAudit{}, io.ErrClosedPipe
	}
	if _, _, err := c.audit(ctx, actor, AuditOptions{Limit: 1}, repo, fail); err != io.ErrClosedPipe {
		t.Fatal(err)
	}
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, _, err := c.audit(ctx, actor, AuditOptions{Limit: 1}, repo, repository.AuditRepository); err != io.ErrClosedPipe {
		t.Fatal(err)
	}
	c.AuditGraph = auditProvider(func(context.Context, access.EffectivePolicy) (AuditOverview, error) {
		return AuditOverview{}, io.ErrClosedPipe
	})
	if _, _, err := c.audit(ctx, actor, AuditOptions{Limit: 1}, fakeRepo(nil), repository.AuditRepository); err != io.ErrClosedPipe {
		t.Fatal(err)
	}
	c.AuditGraph = auditProvider(func(context.Context, access.EffectivePolicy) (AuditOverview, error) {
		return AuditOverview{}, syscall.EACCES
	})
	if _, _, err := c.audit(ctx, actor, AuditOptions{Limit: 1}, fakeRepo(nil), repository.AuditRepository); err != nil {
		t.Fatal(err)
	}
	c.AuditGraph = auditProvider(func(context.Context, access.EffectivePolicy) (AuditOverview, error) {
		return AuditOverview{IndexRevision: "main", Diagnostics: []AuditDiagnostic{{ID: string([]byte{255}), Severity: "error", Rule: "first"}, {ID: "later", Severity: "warning", Rule: "last"}}}, nil
	})
	if _, _, err := c.audit(ctx, actor, AuditOptions{Limit: 1}, fakeRepo(nil), repository.AuditRepository); err == nil {
		t.Fatal("cursor encoding")
	}
	// A prefix may pass read via trash unwrapping but fail write against its
	// literal original grant. Both checks must remain in scope construction.
	policy := access.EffectivePolicy{Principal: "actor", Roles: []string{"proposer"}, ReadPrefixes: []string{"/trash/public/", "/public/"}, WritePrefixes: []string{"/trash/public/"}}
	if got := writableAuditPolicy(policy); len(got.ReadPrefixes) != 0 {
		t.Fatal(got)
	}
	if auditKeyLess([3]string{"a", "b", "c"}, [3]string{"a", "b", "c"}) {
		t.Fatal("equal key")
	}
	if err := os.WriteFile(filepath.Join(c.Queue.Paths.CurrentDir, "bad.md"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Audit(ctx, actor, AuditOptions{Limit: 1}); err == nil {
		t.Fatal("malformed bundle")
	}
	filters := auditFilters(AuditOptions{})
	for _, text := range []string{"é", "A", "%%%%", base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"revision":"main","filters":{"path":null,"rule":null,"severity":null},"after":[]}`))} {
		if _, err := decodeAuditCursor(&text, "main", filters); err == nil {
			t.Fatal(text)
		}
	}
	// URL-safe alphabet branches using UTF-8 bytes in valid JSON trailing space
	// are exercised separately from successful round trips.
	for _, text := range []string{"-___", "////"} {
		if _, err := decodeAuditCursor(&text, "main", filters); err == nil {
			t.Fatal(text)
		}
	}
}
func FuzzAuditCursor(f *testing.F) {
	token, _ := encodeAuditCursor([3]string{"warning", "rule", "id"}, "revision", auditFilters(AuditOptions{}))
	for _, s := range []string{token, "", token + "====", "bad", "-___"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			return
		}
		filters := auditFilters(AuditOptions{})
		key, err := decodeAuditCursor(&s, "revision", filters)
		if err != nil {
			return
		}
		token, err := encodeAuditCursor(*key, "revision", filters)
		if err != nil {
			t.Fatal(err)
		}
		again, err := decodeAuditCursor(&token, "revision", filters)
		if err != nil || *again != *key {
			t.Fatal(fmt.Sprint(key), err)
		}
	})
}
