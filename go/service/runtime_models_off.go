package service

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"sort"
	"strings"

	"database/sql"
	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/graphdebug"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

type ModelsOffRuntimeOptions struct {
	Surface       string
	Limits        execute.Limits
	BootstrapSeed string
	Tokens        []BearerPrincipal
	Graph         GraphHTTPConfig
}
type modelsOffBuildOps struct {
	storage         func(context.Context, RuntimeConfig, string) (*Runtime, error)
	revision        func(repository.GitRepositoryPaths) (string, error)
	state           func(context.Context, *derived.Index) (derived.IndexState, error)
	rebuild         func(context.Context, *derived.Index, string, string) error
	expire          func(context.Context, *assets.StagingStore) (int64, error)
	recover         func(context.Context, *repository.TransactionManager) ([]repository.RecoveryRecord, error)
	metadata        func(string) (*ModelsOffMetadata, error)
	register        func(*Jobs, *umcp.Server, string, execute.Limits) error
	openAccess      func(context.Context, *sql.DB, string) (*access.Store, error)
	bootstrapAccess func(context.Context, *access.Store, []access.ConfiguredPrincipal, map[string]string) error
	lookupEnv       func(string) (string, bool)
	registerAccess  func(*Jobs, *umcp.Server) error
}

func defaultModelsOffBuildOps() modelsOffBuildOps {
	return modelsOffBuildOps{BuildRuntimeStorage, repository.GetMainRevision, func(ctx context.Context, i *derived.Index) (derived.IndexState, error) { return i.State(ctx) }, func(ctx context.Context, i *derived.Index, root, revision string) error {
		return i.Rebuild(ctx, root, revision)
	}, func(ctx context.Context, s *assets.StagingStore) (int64, error) { return s.Expire(ctx) }, func(ctx context.Context, m *repository.TransactionManager) ([]repository.RecoveryRecord, error) {
		return m.RecoverStartup(ctx)
	}, NewModelsOffMetadata, func(j *Jobs, s *umcp.Server, surface string, limits execute.Limits) error {
		return j.RegisterModelsOffServer(s, surface, limits, nil)
	}, access.OpenStore, func(ctx context.Context, s *access.Store, p []access.ConfiguredPrincipal, t map[string]string) error {
		return s.Bootstrap(ctx, p, t)
	}, os.LookupEnv, func(j *Jobs, s *umcp.Server) error { return j.RegisterAccessTools(s) }}
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
	var managed *access.Store
	master, _ := ops.lookupEnv("MEMENTO_ADMIN_MASTER_KEY")
	master = strings.TrimSpace(master)
	if options.Tokens == nil && master != "" {
		if managed, err = ops.openAccess(ctx, runtime.DB, master); err != nil {
			return nil, nil, err
		}
		names := make([]string, 0, len(config.Authorization.Principals))
		for name := range config.Authorization.Principals {
			names = append(names, name)
		}
		sort.Strings(names)
		principals := make([]access.ConfiguredPrincipal, 0, len(names))
		bootstrapTokens := map[string]string{}
		for _, name := range names {
			policy := config.Authorization.Principals[name]
			principals = append(principals, access.ConfiguredPrincipal{Name: name, Policy: policy})
			value, _ := ops.lookupEnv(policy.TokenEnv)
			bootstrapTokens[name] = strings.TrimSpace(value)
		}
		if err = ops.bootstrapAccess(ctx, managed, principals, bootstrapTokens); err != nil {
			return nil, nil, err
		}
		tokens = nil
	} else if tokens == nil {
		tokens, err = StaticBearerPrincipals(config.Authorization, ops.lookupEnv)
		if err != nil {
			return nil, nil, err
		}
	}
	var managedIdentity ManagedIdentity
	var graphManaged GraphManagedPolicies
	if managed != nil {
		managedIdentity = managed
		graphManaged = managed
	}
	identity, err := NewIdentity(tokens, config.Authorization, managedIdentity, nil)
	if err != nil {
		return nil, nil, err
	}
	metadata, err := ops.metadata(options.Surface)
	if err != nil {
		return nil, nil, err
	}
	controls := &ProposalControls{Queue: ProposalQueue{Proposals: control.Proposals{DB: runtime.DB}, Paths: paths.Repository}, Random: rand.Reader, DerivedIndexPath: paths.DerivedDB, Staging: staging, MaxConceptBytes: config.Limits.MaxConceptBytes, Index: index, DefaultSearchMode: "lexical", Metadata: metadata, DerivedUpdate: manager.DerivedUpdate}
	jobs := &Jobs{Controls: controls, Identity: identity, DBPath: paths.ControlDB}
	server = umcp.NewServer("memento")
	if err = ops.register(jobs, server, options.Surface, options.Limits); err != nil {
		return nil, nil, err
	}
	if managed != nil {
		if err = ops.registerAccess(jobs, server); err != nil {
			return nil, nil, err
		}
	}
	runtime.Jobs = jobs
	runtime.HTTPHooks = identity.HTTPHooks()
	stagingHTTP := StagingHTTP{Store: staging, Authenticate: identity.AuthenticateHeaders}
	graphHTTP := GraphHTTP{Config: options.Graph, Snapshots: graphdebug.NewSnapshotService(paths.Repository.CurrentDir, paths.DerivedDB, paths.ControlDB), Policies: &GraphPolicyDirectory{Static: config.Authorization, Managed: graphManaged}}
	runtime.HTTPHooks.Route = func(ctx context.Context, method, path string, headers map[string]string, body []byte, peer string) (*umcp.HTTPResponse, error) {
		response, routeErr := stagingHTTP.Handle(ctx, method, path, headers, body, peer)
		if response != nil || routeErr != nil {
			return response, routeErr
		}
		return graphHTTP.Handle(ctx, method, path, headers, body, peer)
	}
	return runtime, server, nil
}
