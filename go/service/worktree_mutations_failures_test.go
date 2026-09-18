package service

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/repository"
)

const mutationConcept = "---\nid: '12345678'\ntype: concept\ntitle: Title\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: original\n---\n[target](/public/a.md)\n"

func mutationTest(t *testing.T) (string, WorktreeMutator) {
	t.Helper()
	root := t.TempDir()
	installMutationFiles(t, root, map[string]string{"/public/a.md": mutationConcept, "/public/ref.md": mutationConcept})
	if _, err := repository.ReadBundleEntry(root, "/public/a.md"); err != nil {
		t.Fatal(err)
	}
	return root, WorktreeMutator{MaxConceptBytes: 65536, Random: bytes.NewReader(make([]byte, 16))}
}
func normalizedMutation(t *testing.T, raw map[string]any) ProposalChange {
	t.Helper()
	changes, err := NormalizeProposalChanges([]any{raw})
	if err != nil {
		t.Fatal(err)
	}
	return changes[0]
}
func TestMutationCreatePatchFailures(t *testing.T) {
	for _, stage := range []string{"path", "random", "mkdir", "write", "root-file"} {
		t.Run("create/"+stage, func(t *testing.T) {
			root, m := mutationTest(t)
			ops := defaultMutationIO()
			change := normalizedMutation(t, map[string]any{"kind": "create", "path": "/new/a.md", "concept_type": "concept", "title": "Title", "body": "Body"})
			switch stage {
			case "path":
				change["path"] = "relative"
			case "random":
				m.Random = bytes.NewReader(nil)
			case "mkdir":
				ops.mkdir = func(string, string) error { return io.ErrClosedPipe }
			case "write":
				ops.write = func(string, string, string, bool) error { return io.ErrClosedPipe }
			case "root-file":
				root = filepath.Join(root, "public/a.md")
			}
			if err := m.create(root, change, "actor", ops); err == nil {
				t.Fatal("missing failure")
			}
		})
	}
	root, m := mutationTest(t)
	change := normalizedMutation(t, map[string]any{"kind": "create", "path": "/root.md", "concept_type": "concept", "title": "Title", "body": "Body"})
	if err := m.Create(root, change, "actor"); err != nil {
		t.Fatal(err)
	}
	if m.now().IsZero() {
		t.Fatal("clock")
	}
	ops := defaultMutationIO()
	ops.read = func(root, path string) (repository.BundleEntry, error) {
		entry, err := repository.ReadBundleEntry(root, path)
		if err == nil {
			if err = os.Remove(filepath.Join(root, "public/a.md")); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink("ref.md", filepath.Join(root, "public/a.md")); err != nil {
				t.Fatal(err)
			}
		}
		return entry, err
	}
	if err := m.patch(root, ProposalChange{"path": "/public/a.md"}, "actor", ops); err == nil {
		t.Fatal("replacement symlink")
	}
}
func TestMutationTrashRenameFailures(t *testing.T) {
	for _, kind := range []string{"trash", "rename"} {
		for _, stage := range []string{"old-path", "new-path", "read", "list", "mkdir", "write", "remove", "rewrite", "rename", "source-replaced"} {
			if kind == "trash" && (stage == "list" || stage == "write" || stage == "remove" || stage == "rewrite") {
				continue
			}
			if kind == "rename" && (stage == "rename" || stage == "source-replaced") {
				continue
			}
			t.Run(kind+"/"+stage, func(t *testing.T) {
				root, m := mutationTest(t)
				ops := defaultMutationIO()
				change := ProposalChange{"kind": kind, "path": "/public/a.md", "new_path": "/public/new/a.md"}
				switch stage {
				case "old-path":
					change["path"] = "relative"
				case "new-path":
					if kind == "rename" {
						change["new_path"] = "relative"
					} else {
						if err := os.Symlink(t.TempDir(), filepath.Join(root, "trash")); err != nil {
							t.Fatal(err)
						}
					}
				case "read":
					ops.read = func(string, string) (repository.BundleEntry, error) {
						return repository.BundleEntry{}, io.ErrClosedPipe
					}
				case "list":
					ops.list = func(string) ([]string, error) { return nil, io.ErrClosedPipe }
				case "mkdir":
					ops.mkdir = func(string, string) error { return io.ErrClosedPipe }
				case "write":
					ops.write = func(string, string, string, bool) error { return io.ErrClosedPipe }
				case "remove":
					ops.remove = func(string, string) error { return io.ErrClosedPipe }
				case "rewrite":
					ops.write = func(root, path, text string, exclusive bool) error {
						if !exclusive {
							return io.ErrClosedPipe
						}
						return writeMutationText(root, path, text, exclusive)
					}
				case "rename":
					ops.rename = func(string, string, string) error { return io.ErrClosedPipe }
				case "source-replaced":
					ops.read = func(root, path string) (repository.BundleEntry, error) {
						entry, err := repository.ReadBundleEntry(root, path)
						if err == nil {
							if err = os.Remove(filepath.Join(root, "public/a.md")); err != nil {
								t.Fatal(err)
							}
							if err = os.Symlink("ref.md", filepath.Join(root, "public/a.md")); err != nil {
								t.Fatal(err)
							}
						}
						return entry, err
					}
				}
				var err error
				if kind == "trash" {
					_, err = m.trash(root, change, archivePolicy(), ops)
				} else {
					_, err = m.rename(root, change, "actor", archivePolicy(), ops)
				}
				if err == nil {
					t.Fatal("missing failure")
				}
			})
		}
	}
}

type failedMutationWriter struct {
	count              int
	writeErr, closeErr error
	closed             bool
}

func (w *failedMutationWriter) Write(data []byte) (int, error) { return w.count, w.writeErr }
func (w *failedMutationWriter) Close() error                   { w.closed = true; return w.closeErr }
func TestMutationFilesystemBoundaries(t *testing.T) {
	ops := defaultMutationIO()
	missing := filepath.Join(t.TempDir(), "absent")
	for _, err := range []error{ops.mkdir(missing, "dir"), ops.remove(missing, "file"), ops.rename(missing, "a", "b"), ops.write(missing, "a", "text", true)} {
		if err == nil {
			t.Fatal("missing root")
		}
	}
	for _, test := range []struct {
		writer failedMutationWriter
		want   error
	}{{failedMutationWriter{writeErr: io.ErrClosedPipe}, io.ErrClosedPipe}, {failedMutationWriter{}, io.ErrShortWrite}, {failedMutationWriter{count: 4, closeErr: io.ErrClosedPipe}, io.ErrClosedPipe}} {
		err := writeMutationFile(&test.writer, "text")
		if !errors.Is(err, test.want) || !test.writer.closed {
			t.Fatal(err, test)
		}
	}
	root, m := mutationTest(t)
	outside := t.TempDir()
	target := filepath.Join(outside, "victim.md")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link.md")); err != nil {
		t.Fatal(err)
	}
	for _, exclusive := range []bool{false, true} {
		if err := writeMutationText(root, "/link.md", "overwrite", exclusive); err == nil {
			t.Fatal("symlink write")
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := writeMutationText(root, "/escape/victim.md", "overwrite", false); err == nil {
		t.Fatal("ancestor escape")
	}
	if content, err := os.ReadFile(target); err != nil || string(content) != "untouched" {
		t.Fatal(string(content), err)
	}
	// Rooted exclusive creation also prevents replacement between validation and
	// write; a failed transaction owns cleanup of any partial worktree changes.
	change := normalizedMutation(t, map[string]any{"kind": "create", "path": "/collision.md", "concept_type": "concept", "title": "Title", "body": "Body"})
	ops.mkdir = func(root, path string) error {
		return os.WriteFile(filepath.Join(root, "collision.md"), []byte("successor"), 0600)
	}
	if err := m.create(root, change, "actor", ops); err == nil {
		t.Fatal("exclusive collision")
	}
	if content, _ := os.ReadFile(filepath.Join(root, "collision.md")); string(content) != "successor" {
		t.Fatal(string(content))
	}
	// Serialization is bounded in bytes, not Unicode code points.
	doc := repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{SchemaVersion: 1, ID: "12345678", Type: "concept", Title: "Title", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now(), UpdatedBy: "actor"}, Body: "😀"}
	text, err := m.bounded(doc, false)
	if err != nil {
		t.Fatal(err)
	}
	m.MaxConceptBytes = len(text)
	if _, err = m.bounded(doc, false); err != nil {
		t.Fatal(err)
	}
	m.MaxConceptBytes--
	if _, err = m.bounded(doc, false); err == nil {
		t.Fatal("byte cap")
	}
}
