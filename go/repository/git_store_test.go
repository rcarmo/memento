package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
)

type gitFixture struct {
	Files      map[string][]byte
	Base, Next string
	Changed    []string `json:"changed_paths"`
}

func fixtureGit(t *testing.T) (GitRepositoryPaths, gitFixture) {
	t.Helper()
	raw, err := os.ReadFile("../testdata/parity/repository-git.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture gitFixture
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	paths := GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}
	for path, data := range fixture.Files {
		path = filepath.Join(paths.BareDir, path)
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return paths, fixture
}
func TestGitPackedStoreReference(t *testing.T) {
	paths, f := fixtureGit(t)
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := GetMainRevision(paths); err != nil || got != f.Base {
		t.Fatal(got, err)
	}
	if data, err := store.ReadBlob(f.Next, "/a.md"); err != nil || string(data) != "second\n" {
		t.Fatal(string(data), err)
	}
	if data, err := store.ReadBlob(f.Next, "/link"); err != nil || string(data) != "a.md" {
		t.Fatal(string(data), err)
	}
	entries, err := store.TreeEntries(context.Background(), f.Next)
	if err != nil || len(entries) != 4 {
		t.Fatal(entries, err)
	}
	if entries[2].Mode != filemode.Executable {
		t.Fatal(entries)
	}
	if changed, err := DiffMainPaths(context.Background(), paths, f.Base, f.Next); err != nil || !reflect.DeepEqual(changed, f.Changed) {
		t.Fatal(changed, err, f.Changed)
	}
	if changed, err := store.DiffPaths(context.Background(), f.Next, f.Next); err != nil || len(changed) != 0 {
		t.Fatal(changed, err)
	}
	ok, err := PublishMainCompareAndSwap(paths, f.Base, f.Next)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if got, err := GetMainRevision(paths); err != nil || got != f.Next {
		t.Fatal(got, err)
	}
	if ok, err = PublishMainCompareAndSwap(paths, f.Base, f.Base); err != nil || ok {
		t.Fatal(ok, err)
	}
	if _, err = os.Stat(filepath.Join(paths.BareDir, "refs/heads/main.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
func TestGitCASContention(t *testing.T) {
	paths, f := fixtureGit(t)
	lock := filepath.Join(paths.BareDir, "refs/heads/main.lock")
	_ = os.MkdirAll(filepath.Dir(lock), 0700)
	if err := os.WriteFile(lock, []byte("competitor"), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := PublishMainCompareAndSwap(paths, f.Base, f.Next); err != nil || ok {
		t.Fatal(ok, err)
	}
	raw, _ := os.ReadFile(lock)
	if string(raw) != "competitor" {
		t.Fatal("foreign lock modified")
	}
	_ = os.Remove(lock)
	var workers sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for range 12 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ok, err := PublishMainCompareAndSwap(paths, f.Base, f.Next)
			if err != nil {
				t.Error(err)
			}
			if ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	workers.Wait()
	if wins != 1 {
		t.Fatal(wins)
	}
}
func TestGitCASNativeInterop(t *testing.T) {
	// Git is a test oracle only. CI installs it; no runtime code shells out.
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("Git oracle required", err)
	}
	paths, f := fixtureGit(t)
	run := func(args ...string) []byte {
		t.Helper()
		out, err := exec.Command(git, append([]string{"--git-dir", paths.BareDir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		return out
	}
	lock := filepath.Join(paths.BareDir, "refs/heads/main.lock")
	_ = os.MkdirAll(filepath.Dir(lock), 0700)
	_ = os.WriteFile(lock, nil, 0600)
	if err := exec.Command(git, "--git-dir", paths.BareDir, "update-ref", "refs/heads/main", f.Next, f.Base).Run(); err == nil {
		t.Fatal("Git ignored lock")
	}
	_ = os.Remove(lock)
	if ok, err := PublishMainCompareAndSwap(paths, f.Base, f.Next); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if got := strings.TrimSpace(string(run("rev-parse", "refs/heads/main"))); got != f.Next {
		t.Fatal(got)
	}
	run("update-ref", "refs/heads/main", f.Base, f.Next)
	run("fsck", "--strict", "--no-reflogs")
	if got, err := GetMainRevision(paths); err != nil || got != f.Base {
		t.Fatal(got, err)
	}
}

type failedGitFile struct {
	gitLockFile
	writeErr, syncErr, closeErr error
	short                       bool
}

func (f *failedGitFile) Write(p []byte) (int, error) {
	if f.short {
		return 1, nil
	}
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.gitLockFile.Write(p)
}
func (f *failedGitFile) Sync() error {
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.gitLockFile.Sync()
}
func (f *failedGitFile) Close() error {
	err := f.gitLockFile.Close()
	if f.closeErr != nil {
		return f.closeErr
	}
	return err
}
func TestGitCASFailures(t *testing.T) {
	for _, kind := range []string{"open", "write", "short", "sync", "close", "rename", "directory"} {
		t.Run(kind, func(t *testing.T) {
			paths, f := fixtureGit(t)
			ops := defaultGitPublishIO()
			open := ops.openLock
			ops.openLock = func(path string) (gitLockFile, error) {
				if kind == "open" {
					return nil, os.ErrPermission
				}
				file, err := open(path)
				if err != nil {
					return nil, err
				}
				wrapped := &failedGitFile{gitLockFile: file}
				switch kind {
				case "write":
					wrapped.writeErr = io.ErrClosedPipe
				case "short":
					wrapped.short = true
				case "sync":
					wrapped.syncErr = io.ErrClosedPipe
				case "close":
					wrapped.closeErr = io.ErrClosedPipe
				}
				return wrapped, nil
			}
			if kind == "rename" {
				ops.rename = func(string, string) error { return os.ErrPermission }
			}
			if kind == "directory" {
				ops.syncDir = func(string) error { return io.ErrClosedPipe }
			}
			ok, err := publishMain(paths, f.Base, f.Next, ops)
			if err == nil {
				t.Fatal("failure ignored")
			}
			got, _ := GetMainRevision(paths)
			if kind == "directory" {
				var outcome *PublishOutcomeError
				if !ok || !errors.As(err, &outcome) || outcome.Unwrap() == nil || outcome.Error() == "" || got != f.Next {
					t.Fatal(ok, got, err)
				}
			} else if ok || got != f.Base {
				t.Fatal(ok, got, err)
			}
			if _, err = os.Stat(filepath.Join(paths.BareDir, "refs/heads/main.lock")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("lock leaked", err)
			}
		})
	}
}

func encodeObject(t *testing.T, s storage.Storer, value interface {
	Encode(plumbing.EncodedObject) error
}) plumbing.Hash {
	t.Helper()
	encoded := s.NewEncodedObject()
	if err := value.Encode(encoded); err != nil {
		t.Fatal(err)
	}
	hash, err := s.SetEncodedObject(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}
func TestGitStoreMalformedAndMissing(t *testing.T) {
	paths, f := fixtureGit(t)
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, revision := range []string{"bad", strings.Repeat("g", 40), strings.Repeat("1", 40)} {
		if _, err := store.TreeEntries(context.Background(), revision); err == nil {
			t.Fatal(revision)
		}
		if _, err := store.ReadBlob(revision, "/a"); err == nil {
			t.Fatal(revision)
		}
	}
	for _, path := range []string{"relative", "/", "/missing", "/bin"} {
		if _, err := store.ReadBlob(f.Next, path); err == nil {
			t.Fatal(path)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.TreeEntries(ctx, f.Next); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := store.DiffPaths(context.Background(), "bad", f.Next); err == nil {
		t.Fatal("bad base")
	}
	if _, err := store.DiffPaths(context.Background(), f.Base, "bad"); err == nil {
		t.Fatal("bad end")
	}
	if _, err := OpenGitStore("/nonexistent-memento-oracle"); err == nil {
		t.Fatal("missing store")
	}
	if _, err := GetMainRevision(GitRepositoryPaths{}); err == nil {
		t.Fatal("missing repo")
	}
	if _, err := DiffMainPaths(context.Background(), GitRepositoryPaths{}, f.Base, f.Next); err == nil {
		t.Fatal("missing diff")
	}
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "objects"), nil, 0600)
	if _, err := OpenGitStore(root); err == nil {
		t.Fatal("file object dir")
	}
	m := memory.NewStorage()
	badTree := encodeObject(t, m, &object.Tree{Entries: []object.TreeEntry{{Name: "..", Mode: filemode.Regular, Hash: plumbing.ZeroHash}}})
	revision := encodeObject(t, m, &object.Commit{TreeHash: badTree})
	broken := &GitStore{storage: m}
	if _, err := broken.TreeEntries(context.Background(), revision.String()); err == nil {
		t.Fatal("unsafe tree")
	}
	for _, mode := range []filemode.FileMode{filemode.Dir, filemode.Regular} {
		tree := encodeObject(t, m, &object.Tree{Entries: []object.TreeEntry{{Name: "missing", Mode: mode, Hash: plumbing.NewHash(strings.Repeat("1", 40))}}})
		commit := encodeObject(t, m, &object.Commit{TreeHash: tree})
		if _, err := broken.ReadBlob(commit.String(), "/missing"); err == nil {
			t.Fatal(mode)
		}
		if mode == filemode.Dir {
			if _, err := broken.TreeEntries(context.Background(), commit.String()); err == nil {
				t.Fatal("missing subtree silently ignored")
			}
		}
	}
	missingTree := encodeObject(t, m, &object.Commit{TreeHash: plumbing.NewHash(strings.Repeat("2", 40))})
	if _, err := broken.ReadBlob(missingTree.String(), "/a"); err == nil {
		t.Fatal("missing tree")
	}
}

type failingBlobObject struct{ plumbing.EncodedObject }

func (o failingBlobObject) Reader() (io.ReadCloser, error) { return nil, io.ErrClosedPipe }

type failBlobStorage struct{ storage.Storer }

func (s failBlobStorage) EncodedObject(kind plumbing.ObjectType, hash plumbing.Hash) (plumbing.EncodedObject, error) {
	value, err := s.Storer.EncodedObject(kind, hash)
	if err == nil && kind == plumbing.BlobObject {
		return failingBlobObject{value}, nil
	}
	return value, err
}
func TestGitRemainingReadFailures(t *testing.T) {
	paths, f := fixtureGit(t)
	s, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	s.storage = failBlobStorage{s.storage}
	if _, err = s.ReadBlob(f.Next, "/a.md"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	cause := io.ErrClosedPipe
	e := &GitError{Message: "failure", Cause: cause}
	if e.Error() != "failure" || !errors.Is(e, cause) {
		t.Fatal(e)
	}
	if err = syncDirectory(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing sync directory")
	}
	m := memory.NewStorage()
	tree := encodeObject(t, m, &object.Tree{})
	for range 1026 {
		tree = encodeObject(t, m, &object.Tree{Entries: []object.TreeEntry{{Name: "nested", Mode: filemode.Dir, Hash: tree}}})
	}
	revision := encodeObject(t, m, &object.Commit{TreeHash: tree})
	s = &GitStore{storage: m}
	if _, err = s.TreeEntries(context.Background(), revision.String()); errorText(err) != "Git tree depth limit" {
		t.Fatal(err)
	}
}

func TestGitCASValidationFailures(t *testing.T) {
	paths, f := fixtureGit(t)
	for _, pair := range [][2]string{{"bad", f.Next}, {f.Base, "bad"}, {f.Base, plumbing.ZeroHash.String()}, {f.Base, strings.Repeat("1", 40)}} {
		if _, err := PublishMainCompareAndSwap(paths, pair[0], pair[1]); err == nil {
			t.Fatal(pair)
		}
	}
	if _, err := PublishMainCompareAndSwap(GitRepositoryPaths{}, f.Base, f.Next); err == nil {
		t.Fatal("missing store")
	}
	for _, kind := range []string{"symbolic", "reflog", "reflog-parent", "refs-parent", "missing-main", "config-log", "config-format", "config-bad"} {
		t.Run(kind, func(t *testing.T) {
			paths, f := fixtureGit(t)
			ref := filepath.Join(paths.BareDir, "refs/heads/main")
			write := func(path, content string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "symbolic":
				write(ref, "ref: refs/heads/candidate\n")
			case "reflog":
				write(filepath.Join(paths.BareDir, "logs/refs/heads/main"), "history")
			case "reflog-parent":
				write(filepath.Join(paths.BareDir, "logs"), "file")
			case "refs-parent":
				write(filepath.Join(paths.BareDir, "refs"), "file")
			case "missing-main":
				_ = os.Remove(filepath.Join(paths.BareDir, "packed-refs"))
			case "config-log":
				write(filepath.Join(paths.BareDir, "config"), "[core]\n bare = true\n logAllRefUpdates = always\n")
			case "config-format":
				write(filepath.Join(paths.BareDir, "config"), "[core]\n bare = true\n[extensions]\n refStorage = reftable\n")
			case "config-bad":
				write(filepath.Join(paths.BareDir, "config"), "[bad\n")
			}
			base := f.Base
			if kind == "missing-main" {
				base = plumbing.ZeroHash.String()
			}
			ok, err := PublishMainCompareAndSwap(paths, base, f.Next)
			if kind == "missing-main" {
				if err != nil || !ok {
					t.Fatal(ok, err)
				}
			} else if err == nil || ok {
				t.Fatal(ok, err)
			}
		})
	}
}

func TestGitCASRaceWithNativeWriter(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for range 8 {
		paths, f := fixtureGit(t)
		ready := make(chan struct{})
		result := make(chan error, 1)
		go func() {
			<-ready
			result <- exec.Command(git, "--git-dir", paths.BareDir, "-c", "core.filesRefLockTimeout=0", "update-ref", "refs/heads/main", f.Next, f.Base).Run()
		}()
		close(ready)
		ok, err := PublishMainCompareAndSwap(paths, f.Base, f.Next)
		nativeErr := <-result
		if err != nil {
			t.Fatal(err)
		}
		if ok == (nativeErr == nil) {
			t.Fatal("exactly one writer must publish", ok, nativeErr)
		}
		got, err := GetMainRevision(paths)
		if err != nil || got != f.Next {
			t.Fatal(got, err)
		}
	}
}

func TestGitCASSymlinkAndRefReadError(t *testing.T) {
	paths, f := fixtureGit(t)
	ref := filepath.Join(paths.BareDir, "refs/heads/main")
	_ = os.MkdirAll(filepath.Dir(ref), 0700)
	if err := os.Symlink("../../packed-refs", ref); err != nil {
		t.Fatal(err)
	}
	if ok, err := PublishMainCompareAndSwap(paths, f.Base, f.Next); ok || err == nil {
		t.Fatal(ok, err)
	}
	_ = os.Remove(ref)
	ops := defaultGitPublishIO()
	open := ops.openLock
	// Swap the parent to a symlink loop after opening the owned lock to
	// exercise a filesystem error at the post-lock reference check.
	ops.openLock = func(path string) (gitLockFile, error) {
		file, err := open(path)
		if err != nil {
			return nil, err
		}
		parent := filepath.Dir(path)
		if err = os.Rename(parent, parent+"-saved"); err != nil {
			t.Fatal(err)
		}
		if err = os.Symlink("heads", parent); err != nil {
			t.Fatal(err)
		}
		return file, nil
	}
	if ok, err := publishMain(paths, f.Base, f.Next, ops); ok || err == nil {
		t.Fatal(ok, err)
	}
}
