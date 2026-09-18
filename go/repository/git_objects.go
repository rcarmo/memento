package repository

import (
	"context"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitCommitIdentity is explicit so oracle commits and recovery preserve both
// author and committer timestamps, offsets and attribution without environment
// variables leaking into runtime writes.
type GitCommitIdentity struct {
	Name, Email string
	When        time.Time
}

func (s *GitStore) WriteBlob(content []byte) (plumbing.Hash, error) {
	encoded := s.storage.NewEncodedObject()
	encoded.SetType(plumbing.BlobObject)
	encoded.SetSize(int64(len(content)))
	writer, err := encoded.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	n, writeErr := writer.Write(content)
	closeErr := writer.Close()
	if writeErr != nil {
		return plumbing.ZeroHash, writeErr
	}
	if n != len(content) {
		return plumbing.ZeroHash, io.ErrShortWrite
	}
	if closeErr != nil {
		return plumbing.ZeroHash, closeErr
	}
	return s.storage.SetEncodedObject(encoded)
}
func (s *GitStore) writeObject(value interface {
	Encode(plumbing.EncodedObject) error
}) (plumbing.Hash, error) {
	encoded := s.storage.NewEncodedObject()
	if err := value.Encode(encoded); err != nil {
		return plumbing.ZeroHash, err
	}
	return s.storage.SetEncodedObject(encoded)
}

// WriteTree builds canonical Git trees from a flat snapshot, preserving exact
// modes and blob hashes. Existing objects are required; no submodule fetching
// occurs. Canonical order uses Git's directory-aware byte sort, not plain names.
func (s *GitStore) WriteTree(ctx context.Context, entries []GitTreeEntry) (plumbing.Hash, error) {
	if err := validateCheckoutEntries(entries); err != nil {
		return plumbing.ZeroHash, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return plumbing.ZeroHash, err
		}
		if entry.Mode == filemode.Submodule {
			continue
		}
		if _, err := s.storage.EncodedObject(plumbing.BlobObject, entry.Hash); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	var build func([]GitTreeEntry) (plumbing.Hash, error)
	build = func(items []GitTreeEntry) (plumbing.Hash, error) {
		if err := ctx.Err(); err != nil {
			return plumbing.ZeroHash, err
		}
		tree := &object.Tree{}
		groups := map[string][]GitTreeEntry{}
		for _, entry := range items {
			first, rest, nested := strings.Cut(entry.Path, "/")
			if nested {
				entry.Path = rest
				groups[first] = append(groups[first], entry)
			} else {
				tree.Entries = append(tree.Entries, object.TreeEntry{Name: first, Mode: entry.Mode, Hash: entry.Hash})
			}
		}
		names := []string{}
		for name := range groups {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			hash, err := build(groups[name])
			if err != nil {
				return plumbing.ZeroHash, err
			}
			tree.Entries = append(tree.Entries, object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: hash})
		}
		sort.Sort(object.TreeEntrySorter(tree.Entries))
		return s.writeObject(tree)
	}
	return build(entries)
}

// WriteCommit stores a detached commit object only. It neither updates HEAD nor
// publishes main; those are separate locked transitions with their own errors.
func (s *GitStore) WriteCommit(tree plumbing.Hash, parents []string, message string, author, committer GitCommitIdentity) (string, error) {
	for _, identity := range []GitCommitIdentity{author, committer} {
		if strings.TrimSpace(identity.Name) == "" || strings.TrimSpace(identity.Email) == "" || strings.ContainsAny(identity.Name+identity.Email, "\x00\r\n<>") {
			return "", &GitError{Message: "invalid Git commit identity"}
		}
	}
	if strings.ContainsRune(message, 0) {
		return "", &GitError{Message: "invalid Git commit message"}
	}
	if _, err := object.GetTree(s.storage, tree); err != nil {
		return "", err
	}
	hashes := []plumbing.Hash{}
	for _, parent := range parents {
		commit, err := s.commit(parent)
		if err != nil {
			return "", err
		}
		hashes = append(hashes, commit.Hash)
	}
	commit := &object.Commit{TreeHash: tree, ParentHashes: hashes, Message: message, Author: object.Signature{Name: author.Name, Email: author.Email, When: author.When}, Committer: object.Signature{Name: committer.Name, Email: committer.Email, When: committer.When}}
	hash, err := s.writeObject(commit)
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}
