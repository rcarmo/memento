package derived

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

type embeddingRowsStub struct {
	next              bool
	scanErr, finalErr error
}

func (r *embeddingRowsStub) Next() bool {
	if r.next {
		r.next = false
		return true
	}
	return false
}
func (r *embeddingRowsStub) Scan(...any) error { return r.scanErr }
func (r *embeddingRowsStub) Err() error        { return r.finalErr }
func (r *embeddingRowsStub) Close() error      { return nil }
func TestEmbeddingPathRowsFailures(t *testing.T) {
	boom := errors.New("boom")
	if _, err := readEmbeddingPaths(&embeddingRowsStub{next: true, scanErr: boom}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := readEmbeddingPaths(&embeddingRowsStub{finalErr: boom}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	root := t.TempDir()
	installConcept(t, root)
	index := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	if err := index.Rebuild(context.Background(), root, "r"); err != nil {
		t.Fatal(err)
	}
	if _, err := index.pendingEmbeddingPaths(context.Background(), 1, func(context.Context, *sql.DB, int) (embeddingPathRows, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
func TestPendingEmbeddingPaths(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	index := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite"), DeferEmbeddings: true}
	if err := index.Rebuild(ctx, root, "r1"); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", index.Path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO concept_embeddings(concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,updated_at) SELECT id,path,'h','m',2,'r1','stale','v','now' FROM concepts LIMIT 1`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}
	got, err := index.PendingEmbeddingPaths(ctx, 10)
	if err != nil || len(got) == 0 {
		t.Fatal(got, err)
	}
	one, err := index.PendingEmbeddingPaths(ctx, 1)
	if err != nil || !reflect.DeepEqual(one, got[:1]) {
		t.Fatal(one, got, err)
	}
	if _, err = index.PendingEmbeddingPaths(ctx, 0); err == nil {
		t.Fatal("limit")
	}
	var wait sync.WaitGroup
	for range 10 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			paths, e := index.PendingEmbeddingPaths(ctx, 10)
			if e != nil || !reflect.DeepEqual(paths, got) {
				t.Error(paths, e)
			}
		}()
	}
	wait.Wait()
}
