package repository

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeDirEntry struct {
	info    os.FileInfo
	dir     bool
	regular bool
}

func (e fakeDirEntry) Name() string { return "x" }
func (e fakeDirEntry) IsDir() bool  { return e.dir }
func (e fakeDirEntry) Type() fs.FileMode {
	if e.regular {
		return 0
	}
	return os.ModeSymlink
}
func (e fakeDirEntry) Info() (os.FileInfo, error) {
	if e.info == nil {
		return nil, errors.New("info")
	}
	return e.info, nil
}
func TestLegacyBlobInjectedFailures(t *testing.T) {
	boom := errors.New("boom")
	if (&LegacyBlobMigrationError{Message: "x"}).Error() != "x" {
		t.Fatal("error")
	}
	base := defaultLegacyBlobIO()
	ops := base
	ops.walkDir = func(string, fs.WalkDirFunc) error { return boom }
	if _, err := repositoryNeedsLegacyBlobMigration("x", ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ops = base
	ops.walkDir = func(root string, fn fs.WalkDirFunc) error { return fn("x", fakeDirEntry{}, boom) }
	if _, err := repositoryNeedsLegacyBlobMigration("x", ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ops = base
	ops.walkDir = func(root string, fn fs.WalkDirFunc) error { return fn("x", fakeDirEntry{}, nil) }
	if _, err := repositoryNeedsLegacyBlobMigration("x", ops); err == nil {
		t.Fatal("info")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "p"), []byte(legacyPointer([]byte("x"))), 0600); err != nil {
		t.Fatal(err)
	}
	ops = base
	ops.readFile = func(path string) ([]byte, error) {
		if filepath.Base(path) == "p" {
			return nil, boom
		}
		return os.ReadFile(path)
	}
	if _, err := repositoryNeedsLegacyBlobMigration(root, ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	attrs := filepath.Join(root, ".gitattributes")
	if err := os.WriteFile(attrs, []byte("*.x filter=lfs\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ops = base
	ops.remove = func(string) error { return boom }
	if _, err := migrateLegacyBlobsToGit(root, t.TempDir(), ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if err := os.WriteFile(attrs, []byte("keep\n*.x filter=lfs\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ops = base
	ops.writeFile = func(string, []byte, os.FileMode) error { return boom }
	if _, err := migrateLegacyBlobsToGit(root, t.TempDir(), ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ops = base
	ops.walkDir = func(string, fs.WalkDirFunc) error { return boom }
	if _, err := migrateLegacyBlobsToGit(t.TempDir(), t.TempDir(), ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ops = base
	ops.walkDir = func(root string, fn fs.WalkDirFunc) error { return fn("x", fakeDirEntry{}, boom) }
	if _, err := migrateLegacyBlobsToGit(t.TempDir(), t.TempDir(), ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
func TestLegacyBlobInjectedPointerFailures(t *testing.T) {
	boom := errors.New("boom")
	root := t.TempDir()
	blob := []byte("x")
	pointer := legacyPointer(blob)
	target := filepath.Join(root, "p")
	if err := os.WriteFile(target, []byte(pointer), 0600); err != nil {
		t.Fatal(err)
	}
	overflow := strings.Replace(pointer, "size 1", "size 999999999999999999999999", 1)
	if err := os.WriteFile(target, []byte(overflow), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateLegacyBlobsToGit(root, t.TempDir(), defaultLegacyBlobIO()); err == nil {
		t.Fatal("size overflow")
	}
	if err := os.WriteFile(target, []byte(pointer), 0600); err != nil {
		t.Fatal(err)
	}
	cases := []func(*legacyBlobIO){func(o *legacyBlobIO) {
		o.readFile = func(path string) ([]byte, error) {
			if path == target {
				return nil, boom
			}
			return os.ReadFile(path)
		}
	}, func(o *legacyBlobIO) { o.rel = func(string, string) (string, error) { return "", boom } }, func(o *legacyBlobIO) {
		o.readFile = func(path string) ([]byte, error) {
			if path != target && filepath.Base(path) != ".gitattributes" {
				return nil, boom
			}
			return os.ReadFile(path)
		}
	}, func(o *legacyBlobIO) { o.writeFile = func(string, []byte, os.FileMode) error { return boom } }}
	for i, mutate := range cases {
		ops := defaultLegacyBlobIO()
		mutate(&ops)
		if _, err := migrateLegacyBlobsToGit(root, t.TempDir(), ops); err == nil {
			t.Fatal(i)
		}
	}
	objectRoot := t.TempDir()
	putLegacyObject(t, objectRoot, blob)
	ops := defaultLegacyBlobIO()
	ops.readFile = func(path string) ([]byte, error) {
		if path != target && filepath.Base(path) != ".gitattributes" {
			return nil, boom
		}
		return os.ReadFile(path)
	}
	if _, err := migrateLegacyBlobsToGit(root, objectRoot, ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ops = defaultLegacyBlobIO()
	ops.writeFile = func(path string, data []byte, mode os.FileMode) error {
		if path == target {
			return boom
		}
		return os.WriteFile(path, data, mode)
	}
	if _, err := migrateLegacyBlobsToGit(root, objectRoot, ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ops = defaultLegacyBlobIO()
	ops.stat = func(string) (os.FileInfo, error) { return nil, boom }
	if mode := infoMode("x", ops); mode != 0644 {
		t.Fatal(mode)
	}
}
