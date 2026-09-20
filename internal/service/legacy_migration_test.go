package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func TestRuntimeMigratesLegacyBlobsOnce(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}}
	runtime, _, err := BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	blob := []byte("legacy asset bytes")
	digest := fmt.Sprintf("%x", sha256.Sum256(blob))
	object := filepath.Join(runtime.Paths.Repository.BareDir, "lfs", "objects", digest[:2], digest[2:4], digest)
	if err = os.MkdirAll(filepath.Dir(object), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(object, blob, 0600); err != nil {
		t.Fatal(err)
	}
	manager := repository.TransactionManager{Paths: runtime.Paths.Repository, Operations: control.Operations{DB: runtime.DB}}
	revision, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil {
		t.Fatal(err)
	}
	pointer := fmt.Sprintf("version https://git-lfs.github.com/spec/v1\noid sha256:%s\nsize %d\n", digest, len(blob))
	_, err = manager.Apply(ctx, repository.TransactionRequest{Operation: control.OperationRequest{OpID: "seed-legacy", Principal: "test", IdempotencyKey: "seed-legacy", ToolName: "test", RequestJSON: "{}"}, ExpectedRevision: revision, CommitMessage: "seed legacy", AuthorName: "Test", AuthorEmail: "test@example.invalid"}, func(_ context.Context, worktree string) ([]string, error) {
		if err := os.WriteFile(filepath.Join(worktree, "asset.bin"), []byte(pointer), 0600); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(worktree, ".gitattributes"), []byte("*.bin filter=lfs diff=lfs\n"), 0600); err != nil {
			return nil, err
		}
		return []string{"/.gitattributes", "/asset.bin"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	legacyRevision, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	runtime, _, err = BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	migratedRevision, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil || migratedRevision == legacyRevision {
		t.Fatal(legacyRevision, migratedRevision, err)
	}
	raw, err := os.ReadFile(filepath.Join(runtime.Paths.Repository.CurrentDir, "asset.bin"))
	if err != nil || string(raw) != string(blob) {
		t.Fatal(string(raw), err)
	}
	if _, err = os.Stat(filepath.Join(runtime.Paths.Repository.CurrentDir, ".gitattributes")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	record, err := (control.Operations{DB: runtime.DB}).Get(ctx, "migrate-legacy-blobs-to-git-v1")
	if err != nil || record.State != control.Succeeded {
		t.Fatal(record, err)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	runtime, _, err = BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	restartRevision, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil || restartRevision != migratedRevision {
		t.Fatal(restartRevision, migratedRevision, err)
	}
}
