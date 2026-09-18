package service

import (
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

// WorktreeMutator writes only a disposable transaction worktree. It does not
// publish Git or authorise the originating request. The caller must normalise
// changes and check their read/write policy before invoking these operations.
// Trash and rename also check permissions on affected reference/destination paths.
type WorktreeMutator struct {
	MaxConceptBytes int
	Proposals       control.Proposals
	Now             func() time.Time
	Random          io.Reader
}
type mutationIO struct {
	read   func(string, string) (repository.BundleEntry, error)
	list   func(string) ([]string, error)
	stat   func(string) (os.FileInfo, error)
	mkdir  func(string, string) error
	write  func(string, string, string, bool) error
	remove func(string, string) error
	rename func(string, string, string) error
}

func defaultMutationIO() mutationIO {
	return mutationIO{
		read: repository.ReadBundleEntry,
		list: func(root string) ([]string, error) {
			return repository.ListBundlePaths(root, repository.BundleFilter{})
		},
		stat: os.Stat,
		mkdir: func(root, path string) error {
			r, err := os.OpenRoot(root)
			if err != nil {
				return err
			}
			defer r.Close()
			return r.MkdirAll(strings.TrimPrefix(path, "/"), 0755)
		},
		write: writeMutationText,
		remove: func(root, path string) error {
			r, err := os.OpenRoot(root)
			if err != nil {
				return err
			}
			defer r.Close()
			return r.Remove(strings.TrimPrefix(path, "/"))
		},
		rename: func(root, source, target string) error {
			r, err := os.OpenRoot(root)
			if err != nil {
				return err
			}
			defer r.Close()
			return r.Rename(strings.TrimPrefix(source, "/"), strings.TrimPrefix(target, "/"))
		},
	}
}
func writeMutationText(root, path, text string, exclusive bool) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer r.Close()
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC | syscall.O_NOFOLLOW
	if exclusive {
		flags |= os.O_EXCL
	}
	file, err := r.OpenFile(strings.TrimPrefix(path, "/"), flags, 0644)
	if err != nil {
		return err
	}
	return writeMutationFile(file, text)
}
func writeMutationFile(file io.WriteCloser, text string) error {
	n, err := io.WriteString(file, text)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return closeErr
}
func (m WorktreeMutator) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now().UTC()
}
func (m WorktreeMutator) bounded(document repository.ConceptDocument, copied bool) (string, error) {
	var text string
	var err error
	if copied {
		text, err = repository.SerializeCopiedConcept(document)
	} else {
		text, err = repository.SerializeConcept(document)
	}
	if err != nil {
		return "", err
	}
	if len(text) > m.MaxConceptBytes {
		return "", &Error{"validation_error", "serialised concept exceeds max_concept_bytes"}
	}
	return text, nil
}
func mutationParent(path string) string {
	index := strings.LastIndex(path, "/")
	if index <= 0 {
		return "."
	}
	return path[:index]
}
func targetExists(path string, ops mutationIO) bool { _, err := ops.stat(path); return err == nil }

func (m WorktreeMutator) Create(root string, change ProposalChange, actor string) error {
	return m.create(root, change, actor, defaultMutationIO())
}
func (m WorktreeMutator) create(root string, change ProposalChange, actor string, ops mutationIO) error {
	path := change["path"].(string)
	target, err := repository.ValidateRepositoryWritePath(root, path)
	if err != nil {
		return err
	}
	if targetExists(target.AbsolutePath, ops) {
		return &Error{"conflict", "path already exists: " + path}
	}
	id, err := (&ProposalControls{Random: m.Random}).newOperationID()
	if err != nil {
		return err
	}
	metadata := map[string]any{"schema_version": 1, "id": id, "type": change["concept_type"], "title": change["title"], "status": "active", "description": change["description"], "aliases": change["aliases"], "tags": change["tags"], "source_refs": []string{}, "supersedes": []string{}, "created_at": m.now(), "updated_at": m.now(), "updated_by": actor}
	frontmatter, err := repository.ValidateConceptMetadata(metadata)
	if err != nil {
		return err
	}
	if err = ops.mkdir(root, mutationParent(path)); err != nil {
		return err
	}
	text, err := m.bounded(repository.ConceptDocument{Frontmatter: frontmatter, Body: change["body"].(string)}, false)
	if err != nil {
		return err
	}
	return ops.write(root, path, text, true)
}
func (m WorktreeMutator) Patch(root string, change ProposalChange, actor string) error {
	return m.patch(root, change, actor, defaultMutationIO())
}
func (m WorktreeMutator) patch(root string, change ProposalChange, actor string, ops mutationIO) error {
	path := change["path"].(string)
	entry, err := ops.read(root, path)
	if err != nil {
		return err
	}
	document := entry.Document
	meta := &document.Frontmatter
	if value := change["title"]; value != nil {
		meta.Title = value.(string)
	}
	if value := change["description"]; value != nil {
		text := value.(string)
		meta.Description = &text
	}
	if value := change["status"]; value != nil {
		meta.SetCopiedStatus(value.(string))
	}
	if value := change["tags"]; value != nil {
		meta.Tags = value.([]string)
	}
	if value := change["aliases"]; value != nil {
		meta.Aliases = value.([]string)
	}
	meta.UpdatedAt = m.now()
	meta.UpdatedBy = actor
	if value := change["body"]; value != nil {
		document.Body = value.(string)
	}
	if _, err = repository.ValidateRepositoryWritePath(root, path); err != nil {
		return err
	}
	text, err := m.bounded(document, true)
	if err != nil {
		return err
	}
	return ops.write(root, path, text, false)
}
func (m WorktreeMutator) Trash(root string, change ProposalChange, policy access.EffectivePolicy) ([]string, error) {
	return m.trash(root, change, policy, defaultMutationIO())
}
func (m WorktreeMutator) trash(root string, change ProposalChange, policy access.EffectivePolicy, ops mutationIO) ([]string, error) {
	path := change["path"].(string)
	if err := repository.ValidateBundlePath(path); err != nil {
		return nil, err
	}
	if strings.HasPrefix(path, "/trash/") || !strings.HasSuffix(path, ".md") {
		return nil, &repository.PathSafetyError{Message: "expected an active Markdown concept path"}
	}
	destination := "/trash" + path
	for _, path := range []string{path, destination} {
		for _, action := range []string{"read", "write"} {
			if _, err := access.AuthorizePath(policy, path, action); err != nil {
				return nil, err
			}
		}
	}
	if _, err := ops.read(root, path); err != nil {
		return nil, err
	}
	if _, err := repository.ValidateRepositoryWritePath(root, path); err != nil {
		return nil, err
	}
	target, err := repository.ValidateRepositoryWritePath(root, destination)
	if err != nil {
		return nil, err
	}
	if targetExists(target.AbsolutePath, ops) {
		return nil, &Error{"conflict", "trash destination already exists"}
	}
	if err = ops.mkdir(root, mutationParent(destination)); err != nil {
		return nil, err
	}
	if err = ops.rename(root, path, destination); err != nil {
		return nil, err
	}
	result := []string{path, destination}
	sort.Strings(result)
	return result, nil
}
func (m WorktreeMutator) Rename(root string, change ProposalChange, actor string, policy access.EffectivePolicy) ([]string, error) {
	return m.rename(root, change, actor, policy, defaultMutationIO())
}
func (m WorktreeMutator) rename(root string, change ProposalChange, actor string, policy access.EffectivePolicy, ops mutationIO) ([]string, error) {
	oldPath, newPath := change["path"].(string), change["new_path"].(string)
	old, err := repository.ValidateRepositoryWritePath(root, oldPath)
	if err != nil {
		return nil, err
	}
	target, err := repository.ValidateRepositoryWritePath(root, newPath)
	if err != nil {
		return nil, err
	}
	if !targetExists(old.AbsolutePath, ops) {
		return nil, &Error{"not_found", oldPath}
	}
	if targetExists(target.AbsolutePath, ops) {
		return nil, &Error{"conflict", "path already exists: " + newPath}
	}
	entry, err := ops.read(root, oldPath)
	if err != nil {
		return nil, err
	}
	paths, err := ops.list(root)
	if err != nil {
		return nil, err
	}
	type rewrite struct {
		path     string
		document repository.ConceptDocument
	}
	rewrites := []rewrite{}
	for _, path := range paths {
		if path == oldPath {
			continue
		}
		candidate, err := ops.read(root, path)
		if err != nil {
			return nil, err
		}
		rewritten := repository.RewriteLinksForRename(candidate.Document.Body, oldPath, newPath)
		if rewritten.Changed {
			if _, err = access.AuthorizePath(policy, path, "write"); err != nil {
				return nil, err
			}
			candidate.Document.Body = rewritten.Content
			rewrites = append(rewrites, rewrite{path, candidate.Document})
		}
	}
	entry.Document.Frontmatter.UpdatedAt = m.now()
	entry.Document.Frontmatter.UpdatedBy = actor
	if err = ops.mkdir(root, mutationParent(newPath)); err != nil {
		return nil, err
	}
	text, err := m.bounded(entry.Document, true)
	if err != nil {
		return nil, err
	}
	if err = ops.write(root, newPath, text, true); err != nil {
		return nil, err
	}
	if err = ops.remove(root, oldPath); err != nil {
		return nil, err
	}
	changed := []string{oldPath, newPath}
	for _, r := range rewrites {
		r.document.Frontmatter.UpdatedAt = m.now()
		r.document.Frontmatter.UpdatedBy = actor
		text, err := m.bounded(r.document, true)
		if err != nil {
			return nil, err
		}
		if err = ops.write(root, r.path, text, false); err != nil {
			return nil, err
		}
		changed = append(changed, r.path)
	}
	sort.Strings(changed)
	return changed, nil
}
