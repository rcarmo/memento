package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func bootstrapPaths(t *testing.T) GitRepositoryPaths {
	root := t.TempDir()
	return GitRepositoryPaths{filepath.Join(root, "repo.git"), filepath.Join(root, "current"), filepath.Join(root, "worktrees")}
}
func TestBootstrapReferenceHash(t *testing.T) {
	_, f := fixtureGit(t)
	paths := bootstrapPaths(t)
	seed := t.TempDir()
	_ = os.Mkdir(filepath.Join(seed, "bin"), 0700)
	for name, content := range map[string]string{"a.md": "first\n", "remove.md": "removed\n", "bin/run": "#!/bin/sh\nexit 0\n"} {
		mode := fs.FileMode(0644)
		if name == "bin/run" {
			mode = 0755
		}
		if err := os.WriteFile(filepath.Join(seed, name), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	result, err := bootstrapRepository(context.Background(), paths, seed, time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), defaultBootstrapIO())
	if err != nil || result.Revision != f.Base || !reflect.DeepEqual(result.ChangedPaths, []string{"/a.md", "/bin/run", "/remove.md"}) {
		t.Fatal(result, err, f.Base)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := exec.Command(git, "--git-dir", paths.BareDir, "fsck", "--strict").CombinedOutput(); err != nil {
		t.Fatal(string(raw), err)
	}
	if _, err = BootstrapRepository(context.Background(), paths, seed); err == nil {
		t.Fatal("existing main overwritten")
	}
	if _, err = os.Stat(filepath.Join(paths.WorktreesDir, ".bootstrap")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("bootstrap leaked")
	}
	empty := bootstrapPaths(t)
	result, err = BootstrapRepository(context.Background(), empty, "")
	if err != nil || len(result.ChangedPaths) != 0 {
		t.Fatal(result, err)
	}
	entries, err := os.ReadDir(empty.CurrentDir)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}
func TestBootstrapSeeds(t *testing.T) {
	for _, kind := range []string{"file-link", "directory-link", "git-metadata", "special", "external-link", "missing"} {
		paths := bootstrapPaths(t)
		seed := t.TempDir()
		_ = os.WriteFile(filepath.Join(seed, "file"), []byte("x"), 0600)
		_ = os.Mkdir(filepath.Join(seed, "dir"), 0700)
		switch kind {
		case "file-link":
			_ = os.Symlink("file", filepath.Join(seed, "link"))
		case "directory-link":
			_ = os.Symlink("dir", filepath.Join(seed, "linked"))
		case "git-metadata":
			_ = os.WriteFile(filepath.Join(seed, ".git"), nil, 0600)
		case "special":
			_ = syscall.Mkfifo(filepath.Join(seed, "fifo"), 0600)
		case "external-link":
			_ = os.Symlink("/etc/passwd", filepath.Join(seed, "escape"))
		case "missing":
			seed += "-absent"
		}
		result, err := BootstrapRepository(context.Background(), paths, seed)
		if kind == "file-link" || kind == "directory-link" {
			if err != nil {
				t.Fatal(err)
			}
			if kind == "file-link" {
				info, err := os.Lstat(filepath.Join(paths.CurrentDir, "link"))
				if err != nil || !info.Mode().IsRegular() {
					t.Fatal(info, err)
				}
			}
			if len(result.ChangedPaths) == 0 {
				t.Fatal(result)
			}
		} else if err == nil {
			t.Fatal(kind)
		}
	}
}
func TestBootstrapBasicFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BootstrapRepository(ctx, bootstrapPaths(t), ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := BootstrapRepository(context.Background(), GitRepositoryPaths{}, ""); err == nil {
		t.Fatal("empty paths")
	}
	paths := bootstrapPaths(t)
	paths.WorktreesDir = paths.BareDir
	if _, err := BootstrapRepository(context.Background(), paths, ""); err == nil {
		t.Fatal("overlap")
	}
	paths = bootstrapPaths(t)
	_ = os.MkdirAll(paths.BareDir, 0700)
	_ = os.WriteFile(filepath.Join(paths.BareDir, "objects"), nil, 0600)
	if _, err := BootstrapRepository(context.Background(), paths, ""); err == nil {
		t.Fatal("invalid object dir")
	}
	for _, kind := range []string{"mkdirall", "mkdir", "write", "copy", "publish-error", "publish-conflict", "materialize"} {
		paths := bootstrapPaths(t)
		ops := defaultBootstrapIO()
		switch kind {
		case "mkdirall":
			ops.mkdirAll = func(string, fs.FileMode) error { return os.ErrPermission }
		case "mkdir":
			ops.mkdir = func(string, fs.FileMode) error { return os.ErrPermission }
		case "write":
			ops.writeFile = func(string, []byte, fs.FileMode) error { return os.ErrPermission }
		case "copy":
			ops.copySeed = func(context.Context, string, string) error { return io.ErrClosedPipe }
		case "publish-error":
			ops.publish = func(GitRepositoryPaths, string, string) (bool, error) { return false, io.ErrClosedPipe }
		case "publish-conflict":
			ops.publish = func(GitRepositoryPaths, string, string) (bool, error) { return false, nil }
		case "materialize":
			ops.materialize = func(context.Context, GitRepositoryPaths, string) (MaterializedCheckout, error) {
				return MaterializedCheckout{}, io.ErrClosedPipe
			}
		}
		seed := ""
		if kind == "copy" {
			seed = "seed"
		}
		if _, err := bootstrapRepository(context.Background(), paths, seed, time.Unix(0, 0).UTC(), ops); err == nil {
			t.Fatal(kind)
		}
	}
	if got := cleanCommitMessage("\n\n# comment\n\n\nbody  \r\n"); got != "# comment\n\nbody\n" {
		t.Fatal(got)
	}
}

func TestBootstrapInjectedBoundaries(t *testing.T) {
	for _, kind := range []string{"abs-1", "abs-2", "ancestor-1", "ancestor-2", "metadata-stat", "reopen", "root", "stage", "tree", "commit", "existing-config", "bad-main"} {
		t.Run(kind, func(t *testing.T) {
			paths := bootstrapPaths(t)
			ops := defaultBootstrapIO()
			calls := 0
			switch kind {
			case "abs-1", "abs-2":
				abs := ops.abs
				ops.abs = func(p string) (string, error) {
					calls++
					if fmt.Sprint("abs-", calls) == kind {
						return "", os.ErrPermission
					}
					return abs(p)
				}
			case "ancestor-1", "ancestor-2", "metadata-stat":
				stat := ops.lstat
				ops.lstat = func(p string) (fs.FileInfo, error) {
					if strings.HasSuffix(p, "HEAD") || strings.HasSuffix(p, "config") {
						if kind == "metadata-stat" {
							return nil, os.ErrPermission
						}
					}
					if p == filepath.Dir(paths.BareDir) {
						calls++
						if fmt.Sprint("ancestor-", calls) == kind {
							return nil, os.ErrPermission
						}
					}
					return stat(p)
				}
			case "reopen":
				ops.openStore = func(string) (*GitStore, error) {
					calls++
					if calls == 1 {
						return nil, os.ErrNotExist
					}
					return nil, io.ErrClosedPipe
				}
			case "root":
				ops.openRoot = func(string) (*os.Root, error) { return nil, os.ErrPermission }
			case "stage":
				ops.copySeed = func(_ context.Context, _, target string) error {
					return syscall.Mkfifo(filepath.Join(target, "fifo"), 0600)
				}
			case "tree", "commit":
				open := ops.openStore
				ops.openStore = func(p string) (*GitStore, error) {
					s, err := open(p)
					if err != nil {
						return nil, err
					}
					if kind == "tree" {
						s.storage = &objectFaultStorage{Storer: s.storage, fail: "store"}
					} else {
						s.storage = &commitFailStore{Storer: s.storage}
					}
					return s, nil
				}
			case "existing-config":
				_ = os.MkdirAll(paths.BareDir, 0700)
				_ = os.WriteFile(filepath.Join(paths.BareDir, "config"), []byte("[core]\n bare = true\n"), 0600)
				ops.publish = func(GitRepositoryPaths, string, string) (bool, error) { return false, io.ErrClosedPipe }
			case "bad-main":
				_ = os.MkdirAll(filepath.Join(paths.BareDir, "objects"), 0700)
				_ = os.MkdirAll(filepath.Join(paths.BareDir, "refs/heads"), 0700)
				_ = os.WriteFile(filepath.Join(paths.BareDir, "refs/heads/main"), []byte("ref: refs/heads/other\n"), 0600)
			}
			seed := ""
			if kind == "stage" {
				seed = "ignored"
			}
			if _, err := bootstrapRepository(context.Background(), paths, seed, time.Unix(0, 0).UTC(), ops); err == nil {
				t.Fatal(kind)
			}
		})
	}
	if err := copyBootstrapSeed(context.Background(), t.TempDir(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing destination")
	}
}

type commitFailStore struct{ storage.Storer }

func (s *commitFailStore) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	if o.Type() == plumbing.CommitObject {
		return plumbing.ZeroHash, io.ErrClosedPipe
	}
	return s.Storer.SetEncodedObject(o)
}

type faultSeedFS struct {
	seedFS
	fail string
}

func (s faultSeedFS) open(name string) (seedFile, error) {
	if s.fail == "open" {
		return nil, io.ErrClosedPipe
	}
	f, err := s.seedFS.open(name)
	if err != nil {
		return nil, err
	}
	return faultSeedFile{f, s.fail}, nil
}
func (s faultSeedFS) create(name string, mode fs.FileMode) (seedFile, error) {
	if s.fail == "create" {
		return nil, io.ErrClosedPipe
	}
	f, err := s.seedFS.create(name, mode)
	if err != nil {
		return nil, err
	}
	return faultSeedFile{f, s.fail}, nil
}
func (s faultSeedFS) MkdirAll(p string, m fs.FileMode) error {
	if s.fail == "mkdir" {
		return io.ErrClosedPipe
	}
	return s.seedFS.MkdirAll(p, m)
}
func (s faultSeedFS) Chmod(p string, m fs.FileMode) error {
	if s.fail == "chmod" {
		return io.ErrClosedPipe
	}
	return s.seedFS.Chmod(p, m)
}

type faultSeedFile struct {
	seedFile
	fail string
}

func (f faultSeedFile) ReadDir(n int) ([]os.DirEntry, error) {
	if f.fail == "readdir" {
		return nil, io.ErrClosedPipe
	}
	return f.seedFile.ReadDir(n)
}
func (f faultSeedFile) Read(p []byte) (int, error) {
	if f.fail == "read" {
		return 0, io.ErrClosedPipe
	}
	return f.seedFile.Read(p)
}
func (f faultSeedFile) Close() error {
	err := f.seedFile.Close()
	if f.fail == "close" {
		return io.ErrClosedPipe
	}
	return err
}
func TestBootstrapSeedIOFailures(t *testing.T) {
	source := t.TempDir()
	_ = os.WriteFile(filepath.Join(source, "file"), []byte("text"), 0600)
	_ = os.Mkdir(filepath.Join(source, "dir"), 0700)
	_ = os.WriteFile(filepath.Join(source, "dir/nested"), []byte("x"), 0600)
	in, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	info, err := in.Stat("file")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"open", "readdir", "close", "mkdir"} {
		out, err := os.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		input := faultSeedFS{seedOSRoot{in}, kind}
		output := faultSeedFS{seedOSRoot{out}, kind}
		if err = copySeedTree(context.Background(), input, output, "."); err == nil {
			t.Fatal(kind)
		}
		out.Close()
	}
	for _, kind := range []string{"open", "create", "read", "close", "chmod"} {
		out, err := os.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		input := faultSeedFS{seedOSRoot{in}, kind}
		output := faultSeedFS{seedOSRoot{out}, kind}
		if err = copySeedFile(input, output, "file", info); err == nil {
			t.Fatal(kind)
		}
		out.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = copySeedTree(ctx, seedOSRoot{in}, seedOSRoot{in}, "."); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// Nested copy errors propagate, including the source's special-file boundary.
	_ = syscall.Mkfifo(filepath.Join(source, "dir/fifo"), 0600)
	if err = copyBootstrapSeed(context.Background(), source, t.TempDir()); err == nil {
		t.Fatal("nested error")
	}
	out, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err = copySeedTree(context.Background(), seedOSRoot{in}, faultSeedFS{seedOSRoot{out}, "create"}, "."); err == nil {
		t.Fatal("copy error")
	}
}
