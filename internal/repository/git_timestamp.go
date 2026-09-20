package repository

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// PathCommitTimestamp finds the most recent addition, not modification, at the
// exact path. Pure-Go equivalent of log -1 --diff-filter=A --format=%aI for
// ordinary linear history; merge-history simplification remains a parity edge.
func PathCommitTimestamp(ctx context.Context, paths GitRepositoryPaths, revision, path string) (time.Time, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return time.Time{}, &GitError{Message: "repository path must be an absolute repository file path"}
	}
	relative := strings.TrimPrefix(path, "/")
	for _, part := range strings.Split(relative, "/") {
		if part == "" || part == "." || part == ".." {
			return time.Time{}, &GitError{Message: "repository path must be a canonical file path"}
		}
	}
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		return time.Time{}, err
	}
	head, err := store.commit(revision)
	if err != nil {
		return time.Time{}, err
	}
	return pathAdditionTimestamp(ctx, head, path, relative, func(hash plumbing.Hash) (*object.Commit, error) { return object.GetCommit(store.storage, hash) })
}
func pathAdditionTimestamp(ctx context.Context, head *object.Commit, path, relative string, parentCommit func(plumbing.Hash) (*object.Commit, error)) (time.Time, error) {
	commits := object.NewCommitIterCTime(head, nil, nil)
	defer commits.Close()
	for {
		if err := ctx.Err(); err != nil {
			return time.Time{}, err
		}
		commit, err := commits.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return time.Time{}, err
		}
		present, err := commitHasPath(commit, relative)
		if err != nil {
			return time.Time{}, err
		}
		if !present {
			continue
		}
		added := true
		for _, parentHash := range commit.ParentHashes {
			parent, err := parentCommit(parentHash)
			if err != nil {
				return time.Time{}, err
			}
			exists, err := commitHasPath(parent, relative)
			if err != nil {
				return time.Time{}, err
			}
			if exists {
				added = false
				break
			}
		}
		if added {
			return commit.Author.When, nil
		}
	}
	return time.Time{}, &GitError{Message: "cannot determine creation timestamp for " + path}
}
func commitHasPath(commit *object.Commit, path string) (bool, error) {
	tree, err := commit.Tree()
	if err != nil {
		return false, err
	}
	_, err = tree.FindEntry(path)
	if errors.Is(err, object.ErrEntryNotFound) || errors.Is(err, object.ErrDirectoryNotFound) {
		return false, nil
	}
	return err == nil, err
}
