package repository

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestPathCommitTimestampReference(t *testing.T) {
	paths, _ := fixtureGit(t)
	if output, err := exec.Command("git", "init", "--bare", paths.BareDir).CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var parent []string
	revisions := []string{}
	for i, body := range []string{"created", "changed", "", "recreated", "latest"} {
		entries := []GitTreeEntry{}
		if body != "" {
			hash, err := store.WriteBlob([]byte(body))
			if err != nil {
				t.Fatal(err)
			}
			entries = append(entries, GitTreeEntry{Path: "a.txt", Mode: filemode.Regular, Hash: hash})
		}
		tree, err := store.WriteTree(ctx, entries)
		if err != nil {
			t.Fatal(err)
		}
		stamp := time.Date(2026, 1, i+1, 3, 4, 5, 0, time.FixedZone("offset", 3600))
		author := GitCommitIdentity{Name: "Synthetic", Email: "synthetic@example.invalid", When: stamp}
		committer := author
		committer.When = stamp.Add(2 * time.Hour)
		hash, err := store.WriteCommit(tree, parent, "synthetic", author, committer)
		if err != nil {
			t.Fatal(err)
		}
		parent = []string{hash}
		revisions = append(revisions, hash)
	}
	for _, revision := range revisions {
		expected, err := exec.Command("git", "--git-dir", paths.BareDir, "log", "-1", "--diff-filter=A", "--format=%aI", revision, "--", "a.txt").Output()
		if err != nil {
			t.Fatal(err)
		}
		got, err := PathCommitTimestamp(ctx, paths, revision, "/a.txt")
		if err != nil || got.Format(time.RFC3339) != strings.TrimSpace(string(expected)) {
			t.Fatal(revision, got, err, string(expected))
		}
	}
	for _, path := range []string{"a.txt", "//a.txt", "/", "/a/../b", "/missing", "/dir/missing"} {
		if _, err := PathCommitTimestamp(ctx, paths, revisions[4], path); err == nil {
			t.Fatal(path)
		}
	}
	if _, err := PathCommitTimestamp(ctx, paths, "bad", "/a.txt"); err == nil {
		t.Fatal("bad revision")
	}
	if _, err := PathCommitTimestamp(ctx, GitRepositoryPaths{BareDir: t.TempDir()}, revisions[0], "/a.txt"); err == nil {
		t.Fatal("missing objects")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := PathCommitTimestamp(cancelled, paths, revisions[0], "/a.txt"); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestTimestampCorruptHistory(t *testing.T) {
	paths, _ := fixtureGit(t)
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	missing := plumbing.NewHash(strings.Repeat("a", 40))
	author := GitCommitIdentity{Name: "Synthetic", Email: "x@example.invalid", When: time.Now()}
	blob, err := store.WriteBlob([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := store.WriteTree(ctx, []GitTreeEntry{{Path: "a", Mode: filemode.Regular, Hash: blob}})
	if err != nil {
		t.Fatal(err)
	}
	head, err := writeCorruptTimestampCommit(store, tree, []plumbing.Hash{missing}, author)
	if err != nil {
		t.Fatal(err)
	}
	// Present head path forces the direct parent probe; absent path exercises
	// the iterator's parent decoding failure on the next visit.
	for _, path := range []string{"/a", "/absent"} {
		if _, err := PathCommitTimestamp(ctx, paths, head, path); err == nil {
			t.Fatal(path)
		}
	}
	badTree, err := writeCorruptTimestampCommit(store, missing, nil, author)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PathCommitTimestamp(ctx, paths, badTree, "/a"); err == nil {
		t.Fatal("bad tree")
	}
	// A missing subtree must propagate corruption, not masquerade as absence.
	encoded := store.storage.NewEncodedObject()
	bad := object.Tree{Entries: []object.TreeEntry{{Name: "dir", Mode: filemode.Dir, Hash: missing}}}
	if err = bad.Encode(encoded); err != nil {
		t.Fatal(err)
	}
	hash, err := store.storage.SetEncodedObject(encoded)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := store.WriteCommit(hash, nil, "bad subtree", author, author)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PathCommitTimestamp(ctx, paths, commit, "/dir/a"); err == nil {
		t.Fatal("bad subtree")
	}
	// Parent with a corrupt tree is distinct from an absent parent commit.
	child, err := store.WriteCommit(tree, []string{badTree}, "bad parent tree", author, author)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PathCommitTimestamp(ctx, paths, child, "/a"); err == nil {
		t.Fatal("parent tree")
	}
	// The iterator has already loaded each parent; deletion between traversal
	// and the per-path parent probe still propagates rather than inventing an add.
	valid, err := store.WriteCommit(tree, nil, "valid", author, author)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := store.WriteCommit(tree, []string{valid}, "probe", author, author)
	if err != nil {
		t.Fatal(err)
	}
	probeCommit, err := store.commit(probe)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pathAdditionTimestamp(ctx, probeCommit, "/a", "a", func(plumbing.Hash) (*object.Commit, error) { return nil, io.ErrClosedPipe }); err != io.ErrClosedPipe {
		t.Fatal(err)
	}
	// Ensure no native Git subprocess is required by the implementation.
	t.Setenv("PATH", t.TempDir())
	root, err := store.WriteCommit(tree, nil, "root", author, author)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PathCommitTimestamp(ctx, paths, root, "/a"); err != nil {
		t.Fatal(err)
	}
}

func writeCorruptTimestampCommit(store *GitStore, tree plumbing.Hash, parents []plumbing.Hash, identity GitCommitIdentity) (string, error) {
	signature := object.Signature{Name: identity.Name, Email: identity.Email, When: identity.When}
	hash, err := store.writeObject(&object.Commit{TreeHash: tree, ParentHashes: parents, Author: signature, Committer: signature, Message: "synthetic corrupt history"})
	return hash.String(), err
}
