package service

import (
	"errors"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/repository"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func legacyFaultRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeLegacySkill(t, root, "demo", "1.0.0", "# Demo\n")
	return root
}
func TestLegacySkillInjectedFailures(t *testing.T) {
	boom := errors.New("boom")
	tests := []func(*legacySkillIO){func(o *legacySkillIO) { o.readDir = func(string) ([]os.DirEntry, error) { return nil, boom } }, func(o *legacySkillIO) {
		base := o.readDir
		o.readDir = func(path string) ([]os.DirEntry, error) {
			if strings.HasSuffix(path, "/.versions/demo") {
				return nil, boom
			}
			return base(path)
		}
	}, func(o *legacySkillIO) { o.readFile = func(string) ([]byte, error) { return nil, boom } }, func(o *legacySkillIO) {
		o.serialize = func(repository.ConceptDocument) (string, error) { return "", boom }
	}, func(o *legacySkillIO) { o.mkdirAll = func(string, os.FileMode) error { return boom } }, func(o *legacySkillIO) { o.writeFile = func(string, []byte, os.FileMode) error { return boom } }, func(o *legacySkillIO) {
		base := o.readFile
		o.readFile = func(path string) ([]byte, error) {
			if strings.HasSuffix(path, ".zip") {
				return nil, boom
			}
			return base(path)
		}
	}, func(o *legacySkillIO) {
		o.writeAsset = func(string, assets.AcceptedVersion) ([]string, error) { return nil, boom }
	}, func(o *legacySkillIO) {
		o.remove = func(path string) error {
			if strings.HasSuffix(path, ".md") {
				return boom
			}
			return nil
		}
	}, func(o *legacySkillIO) {
		o.remove = func(path string) error {
			if strings.HasSuffix(path, ".zip") {
				return boom
			}
			return nil
		}
	}}
	for i, mutate := range tests {
		ops := defaultLegacySkillIO()
		mutate(&ops)
		if _, err := migrateLegacySkillPacks(legacyFaultRoot(t), ops); err == nil {
			t.Fatal(i)
		}
	}
}
func TestLegacySkillDefaultsAndConceptFailure(t *testing.T) {
	root := legacyFaultRoot(t)
	metadata := filepath.Join(root, "skills", ".versions", "demo", "1.0.0.md")
	raw, err := os.ReadFile(metadata)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(raw), `"source_proposal_id":"12345678-abcd"`, `"source_proposal_id":""`)
	text = strings.ReplaceAll(text, `"accepted_by":"curator"`, `"accepted_by":""`)
	if err = os.WriteFile(metadata, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	var captured assets.AcceptedVersion
	ops := defaultLegacySkillIO()
	ops.writeAsset = func(_ string, value assets.AcceptedVersion) ([]string, error) {
		captured = value
		return []string{"/.assets/x.json", "/.assets/x.zip"}, nil
	}
	if _, err = migrateLegacySkillPacks(root, ops); err != nil || captured.AcceptedBy != "memento-migration" || captured.SourceProposalID != "legacy" || captured.ConceptID != "legacy-demo" {
		t.Fatal(captured, err)
	}
	ops = defaultLegacySkillIO()
	ops.now = func() time.Time { return time.Time{} }
	document := legacySkillConcept("missing", "demo", "id", "body", ops)
	if document.Frontmatter.CreatedAt.Year() != 1 {
		t.Fatal(document)
	}
	root = legacyFaultRoot(t)
	writeLegacySkill(t, root, "demo", "2.0.0", "# New\n")
	old := filepath.Join(root, "skills", ".versions", "demo", "1.0.0.md")
	base := defaultLegacySkillIO()
	baseRead := base.readFile
	base.readFile = func(path string) ([]byte, error) {
		if path == old {
			return nil, errors.New("old metadata")
		}
		return baseRead(path)
	}
	if _, err = migrateLegacySkillPacks(root, base); err == nil {
		t.Fatal("per-version metadata")
	}
	_ = captured
}
func TestLegacySkillVersionBranches(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", ".versions", "demo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	versions, err := legacySkillVersions(dir, defaultLegacySkillIO())
	if err != nil || len(versions) != 0 {
		t.Fatal(versions, err)
	}
	if err = os.WriteFile(filepath.Join(dir, "bad.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = legacySkillVersions(dir, defaultLegacySkillIO()); err == nil {
		t.Fatal("semver")
	}
	if err = os.Remove(filepath.Join(dir, "bad.md")); err != nil {
		t.Fatal(err)
	}
	writeLegacySkill(t, root, "demo", "1.0.0", "# One\n")
	writeLegacySkill(t, root, "demo", "1.0.1", "# Two\n")
	writeLegacySkill(t, root, "demo", "2.0.0", "# Three\n")
	if err = os.WriteFile(filepath.Join(dir, "1.10.0.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "1.1٠.0.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	versions, err = legacySkillVersions(dir, defaultLegacySkillIO())
	if err != nil || strings.Join(versions, ",") != "1.0.0,1.0.1,1.10.0,1.1٠.0,2.0.0" {
		t.Fatal(versions, err)
	}
	empty := t.TempDir()
	if err = os.MkdirAll(filepath.Join(empty, "skills", ".versions", "file"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(empty, "skills", ".versions", "plain"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	changed, err := MigrateLegacySkillPacks(empty)
	if err != nil || len(changed) != 0 {
		t.Fatal(changed, err)
	}
}
