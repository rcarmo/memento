package service

import (
	"context"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
	"testing"
)

func TestLegacySkillMigrationTransaction(t *testing.T) {
	ctx := context.Background()
	seed := t.TempDir()
	writeLegacySkill(t, seed, "demo", "1.0.0", "# Demo\n")
	var config RuntimeConfig
	config.Repository.RootPath = t.TempDir()
	runtime, err := BuildRuntimeStorage(ctx, config, seed)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	revision, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil {
		t.Fatal(err)
	}
	manager := repository.TransactionManager{Paths: runtime.Paths.Repository, Operations: control.Operations{DB: runtime.DB}}
	_, err = manager.Apply(ctx, repository.TransactionRequest{Operation: control.OperationRequest{OpID: "migrate-generic-asset-packs-v1", Principal: "memento-migration", IdempotencyKey: "migrate-generic-asset-packs-v1", ToolName: "internal_asset_migration", RequestJSON: `{"base_revision":"` + revision + `"}`}, ExpectedRevision: revision, CommitMessage: "memory: migrate skills to generic assets", AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}, func(_ context.Context, worktree string) ([]string, error) { return MigrateLegacySkillPacks(worktree) })
	if err != nil {
		t.Fatal(err)
	}
}
