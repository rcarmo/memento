package repository

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

type BootstrapResult struct {
	Revision     string
	ChangedPaths []string
}

// BootstrapRepository creates an initial bare main commit and a plain current
// checkout without executing Git. An existing main is never overwritten. The
// service must hold its writer lease before bootstrapping. Empty seeds produce
// the canonical empty tree and an allow-empty root commit, as in Python.
func BootstrapRepository(ctx context.Context, paths GitRepositoryPaths, seedDir string) (BootstrapResult, error) {
	return bootstrapRepository(ctx, paths, seedDir, time.Now(), defaultBootstrapIO())
}

type bootstrapIO struct {
	abs         func(string) (string, error)
	lstat       func(string) (fs.FileInfo, error)
	openStore   func(string) (*GitStore, error)
	openRoot    func(string) (*os.Root, error)
	mkdirAll    func(string, fs.FileMode) error
	mkdir       func(string, fs.FileMode) error
	writeFile   func(string, []byte, fs.FileMode) error
	copySeed    func(context.Context, string, string) error
	publish     func(GitRepositoryPaths, string, string) (bool, error)
	materialize func(context.Context, GitRepositoryPaths, string) (MaterializedCheckout, error)
}

func defaultBootstrapIO() bootstrapIO {
	return bootstrapIO{filepath.Abs, os.Lstat, OpenGitStore, os.OpenRoot, os.MkdirAll, os.Mkdir, os.WriteFile, copyBootstrapSeed, PublishMainCompareAndSwap, MaterializeCurrentCheckout}
}
func bootstrapRepository(ctx context.Context, paths GitRepositoryPaths, seedDir string, when time.Time, ops bootstrapIO) (BootstrapResult, error) {
	if err := ctx.Err(); err != nil {
		return BootstrapResult{}, err
	}
	for _, value := range []string{paths.BareDir, paths.WorktreesDir, paths.CurrentDir} {
		if value == "" {
			return BootstrapResult{}, &GitError{Message: "repository bootstrap paths are required"}
		}
	}
	bare, err := ops.abs(paths.BareDir)
	if err != nil {
		return BootstrapResult{}, err
	}
	worktrees, err := ops.abs(paths.WorktreesDir)
	if err != nil {
		return BootstrapResult{}, err
	}
	if pathsOverlap(bare, worktrees) {
		return BootstrapResult{}, &GitError{Message: "bootstrap worktree overlaps bare repository"}
	}
	if err = rejectSymlinkAncestors(bare, ops.lstat); err != nil {
		return BootstrapResult{}, err
	}
	if err = rejectSymlinkAncestors(worktrees, ops.lstat); err != nil {
		return BootstrapResult{}, err
	}
	// Existing repositories must go through ordinary recovery, not a new root.
	if store, err := ops.openStore(bare); err == nil {
		if _, err = store.MainRevision(); !errors.Is(err, plumbing.ErrReferenceNotFound) {
			if err != nil {
				return BootstrapResult{}, err
			}
			return BootstrapResult{}, &GitError{Message: "repository main already exists"}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return BootstrapResult{}, err
	}
	for _, name := range []string{bare, filepath.Join(bare, "objects"), filepath.Join(bare, "objects", "pack"), filepath.Join(bare, "refs", "heads"), worktrees} {
		if err = ops.mkdirAll(name, 0777); err != nil {
			return BootstrapResult{}, err
		}
	}
	for name, content := range map[string]string{"HEAD": "ref: refs/heads/main\n", "config": "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = true\n"} {
		target := filepath.Join(bare, name)
		if _, err = ops.lstat(target); errors.Is(err, fs.ErrNotExist) {
			if err = ops.writeFile(target, []byte(content), 0666); err != nil {
				return BootstrapResult{}, err
			}
		} else if err != nil {
			return BootstrapResult{}, err
		}
	}
	temporary := filepath.Join(worktrees, ".bootstrap")
	if err = ops.mkdir(temporary, 0777); err != nil {
		return BootstrapResult{}, err
	}
	defer os.RemoveAll(temporary)
	if seedDir != "" {
		if err = ops.copySeed(ctx, seedDir, temporary); err != nil {
			return BootstrapResult{}, err
		}
	}
	store, err := ops.openStore(bare)
	if err != nil {
		return BootstrapResult{}, err
	}
	root, err := ops.openRoot(temporary)
	if err != nil {
		return BootstrapResult{}, err
	}
	defer root.Close()
	snapshot := map[string]GitTreeEntry{}
	if err = stageLiteralPath(ctx, store, stageOSRoot{root}, ".", snapshot); err != nil {
		return BootstrapResult{}, err
	}
	entries := []GitTreeEntry{}
	changed := []string{}
	for _, name := range sortedKeys(snapshot) {
		entries = append(entries, snapshot[name])
		changed = append(changed, "/"+name)
	}
	tree, err := store.WriteTree(ctx, entries)
	if err != nil {
		return BootstrapResult{}, err
	}
	identity := GitCommitIdentity{Name: "Memento", Email: "memento@example.invalid", When: when}
	revision, err := store.WriteCommit(tree, nil, "memento: bootstrap main\n", identity, identity)
	if err != nil {
		return BootstrapResult{}, err
	}
	published, err := ops.publish(paths, plumbing.ZeroHash.String(), revision)
	if err != nil {
		return BootstrapResult{}, err
	}
	if !published {
		return BootstrapResult{}, &GitError{Message: "repository main moved during bootstrap"}
	}
	checkout, err := ops.materialize(ctx, paths, revision)
	if err != nil {
		return BootstrapResult{}, err
	}
	return BootstrapResult{Revision: checkout.Revision, ChangedPaths: changed}, nil
}

// The Python seed copy follows file symlinks, creates but does not descend
// symlink directories, and copies file modes/mtime. Rooted reads fail closed on
// links escaping the seed root; external seed symlinks are an explicit gap.
func copyBootstrapSeed(ctx context.Context, source, target string) error {
	input, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenRoot(target)
	if err != nil {
		return err
	}
	defer output.Close()
	return copySeedTree(ctx, seedOSRoot{input}, seedOSRoot{output}, ".")
}

type seedFile interface {
	io.ReadWriteCloser
	ReadDir(int) ([]os.DirEntry, error)
}
type seedFS interface {
	open(string) (seedFile, error)
	create(string, fs.FileMode) (seedFile, error)
	Stat(string) (fs.FileInfo, error)
	MkdirAll(string, fs.FileMode) error
	Chmod(string, fs.FileMode) error
	Chtimes(string, time.Time, time.Time) error
}
type seedOSRoot struct{ *os.Root }

func (r seedOSRoot) open(name string) (seedFile, error) { return r.Open(name) }
func (r seedOSRoot) create(name string, mode fs.FileMode) (seedFile, error) {
	return r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
}
func copySeedTree(ctx context.Context, input, output seedFS, directory string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := input.open(directory)
	if err != nil {
		return err
	}
	items, readErr := file.ReadDir(-1)
	closeErr := file.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, item := range items {
		if strings.EqualFold(item.Name(), ".git") {
			return &GitError{Message: "seed Git metadata is not supported"}
		}
		name := filepath.Join(directory, item.Name())
		info, err := input.Stat(name)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err = output.MkdirAll(name, 0777); err != nil {
				return err
			}
			if item.Type()&os.ModeSymlink == 0 {
				if err = copySeedTree(ctx, input, output, name); err != nil {
					return err
				}
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return &GitError{Message: "seed contains a special file"}
		}
		if err = copySeedFile(input, output, name, info); err != nil {
			return err
		}
	}
	return nil
}
func copySeedFile(input, output seedFS, name string, info fs.FileInfo) error {
	source, err := input.open(name)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := output.create(name, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err = output.Chmod(name, info.Mode().Perm()); err != nil {
		return err
	}
	return output.Chtimes(name, info.ModTime(), info.ModTime())
}
