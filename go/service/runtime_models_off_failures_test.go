package service

import (
	"context"
	"errors"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
	"path/filepath"
	"testing"
)

func TestBuildModelsOffComponentFailures(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	for _, stage := range []string{"revision", "state-rebuild", "rebuild", "expire", "recover", "metadata", "register"} {
		t.Run(stage, func(t *testing.T) {
			var config RuntimeConfig
			config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
			options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}}
			ops := defaultModelsOffBuildOps()
			switch stage {
			case "revision":
				ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
			case "state-rebuild":
				ops.state = func(context.Context, *derived.Index) (derived.IndexState, error) { return derived.IndexState{}, boom }
				ops.rebuild = func(context.Context, *derived.Index, string, string) error { return boom }
			case "rebuild":
				ops.state = func(context.Context, *derived.Index) (derived.IndexState, error) { return derived.IndexState{}, nil }
				ops.rebuild = func(context.Context, *derived.Index, string, string) error { return boom }
			case "expire":
				ops.expire = func(context.Context, *assets.StagingStore) (int64, error) { return 0, boom }
			case "recover":
				ops.recover = func(context.Context, *repository.TransactionManager) ([]repository.RecoveryRecord, error) {
					return nil, boom
				}
			case "metadata":
				ops.metadata = func(string) (*ModelsOffMetadata, error) { return nil, boom }
			case "register":
				ops.register = func(*Jobs, *umcp.Server, string, execute.Limits) error { return boom }
			}
			runtime, server, err := buildModelsOffRuntime(ctx, config, options, ops)
			if !errors.Is(err, boom) || runtime != nil || server != nil {
				t.Fatal(runtime, server, err)
			}
			lease, err := repository.AcquireWriterLease(RuntimePathsFor(config).WriterLock, "retry")
			if err != nil {
				t.Fatal(err)
			}
			_ = lease.Release()
		})
	}
}
