package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
)

func TestGitObjectEncodingReference(t *testing.T) {
	paths, f := fixtureGit(t)
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../testdata/parity/repository-git.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Objects map[string]struct {
			Commit []byte
			Tree   string
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, revision := range []string{f.Base, f.Next} {
		entries, err := store.TreeEntries(context.Background(), revision)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			data, err := store.ReadBlob(revision, "/"+entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			hash, err := store.WriteBlob(data)
			if err != nil || hash != entry.Hash {
				t.Fatal(hash, entry.Hash, err)
			}
		}
		tree, err := store.WriteTree(context.Background(), entries)
		if err != nil || tree.String() != fixture.Objects[revision].Tree {
			t.Fatal(tree, err)
		}
		original, err := store.commit(revision)
		if err != nil {
			t.Fatal(err)
		}
		parents := []string{}
		for _, p := range original.ParentHashes {
			parents = append(parents, p.String())
		}
		got, err := store.WriteCommit(tree, parents, original.Message, GitCommitIdentity{original.Author.Name, original.Author.Email, original.Author.When}, GitCommitIdentity{original.Committer.Name, original.Committer.Email, original.Committer.When})
		if err != nil || got != revision {
			t.Fatal(got, revision, err)
		}
		encoded, err := store.storage.EncodedObject(plumbing.CommitObject, plumbing.NewHash(got))
		if err != nil {
			t.Fatal(err)
		}
		reader, err := encoded.Reader()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || string(data) != string(fixture.Objects[revision].Commit) {
			t.Fatal(string(data), err)
		}
	}
	empty, err := store.WriteTree(context.Background(), nil)
	if err != nil || empty.String() != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		t.Fatal(empty, err)
	}
}

type objectFaultStorage struct {
	storage.Storer
	fail string
	sets int
}

func (s *objectFaultStorage) NewEncodedObject() plumbing.EncodedObject {
	return &objectFault{EncodedObject: s.Storer.NewEncodedObject(), fail: s.fail}
}
func (s *objectFaultStorage) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	s.sets++
	if s.fail == "store" || s.fail == "child" && s.sets == 1 {
		return plumbing.ZeroHash, io.ErrClosedPipe
	}
	return s.Storer.SetEncodedObject(o)
}

type objectFault struct {
	plumbing.EncodedObject
	fail string
}

func (o *objectFault) Writer() (io.WriteCloser, error) {
	if o.fail == "writer" {
		return nil, io.ErrClosedPipe
	}
	w, err := o.EncodedObject.Writer()
	return objectFaultWriter{w, o.fail}, err
}

type objectFaultWriter struct {
	io.WriteCloser
	fail string
}

func (w objectFaultWriter) Write(p []byte) (int, error) {
	if w.fail == "write" {
		return 0, io.ErrClosedPipe
	}
	if w.fail == "short" {
		return 0, nil
	}
	return w.WriteCloser.Write(p)
}
func (w objectFaultWriter) Close() error {
	err := w.WriteCloser.Close()
	if w.fail == "close" {
		return io.ErrClosedPipe
	}
	return err
}
func TestGitObjectWriteErrors(t *testing.T) {
	for _, kind := range []string{"writer", "write", "short", "close", "store"} {
		s := &GitStore{storage: &objectFaultStorage{Storer: memory.NewStorage(), fail: kind}}
		if _, err := s.WriteBlob([]byte("test")); err == nil {
			t.Fatal(kind)
		}
	}
	m := memory.NewStorage()
	s := &GitStore{storage: m}
	blob, err := s.WriteBlob([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	entries := []GitTreeEntry{{Path: "dir/file", Mode: filemode.Regular, Hash: blob}}
	for _, kind := range []string{"writer", "write", "close", "store", "child"} {
		broken := &GitStore{storage: &objectFaultStorage{Storer: m, fail: kind}}
		if _, err := broken.WriteTree(context.Background(), entries); err == nil {
			t.Fatal(kind)
		}
	}
	if _, err = s.WriteTree(context.Background(), []GitTreeEntry{{Path: "../bad", Mode: filemode.Regular}}); err == nil {
		t.Fatal("bad path")
	}
	if _, err = s.WriteTree(context.Background(), []GitTreeEntry{{Path: "missing", Mode: filemode.Regular}}); err == nil {
		t.Fatal("missing blob")
	}
	if _, err = s.WriteTree(context.Background(), []GitTreeEntry{{Path: "module", Mode: filemode.Submodule}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.WriteTree(ctx, entries); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = s.WriteTree(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	tree, err := s.WriteTree(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	identity := GitCommitIdentity{"Test", "test@example.invalid", time.Unix(0, 0).UTC()}
	for _, bad := range []GitCommitIdentity{{"", "x", time.Time{}}, {"x", "", time.Time{}}, {"x\n", "e", time.Time{}}, {"<x>", "e", time.Time{}}} {
		if _, err = s.WriteCommit(tree, nil, "message", bad, identity); err == nil {
			t.Fatal(bad)
		}
	}
	if _, err = s.WriteCommit(tree, nil, "bad\x00", identity, identity); err == nil {
		t.Fatal("bad message")
	}
	if _, err = s.WriteCommit(plumbing.ZeroHash, nil, "message", identity, identity); err == nil {
		t.Fatal("bad tree")
	}
	if _, err = s.WriteCommit(tree, []string{"bad"}, "message", identity, identity); err == nil {
		t.Fatal("bad parent")
	}
	broken := &GitStore{storage: &objectFaultStorage{Storer: m, fail: "store"}}
	if _, err = broken.WriteCommit(tree, nil, "message", identity, identity); err == nil {
		t.Fatal("commit failure")
	}
	badTree := &object.Tree{Entries: []object.TreeEntry{{Name: "z"}, {Name: "a"}}}
	if _, err = s.writeObject(badTree); err == nil {
		t.Fatal("invalid encoding")
	}
}
func FuzzGitObjectRoundTrip(f *testing.F) {
	for _, seed := range []string{"", "test", "\x00\xff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data string) {
		if len(data) > 65536 {
			return
		}
		s := &GitStore{storage: memory.NewStorage()}
		hash, err := s.WriteBlob([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		blob, err := object.GetBlob(s.storage, hash)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := blob.Reader()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || string(raw) != data {
			t.Fatal(err)
		}
		_, _ = s.WriteTree(context.Background(), []GitTreeEntry{{Path: strings.Repeat("a", 1), Mode: filemode.Regular, Hash: hash}})
	})
}
