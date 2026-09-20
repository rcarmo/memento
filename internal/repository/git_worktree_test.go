package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
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

func TestDetachedWorktreeReference(t *testing.T) {
	paths, f := fixtureGit(t)
	ctx := context.Background()
	work, err := CreateOperationWorktree(ctx, paths, "synthetic-op", f.Base)
	if err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		raw, err := exec.Command(git, append([]string{"-C", work.Path}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(args, string(raw), err)
		}
		return strings.TrimSpace(string(raw))
	}
	if got := run("rev-parse", "HEAD"); got != f.Base {
		t.Fatal(got)
	}
	if got := run("status", "--porcelain"); got != "" {
		t.Fatal(got)
	}
	if err = os.WriteFile(filepath.Join(work.Path, "a.md"), []byte("second\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(work.Path, "remove.md"))
	_ = os.WriteFile(filepath.Join(work.Path, "added.md"), []byte("added\n"), 0600)
	_ = os.Symlink("a.md", filepath.Join(work.Path, "link"))
	_ = os.WriteFile(filepath.Join(work.Path, "untracked.md"), []byte("not committed"), 0600)
	stamp := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	identity := GitCommitIdentity{"Synthetic Agent", "test@example.invalid", stamp}
	staged, err := CommitExactPaths(ctx, work, []string{"/a.md", "/added.md", "/remove.md", "/link"}, "synthetic commit", identity, identity)
	if err != nil || staged.Revision != f.Next {
		t.Fatal(staged, err, f.Next)
	}
	if got, err := ResolveWorktreeRevision(work.Path); err != nil || got != f.Next {
		t.Fatal(got, err)
	}
	if got, err := GetMainRevision(paths); err != nil || got != f.Base {
		t.Fatal("main moved before CAS", got, err)
	}
	if got := run("status", "--porcelain"); got != "?? untracked.md" {
		t.Fatal(got)
	}
	if stagedPaths, err := ExactStagedPaths(work.Path); err != nil || len(stagedPaths) != 0 {
		t.Fatal(stagedPaths, err)
	}
	run("fsck", "--strict", "--no-reflogs")
	if ok, err := PublishMainCompareAndSwap(paths, f.Base, f.Next); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err = RemoveOperationWorktree(paths, "synthetic-op"); err != nil {
		t.Fatal(err)
	}
	if got, err := ResolveWorktreeRevision(work.Path); err != nil || got != "" {
		t.Fatal(got, err)
	}
	if err = RemoveOperationWorktree(paths, "synthetic-op"); err != nil {
		t.Fatal(err)
	}
}

func TestWorktreePathsStagingAndConflicts(t *testing.T) {
	paths, f := fixtureGit(t)
	ctx := context.Background()
	work, err := CreateOperationWorktree(ctx, paths, "op", f.Base)
	if err != nil {
		t.Fatal(err)
	}
	identity := GitCommitIdentity{"Test", "test@example.invalid", time.Unix(0, 0).UTC()}
	if _, err = CreateOperationWorktree(ctx, paths, "op", f.Base); err == nil {
		t.Fatal("collision")
	}
	for _, id := range []string{"", "..", "a/b", "x\\y"} {
		if _, err = CreateOperationWorktree(ctx, paths, id, f.Base); err == nil {
			t.Fatal(id)
		}
		if err = RemoveOperationWorktree(paths, id); err == nil {
			t.Fatal(id)
		}
	}
	for _, changed := range [][]string{nil, {"/missing"}, {"/../bad"}, {"/"}, {"/.git"}, {"/a.md"}} {
		if _, err = CommitExactPaths(ctx, work, changed, "msg", identity, identity); err == nil {
			t.Fatal(changed)
		}
	}
	admin, _, err := worktreeAdmin(work.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, lock := range []string{"index.lock", "HEAD.lock"} {
		p := filepath.Join(admin, lock)
		_ = os.WriteFile(p, []byte("other"), 0600)
		if _, err = CommitExactPaths(ctx, work, []string{"/a.md"}, "msg", identity, identity); err == nil {
			t.Fatal(lock)
		}
		_ = os.Remove(p)
	}
	// Stage a directory, preserving executable mode and excluding an unrelated
	// change. The existing tracked file inside the directory is removed.
	_ = os.Remove(filepath.Join(work.Path, "bin/run"))
	_ = os.WriteFile(filepath.Join(work.Path, "bin/new"), []byte("new"), 0755)
	_ = os.WriteFile(filepath.Join(work.Path, "a.md"), []byte("unstaged"), 0600)
	result, err := CommitExactPaths(ctx, work, []string{"/bin"}, "\n message  \n\n\nbody\t\n", identity, identity)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := store.commit(result.Revision)
	if err != nil || commit.Message != " message\n\nbody\n" {
		t.Fatal(commit, err)
	}
	got, err := store.ReadBlob(result.Revision, "/a.md")
	if err != nil || string(got) != "first\n" {
		t.Fatal(string(got), err)
	}
	diff, err := store.DiffPaths(ctx, f.Base, result.Revision)
	if err != nil || !reflect.DeepEqual(diff, []string{"/bin/new", "/bin/run"}) {
		t.Fatal(diff, err)
	}
	if _, err = CommitExactPaths(ctx, work, []string{"/bin"}, "msg", identity, identity); err == nil {
		t.Fatal("empty commit")
	}
}

func TestWorktreeBasicFailures(t *testing.T) {
	if _, err := CreateOperationWorktree(context.Background(), GitRepositoryPaths{}, "op", strings.Repeat("1", 40)); err == nil {
		t.Fatal("no repo")
	}
	paths, f := fixtureGit(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CreateOperationWorktree(ctx, paths, "op", f.Base); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := CreateOperationWorktree(context.Background(), paths, "op", "bad"); err == nil {
		t.Fatal("bad base")
	}
	if _, err := ResolveWorktreeRevision(t.TempDir()); err == nil {
		t.Fatal("missing gitdir")
	}
	if _, err := ExactStagedPaths(t.TempDir()); err == nil {
		t.Fatal("missing staged")
	}
	if err := writeAndSync(&failedGitFile{gitLockFile: &memoryLock{}, short: true}, []byte("value")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if err := writeAndSync(&failedGitFile{gitLockFile: &memoryLock{}, writeErr: io.ErrClosedPipe}, []byte("value")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

type memoryLock struct{}

func (*memoryLock) Write(p []byte) (int, error) { return len(p), nil }
func (*memoryLock) Sync() error                 { return nil }
func (*memoryLock) Close() error                { return nil }

func TestWorktreeCreationInjectedFailures(t *testing.T) {
	for _, kind := range []string{"abs-1", "abs-2", "lstat", "mkdirall-1", "mkdirall-2", "mkdir", "checkout", "index", "metadata"} {
		t.Run(kind, func(t *testing.T) {
			paths, f := fixtureGit(t)
			ops := defaultWorktreeIO()
			calls := 0
			switch kind {
			case "abs-1", "abs-2":
				orig := ops.abs
				ops.abs = func(p string) (string, error) {
					calls++
					if kind == fmt.Sprint("abs-", calls) {
						return "", os.ErrPermission
					}
					return orig(p)
				}
			case "lstat":
				ops.lstat = func(string) (fs.FileInfo, error) { return nil, os.ErrPermission }
			case "mkdirall-1", "mkdirall-2":
				orig := ops.mkdirAll
				ops.mkdirAll = func(p string, m fs.FileMode) error {
					calls++
					if kind == fmt.Sprint("mkdirall-", calls) {
						return os.ErrPermission
					}
					return orig(p, m)
				}
			case "mkdir":
				ops.mkdir = func(string, fs.FileMode) error { return os.ErrPermission }
			case "checkout":
				paths.WorktreesDir = paths.BareDir
			case "index":
				ops.encodeIndex = func([]GitTreeEntry) ([]byte, error) { return nil, io.ErrClosedPipe }
			case "metadata":
				ops.writeFile = func(string, []byte, fs.FileMode) error { return io.ErrClosedPipe }
			}
			if _, err := createOperationWorktree(context.Background(), paths, "op", f.Base, ops); err == nil {
				t.Fatal(kind)
			}
		})
	}
}
func TestWorktreeRemovalInjectedFailures(t *testing.T) {
	for _, kind := range []string{"lstat", "abs-1", "abs-2", "abs-3", "abs-4", "remove-1", "remove-2", "mismatch", "bad-gitdir"} {
		t.Run(kind, func(t *testing.T) {
			paths, f := fixtureGit(t)
			work, err := CreateOperationWorktree(context.Background(), paths, "op", f.Base)
			if err != nil {
				t.Fatal(err)
			}
			ops := defaultWorktreeIO()
			calls := 0
			switch kind {
			case "lstat":
				ops.lstat = func(string) (fs.FileInfo, error) { return nil, os.ErrPermission }
			case "abs-1", "abs-2", "abs-3", "abs-4":
				orig := ops.abs
				ops.abs = func(p string) (string, error) {
					calls++
					if kind == fmt.Sprint("abs-", calls) {
						return "", os.ErrPermission
					}
					return orig(p)
				}
			case "remove-1", "remove-2":
				orig := ops.removeAll
				ops.removeAll = func(p string) error {
					calls++
					if kind == fmt.Sprint("remove-", calls) {
						return os.ErrPermission
					}
					return orig(p)
				}
			case "mismatch":
				ops.abs = func(p string) (string, error) {
					calls++
					if calls == 1 {
						return "/wrong", nil
					}
					return filepath.Abs(p)
				}
			case "bad-gitdir":
				_ = os.WriteFile(filepath.Join(work.Path, ".git"), []byte("invalid"), 0600)
			}
			if err = removeOperationWorktree(paths, "op", ops); err == nil {
				t.Fatal(kind)
			}
		})
	}
}
func TestWorktreeCommitInjectedFailures(t *testing.T) {
	for _, kind := range []string{"open-root", "index-encode", "index-write", "head-write", "index-close", "head-close", "index-rename", "head-rename", "sync"} {
		t.Run(kind, func(t *testing.T) {
			paths, f := fixtureGit(t)
			work, err := CreateOperationWorktree(context.Background(), paths, "op", f.Base)
			if err != nil {
				t.Fatal(err)
			}
			_ = os.WriteFile(filepath.Join(work.Path, "a.md"), []byte("changed"), 0600)
			ops := defaultWorktreeIO()
			identity := GitCommitIdentity{"Test", "test@example.invalid", time.Unix(0, 0).UTC()}
			switch kind {
			case "open-root":
				ops.openRoot = func(string) (*os.Root, error) { return nil, os.ErrPermission }
			case "index-encode":
				ops.encodeIndex = func([]GitTreeEntry) ([]byte, error) { return nil, io.ErrClosedPipe }
			case "index-write", "head-write", "index-close", "head-close":
				open := ops.lock
				ops.lock = func(p string) (gitLockFile, error) {
					file, err := open(p)
					if err != nil {
						return nil, err
					}
					wrapped := &failedGitFile{gitLockFile: file}
					match := strings.Contains(p, "index.lock") && strings.HasPrefix(kind, "index") || strings.Contains(p, "HEAD.lock") && strings.HasPrefix(kind, "head")
					if match {
						if strings.HasSuffix(kind, "write") {
							wrapped.writeErr = io.ErrClosedPipe
						} else {
							wrapped.closeErr = io.ErrClosedPipe
						}
					}
					return wrapped, nil
				}
			case "index-rename", "head-rename":
				rename := ops.rename
				ops.rename = func(a, b string) error {
					if strings.HasSuffix(a, "index.lock") && kind == "index-rename" || strings.HasSuffix(a, "HEAD.lock") && kind == "head-rename" {
						return os.ErrPermission
					}
					return rename(a, b)
				}
			case "sync":
				ops.syncDir = func(string) error { return io.ErrClosedPipe }
			}
			if _, err = commitExactPaths(context.Background(), work, []string{"/a.md"}, "message", identity, identity, ops); err == nil {
				t.Fatal(kind)
			}
			if got, err := GetMainRevision(paths); err != nil || got != f.Base {
				t.Fatal("failed worktree changed main", got, err)
			}
			head, _ := ResolveWorktreeRevision(work.Path)
			if kind == "sync" {
				if head == f.Base {
					t.Fatal("head should have advanced")
				}
			} else if head != f.Base {
				t.Fatal(kind, head)
			}
			if kind == "head-rename" {
				staged, err := ExactStagedPaths(work.Path)
				if err != nil || !reflect.DeepEqual(staged, []string{"/a.md"}) {
					t.Fatal(staged, err)
				}
			}
		})
	}
}

func TestWorktreeMalformedMetadata(t *testing.T) {
	for _, kind := range []string{"head-missing", "head-invalid", "head-object-missing", "index-missing", "index-corrupt", "index-unmerged", "common-missing", "store-missing", "gitdir-invalid", "relative-gitdir", "absolute-common"} {
		t.Run(kind, func(t *testing.T) {
			paths, f := fixtureGit(t)
			work, err := CreateOperationWorktree(context.Background(), paths, "op", f.Base)
			if err != nil {
				t.Fatal(err)
			}
			admin, _, _ := worktreeAdmin(work.Path)
			switch kind {
			case "head-missing":
				_ = os.Remove(filepath.Join(admin, "HEAD"))
			case "head-invalid":
				_ = os.WriteFile(filepath.Join(admin, "HEAD"), []byte("ref: refs/heads/main"), 0600)
			case "head-object-missing":
				_ = os.WriteFile(filepath.Join(admin, "HEAD"), []byte(strings.Repeat("1", 40)), 0600)
			case "index-missing":
				_ = os.Remove(filepath.Join(admin, "index"))
			case "index-corrupt":
				_ = os.WriteFile(filepath.Join(admin, "index"), []byte("bad"), 0600)
			case "index-unmerged":
				var raw bytes.Buffer
				idx := &index.Index{Version: 2, Entries: []*index.Entry{{Name: "a.md", Mode: filemode.Regular, Stage: 1}}}
				if err = index.NewEncoder(&raw).Encode(idx); err != nil {
					t.Fatal(err)
				}
				_ = os.WriteFile(filepath.Join(admin, "index"), raw.Bytes(), 0600)
			case "common-missing":
				_ = os.Remove(filepath.Join(admin, "commondir"))
			case "store-missing":
				_ = os.WriteFile(filepath.Join(admin, "commondir"), []byte(t.TempDir()), 0600)
			case "gitdir-invalid":
				_ = os.WriteFile(filepath.Join(work.Path, ".git"), []byte("bad"), 0600)
			case "relative-gitdir":
				rel, err := filepath.Rel(work.Path, admin)
				if err != nil {
					t.Fatal(err)
				}
				_ = os.WriteFile(filepath.Join(work.Path, ".git"), []byte("gitdir: "+rel), 0600)
			case "absolute-common":
				_ = os.WriteFile(filepath.Join(admin, "commondir"), []byte(paths.BareDir), 0600)
			}
			_, stageErr := ExactStagedPaths(work.Path)
			if kind == "relative-gitdir" || kind == "absolute-common" {
				if stageErr != nil {
					t.Fatal(stageErr)
				}
				return
			}
			if stageErr == nil {
				t.Fatal("invalid metadata accepted")
			}
			_, _ = ResolveWorktreeRevision(work.Path)
			identity := GitCommitIdentity{"Test", "test@example.invalid", time.Unix(0, 0).UTC()}
			if _, err = CommitExactPaths(context.Background(), work, []string{"/a.md"}, "message", identity, identity); err == nil {
				t.Fatal("invalid commit accepted")
			}
		})
	}
	root := t.TempDir()
	_ = os.Symlink("loop", filepath.Join(root, "loop"))
	if _, err := ResolveWorktreeRevision(filepath.Join(root, "loop")); err == nil {
		t.Fatal("stat loop")
	}
	before := []GitTreeEntry{{Path: "deleted", Mode: filemode.Regular}}
	if got := changedTreePaths(before, nil); !reflect.DeepEqual(got, []string{"/deleted"}) {
		t.Fatal(got)
	}
}

type stageFaultRoot struct {
	*os.Root
	fail string
}

func (r stageFaultRoot) Lstat(name string) (fs.FileInfo, error) {
	if r.fail == "lstat" {
		return nil, os.ErrPermission
	}
	return r.Root.Lstat(name)
}
func (r stageFaultRoot) ReadFile(name string) ([]byte, error) {
	if r.fail == "read" {
		return nil, io.ErrClosedPipe
	}
	return r.Root.ReadFile(name)
}
func (r stageFaultRoot) Readlink(name string) (string, error) {
	if r.fail == "link" {
		return "", io.ErrClosedPipe
	}
	return r.Root.Readlink(name)
}
func (r stageFaultRoot) readDir(name string) ([]os.DirEntry, error) {
	if r.fail == "directory" {
		return nil, io.ErrClosedPipe
	}
	return stageOSRoot{r.Root}.readDir(name)
}
func TestStageLiteralFailuresAndBoundaries(t *testing.T) {
	paths, f := fixtureGit(t)
	work, err := CreateOperationWorktree(context.Background(), paths, "op", f.Base)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(work.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink("a.md", filepath.Join(work.Path, "link"))
	_ = syscall.Mkfifo(filepath.Join(work.Path, "fifo"), 0600)
	for _, c := range []struct{ Name, Fail string }{{"a.md", "lstat"}, {"a.md", "read"}, {"link", "link"}, {"bin", "directory"}, {"fifo", ""}} {
		if err = stageLiteralPath(context.Background(), store, stageFaultRoot{root, c.Fail}, c.Name, map[string]GitTreeEntry{}); err == nil {
			t.Fatal(c)
		}
	}
	if _, err = (stageOSRoot{root}).readDir("missing"); err == nil {
		t.Fatal("missing directory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = stageLiteralPath(ctx, store, stageFaultRoot{root, ""}, "a.md", map[string]GitTreeEntry{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	_ = os.Mkdir(filepath.Join(work.Path, "bin/.git"), 0700)
	if err = stageLiteralPath(context.Background(), store, stageOSRoot{root}, "bin", map[string]GitTreeEntry{}); err != nil {
		t.Fatal(err)
	}
	_ = syscall.Mkfifo(filepath.Join(work.Path, "bin/fifo"), 0600)
	if err = stageLiteralPath(context.Background(), store, stageOSRoot{root}, "bin", map[string]GitTreeEntry{}); err == nil {
		t.Fatal("nested special")
	}
	broken := &GitStore{storage: &objectFaultStorage{Storer: store.storage, fail: "store"}}
	if err = stageLiteralPath(context.Background(), broken, stageOSRoot{root}, "a.md", map[string]GitTreeEntry{}); err == nil {
		t.Fatal("blob write")
	}
	identity := GitCommitIdentity{"Test", "test@example.invalid", time.Unix(0, 0).UTC()}
	_ = os.WriteFile(filepath.Join(work.Path, "a.md"), []byte("changed"), 0600)
	if _, err = CommitExactPaths(context.Background(), work, []string{"/a.md"}, " \n\n", identity, identity); err == nil {
		t.Fatal("empty message")
	}
	if _, err = CommitExactPaths(context.Background(), work, []string{"/a.md"}, "message", GitCommitIdentity{}, identity); err == nil {
		t.Fatal("bad author")
	}
	// A missing object that remains in the index must prevent a new tree commit.
	admin, _, _ := worktreeAdmin(work.Path)
	idx, err := encodeGitIndex([]GitTreeEntry{{Path: "missing", Mode: filemode.Regular, Hash: plumbing.NewHash(strings.Repeat("1", 40))}})
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(admin, "index"), idx, 0600)
	if _, err = CommitExactPaths(context.Background(), work, []string{"/a.md"}, "message", identity, identity); err == nil {
		t.Fatal("missing indexed blob")
	}
}

func TestWorktreeDoesNotUnlinkNextWritersLocks(t *testing.T) {
	paths, f := fixtureGit(t)
	work, err := CreateOperationWorktree(context.Background(), paths, "op", f.Base)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(work.Path, "a.md"), []byte("changed"), 0600)
	ops := defaultWorktreeIO()
	rename := ops.rename
	ops.rename = func(a, b string) error {
		if err := rename(a, b); err != nil {
			return err
		}
		return os.WriteFile(a, []byte("next writer"), 0600)
	}
	identity := GitCommitIdentity{"Test", "test@example.invalid", time.Unix(0, 0).UTC()}
	if _, err = commitExactPaths(context.Background(), work, []string{"/a.md"}, "message", identity, identity, ops); err != nil {
		t.Fatal(err)
	}
	admin, _, _ := worktreeAdmin(work.Path)
	for _, name := range []string{"HEAD.lock", "index.lock"} {
		data, err := os.ReadFile(filepath.Join(admin, name))
		if err != nil || string(data) != "next writer" {
			t.Fatal("new owner's lock removed", name, err)
		}
	}
}
