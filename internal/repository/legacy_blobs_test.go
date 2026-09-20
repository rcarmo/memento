package repository

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func legacyPointer(blob []byte) string {
	digest := sha256.Sum256(blob)
	return fmt.Sprintf("version https://git-lfs.github.com/spec/v1\noid sha256:%x\nsize %d\n", digest, len(blob))
}
func putLegacyObject(t *testing.T, root string, blob []byte) string {
	t.Helper()
	digest := fmt.Sprintf("%x", sha256.Sum256(blob))
	path := filepath.Join(root, digest[:2], digest[2:4], digest)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, blob, 0600); err != nil {
		t.Fatal(err)
	}
	return digest
}
func TestLegacyBlobDetection(t *testing.T) {
	root := t.TempDir()
	needed, err := RepositoryNeedsLegacyBlobMigration(root)
	if err != nil || needed {
		t.Fatal(needed, err)
	}
	if err = os.WriteFile(filepath.Join(root, "large"), make([]byte, 257), 0600); err != nil {
		t.Fatal(err)
	}
	needed, err = RepositoryNeedsLegacyBlobMigration(root)
	if err != nil || needed {
		t.Fatal(needed, err)
	}
	blob := []byte("asset")
	if err = os.WriteFile(filepath.Join(root, "pointer"), []byte(legacyPointer(blob)), 0600); err != nil {
		t.Fatal(err)
	}
	needed, err = RepositoryNeedsLegacyBlobMigration(root)
	if err != nil || !needed {
		t.Fatal(needed, err)
	}
	if err = os.Remove(filepath.Join(root, "pointer")); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.zip filter=lfs diff=lfs\n"), 0600); err != nil {
		t.Fatal(err)
	}
	needed, err = RepositoryNeedsLegacyBlobMigration(root)
	if err != nil || !needed {
		t.Fatal(needed, err)
	}
	if _, err = RepositoryNeedsLegacyBlobMigration(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing")
	}
}
func TestMigrateLegacyBlobs(t *testing.T) {
	root, objects := t.TempDir(), t.TempDir()
	one, two := []byte("one"), []byte("two")
	putLegacyObject(t, objects, one)
	putLegacyObject(t, objects, two)
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a"), []byte(legacyPointer(one)), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "b"), []byte(legacyPointer(two)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.zip filter=lfs\nkeep text\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := MigrateLegacyBlobsToGit(root, objects)
	if err != nil || !reflect.DeepEqual(changed, []string{"/.gitattributes", "/a", "/nested/b"}) {
		t.Fatal(changed, err)
	}
	if raw, _ := os.ReadFile(filepath.Join(root, "a")); !reflect.DeepEqual(raw, one) {
		t.Fatal(raw)
	}
	if info, statErr := os.Stat(filepath.Join(root, "a")); statErr != nil || info.Mode().Perm() != 0640 {
		t.Fatal(info, statErr)
	}
	if raw, _ := os.ReadFile(filepath.Join(root, ".gitattributes")); string(raw) != "keep text\n" {
		t.Fatal(string(raw))
	}
	changed, err = MigrateLegacyBlobsToGit(root, objects)
	if err != nil || len(changed) != 0 {
		t.Fatal(changed, err)
	}
}
func TestLegacyBlobFilesystemFailures(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".gitattributes"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := RepositoryNeedsLegacyBlobMigration(root); err == nil {
		t.Fatal("attribute read")
	}
	if _, err := MigrateLegacyBlobsToGit(root, t.TempDir()); err == nil {
		t.Fatal("attribute migrate")
	}
	root = t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "blocked"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "blocked"), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(root, "blocked"), 0700)
	_, _ = RepositoryNeedsLegacyBlobMigration(root)
}

func TestMigrateLegacyBlobFailures(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		root, objects := t.TempDir(), t.TempDir()
		blob := []byte("asset")
		digest := fmt.Sprintf("%x", sha256.Sum256(blob))
		if corrupt {
			path := filepath.Join(objects, digest[:2], digest[2:4], digest)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("wrong"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(root, "a"), []byte(legacyPointer(blob)), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := MigrateLegacyBlobsToGit(root, objects); err == nil {
			t.Fatal(corrupt)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.zip filter=lfs\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := MigrateLegacyBlobsToGit(root, t.TempDir())
	if err != nil || !reflect.DeepEqual(changed, []string{"/.gitattributes"}) {
		t.Fatal(changed, err)
	}
	if _, err = os.Stat(filepath.Join(root, ".gitattributes")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
