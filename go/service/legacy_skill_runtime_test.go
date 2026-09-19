package service

import (
	"context"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeMigratesLegacySkillPacksOnce(t *testing.T) {
	ctx := context.Background()
	seed := t.TempDir()
	writeLegacySkill(t, seed, "demo", "1.0.0", "# Demo\n")
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}, BootstrapSeed: seed}
	runtime, _, err := BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	migratedRevision, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(runtime.Paths.Repository.CurrentDir, ".assets", "12345678-abcd", "skill", "1.0.0.json")); err != nil {
		t.Fatal(err)
	}
	record, err := (control.Operations{DB: runtime.DB}).Get(ctx, "migrate-generic-asset-packs-v1")
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
