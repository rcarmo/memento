package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

type runtimeBuildOps struct {
	mkdir       func(string, os.FileMode) error
	stat        func(string) (os.FileInfo, error)
	lease       func(string, string) (*repository.WriterLease, error)
	bootstrap   func(context.Context, repository.GitRepositoryPaths, string) (repository.BootstrapResult, error)
	revision    func(repository.GitRepositoryPaths) (string, error)
	materialize func(context.Context, repository.GitRepositoryPaths, string) (repository.MaterializedCheckout, error)
	connect     func(context.Context, string) (*sql.DB, error)
	migrate     func(context.Context, *sql.DB) error
}

func defaultRuntimeBuildOps() runtimeBuildOps {
	return runtimeBuildOps{os.MkdirAll, os.Stat, repository.AcquireWriterLease, repository.BootstrapRepository, repository.GetMainRevision, repository.MaterializeCurrentCheckout, control.Connect, control.Migrate}
}
func BuildRuntimeStorage(ctx context.Context, config RuntimeConfig, bootstrapSeed string) (runtime *Runtime, err error) {
	return buildRuntimeStorage(ctx, config, bootstrapSeed, defaultRuntimeBuildOps())
}
func buildRuntimeStorage(ctx context.Context, config RuntimeConfig, bootstrapSeed string, ops runtimeBuildOps) (runtime *Runtime, err error) {
	paths := RuntimePathsFor(config)
	if err = ops.mkdir(paths.Root, 0777); err != nil {
		return nil, err
	}
	lease, err := ops.lease(paths.WriterLock, fmt.Sprintf("memento[%d]", os.Getpid()))
	if err != nil {
		return nil, err
	}
	runtime = &Runtime{Lease: lease}
	owner := runtime
	defer func() {
		if err != nil {
			err = errorsJoin(err, owner.Close(context.Background()))
			runtime = nil
		}
	}()
	if _, statErr := ops.stat(paths.Repository.BareDir); os.IsNotExist(statErr) {
		if _, err = ops.bootstrap(ctx, paths.Repository, bootstrapSeed); err != nil {
			return nil, err
		}
	} else if statErr != nil {
		return nil, statErr
	} else if _, statErr = ops.stat(paths.Repository.CurrentDir); os.IsNotExist(statErr) {
		var revision string
		if revision, err = ops.revision(paths.Repository); err == nil {
			_, err = ops.materialize(ctx, paths.Repository, revision)
		}
		if err != nil {
			return nil, err
		}
	} else if statErr != nil {
		return nil, statErr
	}
	runtime.DB, err = ops.connect(ctx, paths.ControlDB)
	if err != nil {
		return nil, err
	}
	if err = ops.migrate(ctx, runtime.DB); err != nil {
		return nil, err
	}
	return runtime, nil
}
func errorsJoin(primary, cleanup error) error { return errors.Join(primary, cleanup) }
