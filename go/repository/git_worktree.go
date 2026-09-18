package repository

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
)

type worktreeIO struct {
	abs         func(string) (string, error)
	lstat       func(string) (fs.FileInfo, error)
	mkdirAll    func(string, fs.FileMode) error
	mkdir       func(string, fs.FileMode) error
	writeFile   func(string, []byte, fs.FileMode) error
	removeAll   func(string) error
	lock        func(string) (gitLockFile, error)
	rename      func(string, string) error
	syncDir     func(string) error
	openRoot    func(string) (*os.Root, error)
	encodeIndex func([]GitTreeEntry) ([]byte, error)
}

func defaultWorktreeIO() worktreeIO {
	return worktreeIO{filepath.Abs, os.Lstat, os.MkdirAll, os.Mkdir, os.WriteFile, os.RemoveAll, defaultGitPublishIO().openLock, os.Rename, syncDirectory, os.OpenRoot, encodeGitIndex}
}

type Worktree struct{ OpID, Path, BaseRevision string }
type StagedCommit struct {
	Revision     string
	ChangedPaths []string
}

func validOperationID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\\x00\r\n")
}

// CreateOperationWorktree writes the native linked-worktree layout: shared
// objects, detached HEAD, per-worktree index, commondir and both gitdir links.
// It refuses collisions rather than deleting unknown prior operation state.
// Service recovery must reconcile/remove its existing worktree before retrying.
func CreateOperationWorktree(ctx context.Context, paths GitRepositoryPaths, opID, baseRevision string) (Worktree, error) {
	return createOperationWorktree(ctx, paths, opID, baseRevision, defaultWorktreeIO())
}
func createOperationWorktree(ctx context.Context, paths GitRepositoryPaths, opID, baseRevision string, ops worktreeIO) (Worktree, error) {
	if !validOperationID(opID) {
		return Worktree{}, &GitError{Message: "invalid operation worktree id"}
	}
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		return Worktree{}, err
	}
	entries, err := store.TreeEntries(ctx, baseRevision)
	if err != nil {
		return Worktree{}, err
	}
	base, err := ops.abs(paths.WorktreesDir)
	if err != nil {
		return Worktree{}, err
	}
	bare, err := ops.abs(paths.BareDir)
	if err != nil {
		return Worktree{}, err
	}
	workPath := filepath.Join(base, opID)
	admin := filepath.Join(bare, "worktrees", opID)
	for _, target := range []string{workPath, admin} {
		if _, err := ops.lstat(target); err == nil {
			return Worktree{}, &GitError{Message: "operation worktree already exists"}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return Worktree{}, err
		}
	}
	// Bare stores can have only packed refs; retain Git's required refs
	// directory when reconstructing them from an exported object fixture.
	if err := ops.mkdirAll(filepath.Join(bare, "refs", "heads"), 0777); err != nil {
		return Worktree{}, err
	}
	if err := ops.mkdirAll(filepath.Dir(admin), 0777); err != nil {
		return Worktree{}, err
	}
	if err := ops.mkdir(admin, 0777); err != nil {
		return Worktree{}, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = ops.removeAll(admin)
			_ = ops.removeAll(workPath)
		}
	}()
	checkoutPaths := paths
	checkoutPaths.CurrentDir = workPath
	if _, err = MaterializeCurrentCheckout(ctx, checkoutPaths, baseRevision); err != nil {
		return Worktree{}, err
	}
	content := map[string][]byte{
		filepath.Join(admin, "HEAD"):      []byte(baseRevision + "\n"),
		filepath.Join(admin, "commondir"): []byte("../..\n"),
		filepath.Join(admin, "gitdir"):    []byte(workPath + "/.git\n"),
		filepath.Join(workPath, ".git"):   []byte("gitdir: " + admin + "\n"),
	}
	idx, err := ops.encodeIndex(entries)
	if err != nil {
		return Worktree{}, err
	}
	content[filepath.Join(admin, "index")] = idx
	for _, name := range sortedKeys(content) {
		if err = ops.writeFile(name, content[name], 0666); err != nil {
			return Worktree{}, err
		}
	}
	complete = true
	return Worktree{OpID: opID, Path: workPath, BaseRevision: baseRevision}, nil
}
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func encodeGitIndex(entries []GitTreeEntry) ([]byte, error) {
	idx := &index.Index{Version: 2}
	for _, entry := range entries {
		idx.Entries = append(idx.Entries, &index.Entry{Name: entry.Path, Hash: entry.Hash, Mode: entry.Mode})
	}
	var raw bytes.Buffer
	err := index.NewEncoder(&raw).Encode(idx)
	return raw.Bytes(), err
}
func readGitIndex(admin string) ([]GitTreeEntry, error) {
	file, err := os.Open(filepath.Join(admin, "index"))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	idx := &index.Index{}
	if err := index.NewDecoder(file).Decode(idx); err != nil {
		return nil, err
	}
	entries := []GitTreeEntry{}
	for _, entry := range idx.Entries {
		if entry.Stage != 0 {
			return nil, &GitError{Message: "unmerged Git index"}
		}
		entries = append(entries, GitTreeEntry{Path: entry.Name, Hash: entry.Hash, Mode: entry.Mode})
	}
	return entries, nil
}

func worktreeAdmin(worktreePath string) (string, string, error) {
	raw, err := os.ReadFile(filepath.Join(worktreePath, ".git"))
	if err != nil {
		return "", "", err
	}
	value := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(value, "gitdir: ") {
		return "", "", &GitError{Message: "not a linked operation worktree"}
	}
	admin := strings.TrimPrefix(value, "gitdir: ")
	if !filepath.IsAbs(admin) {
		admin = filepath.Join(worktreePath, admin)
	}
	common, err := os.ReadFile(filepath.Join(admin, "commondir"))
	if err != nil {
		return "", "", err
	}
	root := strings.TrimSpace(string(common))
	if !filepath.IsAbs(root) {
		root = filepath.Join(admin, root)
	}
	return admin, root, nil
}
func ResolveWorktreeRevision(worktreePath string) (string, error) {
	if _, err := os.Stat(worktreePath); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	admin, _, err := worktreeAdmin(worktreePath)
	if err != nil {
		return "", err
	}
	return detachedRevision(admin)
}
func detachedRevision(admin string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(admin, "HEAD"))
	if err != nil {
		return "", err
	}
	hash, err := gitHash(strings.TrimSpace(string(raw)))
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}
func RemoveOperationWorktree(paths GitRepositoryPaths, opID string) error {
	return removeOperationWorktree(paths, opID, defaultWorktreeIO())
}
func removeOperationWorktree(paths GitRepositoryPaths, opID string, ops worktreeIO) error {
	if !validOperationID(opID) {
		return &GitError{Message: "invalid operation worktree id"}
	}
	workPath := filepath.Join(paths.WorktreesDir, opID)
	if _, err := ops.lstat(workPath); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	admin, common, err := worktreeAdmin(workPath)
	if err != nil {
		return err
	}
	expected, err := ops.abs(filepath.Join(paths.BareDir, "worktrees", opID))
	if err != nil {
		return err
	}
	actual, err := ops.abs(admin)
	if err != nil {
		return err
	}
	bare, err := ops.abs(paths.BareDir)
	if err != nil {
		return err
	}
	actualCommon, err := ops.abs(common)
	if err != nil {
		return err
	}
	if actual != expected || bare != actualCommon {
		return &GitError{Message: "worktree metadata points outside repository"}
	}
	if err = ops.removeAll(workPath); err != nil {
		return err
	}
	return ops.removeAll(admin)
}
func ExactStagedPaths(worktreePath string) ([]string, error) {
	admin, common, err := worktreeAdmin(worktreePath)
	if err != nil {
		return nil, err
	}
	revision, err := detachedRevision(admin)
	if err != nil {
		return nil, err
	}
	store, err := OpenGitStore(common)
	if err != nil {
		return nil, err
	}
	before, err := store.TreeEntries(context.Background(), revision)
	if err != nil {
		return nil, err
	}
	after, err := readGitIndex(admin)
	if err != nil {
		return nil, err
	}
	return changedTreePaths(before, after), nil
}
func changedTreePaths(before, after []GitTreeEntry) []string {
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
	return changed
}

// CommitExactPaths stages only the named literal files/subtrees atop the current
// index and stores a detached commit. main is not changed. Git pathspec magic,
// ignore/filter/attributes and symlink/directory races need further parity; use
// only validated service-owned operation trees until those gates pass.
func CommitExactPaths(ctx context.Context, worktree Worktree, changedPaths []string, message string, author, committer GitCommitIdentity) (StagedCommit, error) {
	return commitExactPaths(ctx, worktree, changedPaths, message, author, committer, defaultWorktreeIO())
}
func commitExactPaths(ctx context.Context, worktree Worktree, changedPaths []string, message string, author, committer GitCommitIdentity, ops worktreeIO) (StagedCommit, error) {
	if len(changedPaths) == 0 {
		return StagedCommit{}, &GitError{Message: "changed_paths must not be empty"}
	}
	admin, common, err := worktreeAdmin(worktree.Path)
	if err != nil {
		return StagedCommit{}, err
	}
	store, err := OpenGitStore(common)
	if err != nil {
		return StagedCommit{}, err
	}
	// Git locks index before staging and detached HEAD before committing. Take
	// both up front; never truncate either public file before object creation.
	indexLock, err := ops.lock(filepath.Join(admin, "index.lock"))
	if err != nil {
		return StagedCommit{}, err
	}
	indexPublished := false
	defer func() {
		_ = indexLock.Close()
		if !indexPublished {
			_ = os.Remove(filepath.Join(admin, "index.lock"))
		}
	}()
	headLock, err := ops.lock(filepath.Join(admin, "HEAD.lock"))
	if err != nil {
		return StagedCommit{}, err
	}
	headPublished := false
	defer func() {
		_ = headLock.Close()
		if !headPublished {
			_ = os.Remove(filepath.Join(admin, "HEAD.lock"))
		}
	}()
	head, err := detachedRevision(admin)
	if err != nil {
		return StagedCommit{}, err
	}
	entries, err := readGitIndex(admin)
	if err != nil {
		return StagedCommit{}, err
	}
	snapshot := map[string]GitTreeEntry{}
	for _, entry := range entries {
		snapshot[entry.Path] = entry
	}
	root, err := ops.openRoot(worktree.Path)
	if err != nil {
		return StagedCommit{}, err
	}
	defer root.Close()
	for _, bundlePath := range changedPaths {
		if err = ValidateBundlePath(bundlePath); err != nil {
			return StagedCommit{}, err
		}
		relative := strings.TrimPrefix(bundlePath, "/")
		if err = validateCheckoutEntries([]GitTreeEntry{{Path: relative, Mode: filemode.Regular}}); err != nil {
			return StagedCommit{}, err
		}
		if err = stageLiteralPath(ctx, store, stageOSRoot{root}, relative, snapshot); err != nil {
			return StagedCommit{}, err
		}
	}
	entries = []GitTreeEntry{}
	for _, name := range sortedKeys(snapshot) {
		entries = append(entries, snapshot[name])
	}
	tree, err := store.WriteTree(ctx, entries)
	if err != nil {
		return StagedCommit{}, err
	}
	parent, err := store.commit(head)
	if err != nil {
		return StagedCommit{}, err
	}
	if tree == parent.TreeHash {
		return StagedCommit{}, &GitError{Message: "nothing to commit"}
	}
	// git commit -m normalises outer blank lines/trailing spaces by default.
	message = cleanCommitMessage(message)
	if message == "\n" {
		return StagedCommit{}, &GitError{Message: "empty commit message"}
	}
	revision, err := store.WriteCommit(tree, []string{head}, message, author, committer)
	if err != nil {
		return StagedCommit{}, err
	}
	raw, err := ops.encodeIndex(entries)
	if err != nil {
		return StagedCommit{}, err
	}
	if err = writeAndSync(indexLock, raw); err != nil {
		return StagedCommit{}, err
	}
	if err = writeAndSync(headLock, []byte(revision+"\n")); err != nil {
		return StagedCommit{}, err
	}
	if err = indexLock.Close(); err != nil {
		return StagedCommit{}, err
	}
	if err = headLock.Close(); err != nil {
		return StagedCommit{}, err
	}
	// Index publication precedes HEAD; failure leaves recoverable staged changes.
	if err = ops.rename(filepath.Join(admin, "index.lock"), filepath.Join(admin, "index")); err != nil {
		return StagedCommit{}, err
	}
	indexPublished = true
	if err = ops.rename(filepath.Join(admin, "HEAD.lock"), filepath.Join(admin, "HEAD")); err != nil {
		return StagedCommit{}, err
	}
	headPublished = true
	if err = ops.syncDir(admin); err != nil {
		return StagedCommit{}, &PublishOutcomeError{Revision: revision, Cause: err}
	}
	changed := append([]string{}, changedPaths...)
	sort.Strings(changed)
	return StagedCommit{Revision: revision, ChangedPaths: changed}, nil
}
func writeAndSync(file interface {
	io.Writer
	Sync() error
}, data []byte) error {
	n, err := file.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return file.Sync()
}
func cleanCommitMessage(message string) string {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	out := []string{}
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if line == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, line)
	}
	return strings.Trim(strings.Join(out, "\n"), "\n") + "\n"
}

type stageRoot interface {
	Lstat(string) (fs.FileInfo, error)
	ReadFile(string) ([]byte, error)
	Readlink(string) (string, error)
	readDir(string) ([]os.DirEntry, error)
}
type stageOSRoot struct{ *os.Root }

func (r stageOSRoot) readDir(name string) ([]os.DirEntry, error) {
	file, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.ReadDir(-1)
}
func stageLiteralPath(ctx context.Context, store *GitStore, root stageRoot, name string, snapshot map[string]GitTreeEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		matched := false
		for existing := range snapshot {
			if existing == name || strings.HasPrefix(existing, name+"/") {
				delete(snapshot, existing)
				matched = true
			}
		}
		if !matched {
			return &GitError{Message: "pathspec did not match any files: " + name}
		}
		return nil
	}
	if err != nil {
		return err
	}
	// Clear tracked subtree before collecting current contents (directory
	// deletions and file/directory replacements must be staged too).
	for existing := range snapshot {
		if existing == name || strings.HasPrefix(existing, name+"/") {
			delete(snapshot, existing)
		}
	}
	if info.IsDir() {
		children, err := root.readDir(name)
		if err != nil {
			return err
		}
		for _, child := range children {
			if strings.EqualFold(child.Name(), ".git") {
				continue
			}
			if err = stageLiteralPath(ctx, store, root, path.Join(name, child.Name()), snapshot); err != nil {
				return err
			}
		}
		return nil
	}
	var data []byte
	mode := filemode.Regular
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := root.Readlink(name)
		if err != nil {
			return err
		}
		data = []byte(target)
		mode = filemode.Symlink
	} else {
		if !info.Mode().IsRegular() {
			return &GitError{Message: "cannot stage special file"}
		}
		data, err = root.ReadFile(name)
		if err != nil {
			return err
		}
		if info.Mode()&0111 != 0 {
			mode = filemode.Executable
		}
	}
	hash, err := store.WriteBlob(data)
	if err != nil {
		return err
	}
	snapshot[name] = GitTreeEntry{Path: name, Mode: mode, Hash: hash}
	return nil
}
