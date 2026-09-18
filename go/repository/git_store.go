package repository

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

type GitError struct {
	Message string
	Cause   error
}

func (e *GitError) Error() string { return e.Message }
func (e *GitError) Unwrap() error { return e.Cause }

type GitRepositoryPaths struct{ BareDir, CurrentDir, WorktreesDir string }

// GitStore reads the existing SHA-1 bare repository in pure Go. Open a fresh
// store for operations that must observe newly packed objects/refs. The library
// supplies object decoding only; it is NOT used for main-ref publication.
type GitStore struct {
	Path    string
	storage storage.Storer
}

func OpenGitStore(path string) (*GitStore, error) {
	info, err := os.Stat(filepath.Join(path, "objects"))
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &GitError{Message: "Git object directory is not a directory"}
	}
	return &GitStore{Path: path, storage: filesystem.NewStorage(osfs.New(path), cache.NewObjectLRUDefault())}, nil
}
func gitHash(revision string) (plumbing.Hash, error) {
	if len(revision) != 40 {
		return plumbing.ZeroHash, &GitError{Message: "expected full SHA-1 revision"}
	}
	for _, c := range revision {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return plumbing.ZeroHash, &GitError{Message: "invalid SHA-1 revision"}
		}
	}
	return plumbing.NewHash(revision), nil
}
func (s *GitStore) MainRevision() (string, error) {
	ref, err := s.storage.Reference(plumbing.NewBranchReferenceName("main"))
	if err != nil {
		return "", err
	}
	if ref.Type() != plumbing.HashReference {
		return "", &GitError{Message: "symbolic main reference is not supported"}
	}
	return ref.Hash().String(), nil
}
func GetMainRevision(paths GitRepositoryPaths) (string, error) {
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		return "", err
	}
	return store.MainRevision()
}
func (s *GitStore) commit(revision string) (*object.Commit, error) {
	hash, err := gitHash(revision)
	if err != nil {
		return nil, err
	}
	return object.GetCommit(s.storage, hash)
}

// GitTreeEntry includes links/executable modes; paths are relative raw tree
// names, not Git's quoted ls-tree presentation. Service paths must validate
// separately. Gitlinks are included, but this port does not fetch submodules.
type GitTreeEntry struct {
	Path string
	Mode filemode.FileMode
	Hash plumbing.Hash
}

func (s *GitStore) TreeEntries(ctx context.Context, revision string) ([]GitTreeEntry, error) {
	commit, err := s.commit(revision)
	if err != nil {
		return nil, err
	}
	out := []GitTreeEntry{}
	var walk func(plumbing.Hash, string, int) error
	walk = func(hash plumbing.Hash, prefix string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > 1024 {
			return &GitError{Message: "Git tree depth limit"}
		}
		tree, err := object.GetTree(s.storage, hash)
		if err != nil {
			return err
		}
		for _, entry := range tree.Entries {
			// Tree objects are not trusted filesystem paths, even inside our store.
			if entry.Name == "" || entry.Name == "." || entry.Name == ".." || strings.ContainsAny(entry.Name, "/\x00") {
				return &GitError{Message: "invalid Git tree entry name"}
			}
			path := prefix + entry.Name
			if entry.Mode == filemode.Dir {
				if err = walk(entry.Hash, path+"/", depth+1); err != nil {
					return err
				}
			} else {
				out = append(out, GitTreeEntry{Path: path, Mode: entry.Mode, Hash: entry.Hash})
			}
		}
		return nil
	}
	if err = walk(commit.TreeHash, "", 0); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
func (s *GitStore) ReadBlob(revision, bundlePath string) ([]byte, error) {
	if err := ValidateBundlePath(bundlePath); err != nil {
		return nil, err
	}
	if bundlePath == "/" {
		return nil, &GitError{Message: "repository path must name a blob"}
	}
	commit, err := s.commit(revision)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	entry, err := tree.FindEntry(bundlePath[1:])
	if err != nil {
		return nil, err
	}
	if entry.Mode != filemode.Regular && entry.Mode != filemode.Executable && entry.Mode != filemode.Symlink {
		return nil, &GitError{Message: "repository path is not a blob"}
	}
	blob, err := object.GetBlob(s.storage, entry.Hash)
	if err != nil {
		return nil, err
	}
	reader, err := blob.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
func DiffMainPaths(ctx context.Context, paths GitRepositoryPaths, baseRevision, endRevision string) ([]string, error) {
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		return nil, err
	}
	return store.DiffPaths(ctx, baseRevision, endRevision)
}
func (s *GitStore) DiffPaths(ctx context.Context, baseRevision, endRevision string) ([]string, error) {
	before, err := s.TreeEntries(ctx, baseRevision)
	if err != nil {
		return nil, err
	}
	after, err := s.TreeEntries(ctx, endRevision)
	if err != nil {
		return nil, err
	}
	old := map[string]GitTreeEntry{}
	for _, entry := range before {
		old[entry.Path] = entry
	}
	changed := []string{}
	for _, entry := range after {
		previous, exists := old[entry.Path]
		if !exists || previous.Hash != entry.Hash || previous.Mode != entry.Mode {
			changed = append(changed, "/"+entry.Path)
		}
		delete(old, entry.Path)
	}
	for path := range old {
		changed = append(changed, "/"+path)
	}
	sort.Strings(changed)
	return changed, nil
}

// PublishOutcomeError is returned only after the ref rename has succeeded but
// directory durability could not be confirmed. The caller MUST reconcile the
// original operation; blindly retrying a mutation would risk duplicate writes.
type PublishOutcomeError struct {
	Revision string
	Cause    error
}

func (e *PublishOutcomeError) Error() string {
	return "main reference published but durability is indeterminate: " + e.Revision
}
func (e *PublishOutcomeError) Unwrap() error { return e.Cause }

type gitLockFile interface {
	io.Writer
	Sync() error
	Close() error
}
type gitPublishIO struct {
	openLock func(string) (gitLockFile, error)
	rename   func(string, string) error
	remove   func(string) error
	syncDir  func(string) error
}

func defaultGitPublishIO() gitPublishIO {
	return gitPublishIO{
		openLock: func(path string) (gitLockFile, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
		},
		rename: os.Rename, remove: os.Remove, syncDir: syncDirectory,
	}
}
func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// PublishMainCompareAndSwap uses Git's refs/heads/main.lock protocol, rechecks
// loose OR packed main while holding the lock, and atomically renames the new
// loose ref. It never calls go-git's advisory-file-lock CAS. Existing reflogs
// and symbolic main refs fail explicitly until their parity is implemented.
func PublishMainCompareAndSwap(paths GitRepositoryPaths, baseRevision, newRevision string) (bool, error) {
	return publishMain(paths, baseRevision, newRevision, defaultGitPublishIO())
}
func publishMain(paths GitRepositoryPaths, baseRevision, newRevision string, ops gitPublishIO) (bool, error) {
	base, err := gitHash(baseRevision)
	if err != nil {
		return false, err
	}
	next, err := gitHash(newRevision)
	if err != nil {
		return false, err
	}
	if next == plumbing.ZeroHash {
		return false, &GitError{Message: "main deletion is not supported"}
	}
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		return false, err
	}
	if _, err = store.commit(next.String()); err != nil {
		return false, err
	}
	config, err := store.storage.Config()
	if err != nil {
		return false, err
	}
	// A bare default repo has no reflog. Do not silently ignore a request
	// to create one, including the explicit always setting.
	logUpdates := strings.ToLower(config.Raw.Section("core").Option("logallrefupdates"))
	if logUpdates != "" && logUpdates != "false" && logUpdates != "no" && logUpdates != "off" && logUpdates != "0" {
		return false, &GitError{Message: "configured reflog updates are not implemented"}
	}
	if format := config.Raw.Section("extensions").Option("refstorage"); format != "" && format != "files" {
		return false, &GitError{Message: "Git reference storage format is not supported"}
	}
	if _, err = os.Stat(filepath.Join(paths.BareDir, "logs", "refs", "heads", "main")); err == nil {
		return false, &GitError{Message: "main reflog updates are not implemented"}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	refPath := filepath.Join(paths.BareDir, "refs", "heads", "main")
	if err = os.MkdirAll(filepath.Dir(refPath), 0777); err != nil {
		return false, err
	}
	lockPath := refPath + ".lock"
	file, err := ops.openLock(lockPath)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	closed := false
	published := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		if !published {
			_ = ops.remove(lockPath)
		}
	}()
	// Reference reads consult loose/packed refs on disk under our lock.
	if info, err := os.Lstat(refPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return false, &GitError{Message: "symlink main reference is not supported"}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	current, err := store.MainRevision()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		current = plumbing.ZeroHash.String()
	} else if err != nil {
		return false, err
	}
	if current != base.String() {
		return false, nil
	}
	data := []byte(next.String() + "\n")
	n, err := file.Write(data)
	if err != nil {
		return false, err
	}
	if n != len(data) {
		return false, io.ErrShortWrite
	}
	if err = file.Sync(); err != nil {
		return false, err
	}
	err = file.Close()
	closed = true
	if err != nil {
		return false, err
	}
	if err = ops.rename(lockPath, refPath); err != nil {
		return false, err
	}
	published = true
	if err = ops.syncDir(filepath.Dir(refPath)); err != nil {
		return true, &PublishOutcomeError{Revision: next.String(), Cause: err}
	}
	return true, nil
}
