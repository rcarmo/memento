package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildRuntimeStorageFailureCleanup(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	for _, stage := range []string{"bootstrap", "revision", "materialize", "connect", "migrate"} {
		t.Run(stage, func(t *testing.T) {
			var config RuntimeConfig
			config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
			paths := RuntimePathsFor(config)
			ops := defaultRuntimeBuildOps()
			var opened *sql.DB
			if stage == "revision" || stage == "materialize" {
				if _, err := repository.BootstrapRepository(ctx, paths.Repository, ""); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(paths.Repository.CurrentDir); err != nil {
					t.Fatal(err)
				}
			}
			switch stage {
			case "bootstrap":
				ops.bootstrap = func(context.Context, repository.GitRepositoryPaths, string) (repository.BootstrapResult, error) {
					return repository.BootstrapResult{}, boom
				}
			case "revision":
				ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
			case "materialize":
				ops.materialize = func(context.Context, repository.GitRepositoryPaths, string) (repository.MaterializedCheckout, error) {
					return repository.MaterializedCheckout{}, boom
				}
			case "connect":
				ops.connect = func(context.Context, string) (*sql.DB, error) { return nil, boom }
			case "migrate":
				ops.migrate = func(context.Context, *sql.DB) error { return boom }
				ops.connect = func(ctx context.Context, path string) (*sql.DB, error) {
					var err error
					opened, err = control.Connect(ctx, path)
					return opened, err
				}
			}
			if _, err := buildRuntimeStorage(ctx, config, "", ops); !errors.Is(err, boom) {
				t.Fatal(err)
			}
			lease, err := repository.AcquireWriterLease(paths.WriterLock, "retry")
			if err != nil {
				t.Fatal(err)
			}
			_ = lease.Release()
			if opened != nil && opened.Ping() == nil {
				t.Fatal("database open")
			}
		})
	}
}
func TestBuildRuntimeStorageStatFailures(t *testing.T) {
	ctx := context.Background()
	for _, target := range []string{"repo", "current"} {
		var config RuntimeConfig
		config.Repository.RootPath = filepath.Join(t.TempDir(), target)
		paths := RuntimePathsFor(config)
		if target == "current" {
			if _, err := repository.BootstrapRepository(ctx, paths.Repository, ""); err != nil {
				t.Fatal(err)
			}
		}
		ops := defaultRuntimeBuildOps()
		realStat := ops.stat
		ops.stat = func(path string) (os.FileInfo, error) {
			if (target == "repo" && path == paths.Repository.BareDir) || (target == "current" && path == paths.Repository.CurrentDir) {
				return nil, errors.New("stat")
			}
			return realStat(path)
		}
		if _, err := buildRuntimeStorage(ctx, config, "", ops); err == nil {
			t.Fatal(target)
		}
	}
}
