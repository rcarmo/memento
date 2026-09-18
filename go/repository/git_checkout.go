package repository

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type MaterializedCheckout struct{ Revision, Path string }

// MaterializeCurrentCheckout replaces the derived plain checkout, never main.
// Only objects reachable from the requested commit are written; hooks, filters,
// submodule fetches and Git subprocesses are not executed. The directory is a
// rebuildable cache. A failed write can leave it partial; callers must not use
// it until materialisation succeeds and must reconcile from main on restart.
func MaterializeCurrentCheckout(ctx context.Context, paths GitRepositoryPaths, revision string) (MaterializedCheckout, error) {
	return materializeCheckout(ctx, paths, revision, defaultCheckoutIO())
}

type checkoutIO struct {
	abs       func(string) (string, error)
	lstat     func(string) (fs.FileInfo, error)
	removeAll func(string) error
	mkdirAll  func(string, fs.FileMode) error
	openRoot  func(string) (*os.Root, error)
}

func defaultCheckoutIO() checkoutIO {
	return checkoutIO{filepath.Abs, os.Lstat, os.RemoveAll, os.MkdirAll, os.OpenRoot}
}
func materializeCheckout(ctx context.Context, paths GitRepositoryPaths, revision string, ops checkoutIO) (MaterializedCheckout, error) {
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		return MaterializedCheckout{}, err
	}
	if revision == "" {
		revision, err = store.MainRevision()
		if err != nil {
			return MaterializedCheckout{}, err
		}
	}
	entries, err := store.TreeEntries(ctx, revision)
	if err != nil {
		return MaterializedCheckout{}, err
	}
	if err = validateCheckoutEntries(entries); err != nil {
		return MaterializedCheckout{}, err
	}
	// Fail before removing anything if configuration points a cache at the
	// canonical object store (or its parent). These are host paths, not MCP args.
	target, err := ops.abs(paths.CurrentDir)
	if err != nil {
		return MaterializedCheckout{}, err
	}
	bare, err := ops.abs(paths.BareDir)
	if err != nil {
		return MaterializedCheckout{}, err
	}
	if target == string(filepath.Separator) || paths.CurrentDir == "" || pathsOverlap(target, bare) {
		return MaterializedCheckout{}, &GitError{Message: "unsafe checkout destination"}
	}
	if err = rejectSymlinkAncestors(target, ops.lstat); err != nil {
		return MaterializedCheckout{}, err
	}
	if err = rejectSymlinkAncestors(bare, ops.lstat); err != nil {
		return MaterializedCheckout{}, err
	}
	if info, err := ops.lstat(target); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return MaterializedCheckout{}, &GitError{Message: "checkout destination must be a real directory"}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return MaterializedCheckout{}, err
	}
	if err = ops.removeAll(target); err != nil {
		return MaterializedCheckout{}, err
	}
	if err = ops.mkdirAll(target, 0777); err != nil {
		return MaterializedCheckout{}, err
	}
	root, err := ops.openRoot(target)
	if err != nil {
		return MaterializedCheckout{}, err
	}
	defer root.Close()
	destination := checkoutRoot{root}
	if err = materializeEntries(ctx, destination, entries, func(hash plumbing.Hash) (io.ReadCloser, error) {
		blob, err := object.GetBlob(store.storage, hash)
		if err != nil {
			return nil, err
		}
		return blob.Reader()
	}); err != nil {
		return MaterializedCheckout{}, err
	}
	return MaterializedCheckout{Revision: revision, Path: paths.CurrentDir}, nil
}

// Host-owned repository paths must not alias through symlink ancestors. This
// preflight is not protection against a malicious host replacing directories;
// deployment must keep their parents writable only by the service owner.
func rejectSymlinkAncestors(target string, lstat func(string) (fs.FileInfo, error)) error {
	for parent := filepath.Dir(target); parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
		info, err := lstat(parent)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return &GitError{Message: "symlink checkout ancestor is not supported"}
		}
	}
	return nil
}
func pathsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator)) || strings.HasPrefix(b, a+string(filepath.Separator))
}
func validateCheckoutEntries(entries []GitTreeEntry) error {
	seen := map[string]bool{}
	for _, entry := range entries {
		if err := ValidateBundlePath("/" + entry.Path); err != nil {
			return err
		}
		if entry.Path == "" {
			return &GitError{Message: "empty checkout path"}
		}
		for _, part := range strings.Split(entry.Path, "/") {
			if strings.EqualFold(part, ".git") {
				return &GitError{Message: "Git metadata path is not a checkout file"}
			}
		}
		if seen[entry.Path] {
			return &GitError{Message: "duplicate Git checkout path"}
		}
		seen[entry.Path] = true
		if entry.Mode != filemode.Regular && entry.Mode != filemode.Deprecated && entry.Mode != filemode.Executable && entry.Mode != filemode.Symlink && entry.Mode != filemode.Submodule {
			return &GitError{Message: "unsupported checkout file mode"}
		}
	}
	for _, entry := range entries {
		for parent := path.Dir(entry.Path); parent != "."; parent = path.Dir(parent) {
			if seen[parent] {
				return &GitError{Message: "Git checkout file/directory collision"}
			}
		}
	}
	return nil
}

type checkoutFile interface {
	io.Writer
	Close() error
}
type checkoutFS interface {
	MkdirAll(string, fs.FileMode) error
	Symlink(string, string) error
	create(string, fs.FileMode) (checkoutFile, error)
}
type checkoutRoot struct{ *os.Root }

func (r checkoutRoot) create(name string, mode fs.FileMode) (checkoutFile, error) {
	return r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
}
func materializeEntries(ctx context.Context, destination checkoutFS, entries []GitTreeEntry, blob func(plumbing.Hash) (io.ReadCloser, error)) error {
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Mode == filemode.Submodule {
			if err := destination.MkdirAll(entry.Path, 0777); err != nil {
				return err
			}
			continue
		}
		if err := destination.MkdirAll(path.Dir(entry.Path), 0777); err != nil {
			return err
		}
		reader, err := blob(entry.Hash)
		if err != nil {
			return err
		}
		if entry.Mode == filemode.Symlink {
			target, err := io.ReadAll(reader)
			closeErr := reader.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			if err = destination.Symlink(string(target), entry.Path); err != nil {
				return err
			}
			continue
		}
		mode := fs.FileMode(0666)
		if entry.Mode == filemode.Executable {
			mode = 0777
		}
		file, err := destination.create(entry.Path, mode)
		if err != nil {
			_ = reader.Close()
			return err
		}
		_, copyErr := io.Copy(file, reader)
		readCloseErr := reader.Close()
		writeCloseErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if readCloseErr != nil {
			return readCloseErr
		}
		if writeCloseErr != nil {
			return writeCloseErr
		}
	}
	return nil
}
