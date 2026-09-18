package service

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

type ModelsOffRuntimeOptions struct {
	Surface       string
	Limits        execute.Limits
	BootstrapSeed string
	Tokens        []BearerPrincipal
}
type modelsOffBuildOps struct {
	storage  func(context.Context, RuntimeConfig, string) (*Runtime, error)
	revision func(repository.GitRepositoryPaths) (string, error)
	state    func(context.Context, *derived.Index) (derived.IndexState, error)
	rebuild  func(context.Context, *derived.Index, string, string) error
	expire   func(context.Context, *assets.StagingStore) (int64, error)
	recover  func(context.Context, *repository.TransactionManager) ([]repository.RecoveryRecord, error)
	metadata func(string) (*ModelsOffMetadata, error)
	register func(*Jobs, *umcp.Server, string, execute.Limits) error
}

func defaultModelsOffBuildOps() modelsOffBuildOps {
	return modelsOffBuildOps{BuildRuntimeStorage, repository.GetMainRevision, func(ctx context.Context, i *derived.Index) (derived.IndexState, error) { return i.State(ctx) }, func(ctx context.Context, i *derived.Index, root, revision string) error {
		return i.Rebuild(ctx, root, revision)
	}, func(ctx context.Context, s *assets.StagingStore) (int64, error) { return s.Expire(ctx) }, func(ctx context.Context, m *repository.TransactionManager) ([]repository.RecoveryRecord, error) {
		return m.RecoverStartup(ctx)
	}, NewModelsOffMetadata, func(j *Jobs, s *umcp.Server, surface string, limits execute.Limits) error {
		return j.RegisterModelsOffServer(s, surface, limits, nil)
	}}
}
func BuildModelsOffRuntime(ctx context.Context, config RuntimeConfig, options ModelsOffRuntimeOptions) (*Runtime, *umcp.Server, error) {
	return buildModelsOffRuntime(ctx, config, options, defaultModelsOffBuildOps())
}
func buildModelsOffRuntime(ctx context.Context, config RuntimeConfig, options ModelsOffRuntimeOptions, ops modelsOffBuildOps) (runtime *Runtime, server *umcp.Server, err error) {
	runtime, err = ops.storage(ctx, config, options.BootstrapSeed)
	if err != nil {
		return nil, nil, err
	}
	owner := runtime
	defer func() {
		if err != nil {
			err = errors.Join(err, owner.Close(context.Background()))
			runtime = nil
			server = nil
		}
	}()
	paths := runtime.Paths
	revision, err := ops.revision(paths.Repository)
	if err != nil {
		return nil, nil, err
	}
	index := &derived.Index{Path: paths.DerivedDB}
	state, stateErr := ops.state(ctx, index)
	if stateErr != nil || state.IndexRevision == "" {
		if err = ops.rebuild(ctx, index, paths.Repository.CurrentDir, revision); err != nil {
			return nil, nil, err
		}
	}
	staging := &assets.StagingStore{DB: runtime.DB, Random: rand.Reader}
	if _, err = ops.expire(ctx, staging); err != nil {
		return nil, nil, err
	}
	manager := repository.TransactionManager{Paths: paths.Repository, Operations: control.Operations{DB: runtime.DB}, DerivedUpdate: func(ctx context.Context, root, revision string, changed []string) error {
		if len(changed) == 0 {
			return index.Rebuild(ctx, root, revision)
		}
		return index.UpdatePaths(ctx, root, revision, changed)
	}}
	if _, err = ops.recover(ctx, &manager); err != nil {
		return nil, nil, err
	}
	tokens := options.Tokens
	if tokens == nil {
		tokens, err = StaticBearerPrincipals(config.Authorization, nil)
		if err != nil {
			return nil, nil, err
		}
	}
	identity, err := NewIdentity(tokens, config.Authorization, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	metadata, err := ops.metadata(options.Surface)
	if err != nil {
		return nil, nil, err
	}
	controls := &ProposalControls{Queue: ProposalQueue{Proposals: control.Proposals{DB: runtime.DB}, Paths: paths.Repository}, Random: rand.Reader, DerivedIndexPath: paths.DerivedDB, Staging: staging, MaxConceptBytes: 1 << 20, Index: index, DefaultSearchMode: "lexical", Metadata: metadata, DerivedUpdate: manager.DerivedUpdate}
	jobs := &Jobs{Controls: controls, Identity: identity, DBPath: paths.ControlDB}
	server = umcp.NewServer("memento")
	if err = ops.register(jobs, server, options.Surface, options.Limits); err != nil {
		return nil, nil, err
	}
	runtime.Jobs = jobs
	return runtime, server, nil
}
