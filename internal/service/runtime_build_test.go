package service

import (
	"context"
	"github.com/rcarmo/memento/internal/repository"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildRuntimeStorageLifecycle(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	runtime, err := BuildRuntimeStorage(ctx, config, "")
	if err != nil {
		t.Fatal(err)
	}
	paths := RuntimePathsFor(config)
	for _, path := range []string{paths.Repository.BareDir, paths.Repository.CurrentDir, paths.ControlDB, paths.WriterLock} {
		if _, err = os.Stat(path); err != nil {
			t.Fatal(path, err)
		}
	}
	var version string
	if err = runtime.DB.QueryRow("SELECT value FROM service_state WHERE key='schema_version'").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.AcquireWriterLease(paths.WriterLock, "other"); err == nil {
		t.Fatal("lease not held")
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(paths.Repository.CurrentDir); err != nil {
		t.Fatal(err)
	}
	runtime, err = BuildRuntimeStorage(ctx, config, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(paths.Repository.CurrentDir); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestBuildRuntimeStorageErrors(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(config.Repository.RootPath, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildRuntimeStorage(ctx, config, ""); err == nil {
		t.Fatal("root file")
	}
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	runtime, err := BuildRuntimeStorage(ctx, config, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = BuildRuntimeStorage(ctx, config, ""); err == nil {
		t.Fatal("lease")
	}
	_ = runtime.Close(ctx)
}
