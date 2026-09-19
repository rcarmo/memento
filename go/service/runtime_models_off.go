package service

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"database/sql"
	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/graphdebug"
	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

type ModelsOffRuntimeOptions struct {
	Surface        string
	Limits         execute.Limits
	BootstrapSeed  string
	Tokens         []BearerPrincipal
	Graph          GraphHTTPConfig
	Needle         NeedleRouterConfig
	SemanticWorker *derived.SemanticWorker
	Semantic       SemanticSearchConfig
	Dream          DreamConfig
	ModelClient    ModelClient
	ModelProposals ModelProposalsConfig
}
type modelsOffBuildOps struct {
	storage              func(context.Context, RuntimeConfig, string) (*Runtime, error)
	revision             func(repository.GitRepositoryPaths) (string, error)
	state                func(context.Context, *derived.Index) (derived.IndexState, error)
	rebuild              func(context.Context, *derived.Index, string, string) error
	expire               func(context.Context, *assets.StagingStore) (int64, error)
	recover              func(context.Context, *repository.TransactionManager) ([]repository.RecoveryRecord, error)
	needsLegacyMigration func(string) (bool, error)
	migrateLegacy        func(string, string) ([]string, error)
	metadata             func(string) (*ModelsOffMetadata, error)
	register             func(*Jobs, *umcp.Server, string, execute.Limits) error
	openAccess           func(context.Context, *sql.DB, string) (*access.Store, error)
	bootstrapAccess      func(context.Context, *access.Store, []access.ConfiguredPrincipal, map[string]string) error
	lookupEnv            func(string) (string, bool)
	registerAccess       func(*Jobs, *umcp.Server) error
	loadNeedleModel      func(string) (*needle.Model, error)
	loadNeedleTokenizer  func(string) (*needle.Tokenizer, error)
	newNeedleRouter      func(*needle.Model) (*needle.Router, error)
	buildRoute           func(NeedleRouterConfig) (RouteInference, *needle.Tokenizer, error)
	registerRoute        func(*Jobs, *umcp.Server, string, execute.Limits, RouteInference, *needle.Tokenizer) error
	loadSemantic         func(string, string, int, int, int) (*GTESemanticClient, error)
	buildSemantic        func(SemanticSearchConfig) (derived.SemanticClient, error)
	newSemanticWorker    func(derived.SemanticRefreshIndex, derived.SemanticClient, derived.SemanticRefreshConfig) *derived.SemanticWorker
	newProgressiveWorker func(derived.SemanticRefreshIndex, derived.SemanticClient, derived.SemanticRefreshConfig, derived.SemanticWorkerPolicy, func() time.Duration, func() *float64, func() time.Time) *derived.SemanticWorker
}

func defaultModelsOffBuildOps() modelsOffBuildOps {
	return modelsOffBuildOps{BuildRuntimeStorage, repository.GetMainRevision, func(ctx context.Context, i *derived.Index) (derived.IndexState, error) { return i.State(ctx) }, func(ctx context.Context, i *derived.Index, root, revision string) error {
		return i.Rebuild(ctx, root, revision)
	}, func(ctx context.Context, s *assets.StagingStore) (int64, error) { return s.Expire(ctx) }, func(ctx context.Context, m *repository.TransactionManager) ([]repository.RecoveryRecord, error) {
		return m.RecoverStartup(ctx)
	}, repository.RepositoryNeedsLegacyBlobMigration, repository.MigrateLegacyBlobsToGit, NewModelsOffMetadata, func(j *Jobs, s *umcp.Server, surface string, limits execute.Limits) error {
		return j.RegisterModelsOffServer(s, surface, limits, nil)
	}, access.OpenStore, func(ctx context.Context, s *access.Store, p []access.ConfiguredPrincipal, t map[string]string) error {
		return s.Bootstrap(ctx, p, t)
	}, os.LookupEnv, func(j *Jobs, s *umcp.Server) error { return j.RegisterAccessTools(s) }, needle.Load, needle.LoadTokenizer, needle.NewRouter, nil, func(j *Jobs, s *umcp.Server, surface string, limits execute.Limits, inference RouteInference, tokenizer *needle.Tokenizer) error {
		catalog, _ := NewCatalog(CatalogConfig{Surface: surface, RouteEnabled: true})
		endpoint, _ := NewExecuteEndpoint(j, catalog, limits)
		return (RouteEndpoint{Jobs: j, Router: inference, Tokenizer: tokenizer, Execute: endpoint}).Register(s)
	}, LoadGTESemanticClient, nil, derived.NewSemanticWorker, derived.NewProgressiveSemanticWorker}
}
func semanticRefreshNeeded(repo, embedding string) bool { return repo != embedding }
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
	index := &derived.Index{Path: paths.DerivedDB, DeferEmbeddings: options.Semantic.Enabled, MaxInputChars: options.Semantic.MaxInputChars}
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
	needsMigration, err := ops.needsLegacyMigration(paths.Repository.CurrentDir)
	if err != nil {
		return nil, nil, err
	}
	if needsMigration {
		revision, err = ops.revision(paths.Repository)
		if err != nil {
			return nil, nil, err
		}
		request := repository.TransactionRequest{Operation: control.OperationRequest{OpID: "migrate-legacy-blobs-to-git-v1", Principal: "memento-migration", IdempotencyKey: "migrate-legacy-blobs-to-git-v1", ToolName: "internal_legacy_blob_migration", RequestJSON: `{"base_revision":"` + revision + `"}`}, ExpectedRevision: revision, CommitMessage: "memory: migrate legacy assets to ordinary Git blobs", AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}
		if _, err = manager.Apply(ctx, request, func(_ context.Context, worktree string) ([]string, error) {
			return ops.migrateLegacy(worktree, filepath.Join(paths.Repository.BareDir, "lfs", "objects"))
		}); err != nil {
			return nil, nil, err
		}
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
	activity := NewActivityClock(nil)
	identity, err := NewIdentity(tokens, config.Authorization, managedIdentity, activity.Touch)
	if err != nil {
		return nil, nil, err
	}
	metadata, err := ops.metadata(options.Surface)
	if err != nil {
		return nil, nil, err
	}
	if options.Needle.Enabled {
		metadata.Catalog, _ = NewCatalog(CatalogConfig{Surface: options.Surface, RouteEnabled: true})
	}
	capabilities := RuntimeCapabilities{SemanticEnabled: options.Semantic.Enabled, SemanticModelID: options.Semantic.ModelID, SemanticDimensions: options.Semantic.Dimensions, NeedleEnabled: options.Needle.Enabled, NeedleModelPath: options.Needle.ModelPath}
	controls := &ProposalControls{Queue: ProposalQueue{Proposals: control.Proposals{DB: runtime.DB}, Paths: paths.Repository}, Random: rand.Reader, DerivedIndexPath: paths.DerivedDB, Staging: staging, MaxConceptBytes: config.Limits.MaxConceptBytes, Index: index, DefaultSearchMode: "lexical", Metadata: metadata, RuntimeCapabilities: capabilities, DerivedUpdate: manager.DerivedUpdate}
	jobs := &Jobs{Controls: controls, Identity: identity, DBPath: paths.ControlDB}
	if options.Semantic.Enabled && options.SemanticWorker == nil {
		if options.Semantic.ModelPath == nil {
			return nil, nil, errors.New("semantic model path is required")
		}
		var client derived.SemanticClient
		var loadErr error
		if ops.buildSemantic != nil {
			client, loadErr = ops.buildSemantic(options.Semantic)
		} else {
			client, loadErr = ops.loadSemantic(*options.Semantic.ModelPath, options.Semantic.ModelID, options.Semantic.Dimensions, options.Semantic.MaxBatchSize, options.Semantic.MaxInputChars)
		}
		if loadErr != nil {
			return nil, nil, loadErr
		}
		controls.SemanticClient = client
		controls.RuntimeCapabilities.SemanticLoaded = true
		controls.SemanticMaxCandidates = options.Semantic.MaxCandidates
		controls.DefaultSearchMode = options.Semantic.DefaultSearchMode
		refreshConfig := derived.SemanticRefreshConfig{ModelID: options.Semantic.ModelID, Dimensions: options.Semantic.Dimensions, MaxInputChars: options.Semantic.MaxInputChars, MaxBatch: options.Semantic.MaxBatchSize}
		if options.Semantic.ProgressiveEnabled {
			sampler := NewCPUSampler()
			policy := derived.SemanticWorkerPolicy{Enabled: true, StartupDelay: time.Duration(options.Semantic.ProgressiveStartupDelaySeconds * float64(time.Second)), InteractiveIdle: time.Duration(options.Semantic.ProgressiveInteractiveIdleSeconds * float64(time.Second)), Delay: time.Duration(options.Semantic.ProgressiveDelaySeconds * float64(time.Second)), CPUBusyLimit: options.Semantic.ProgressiveCPUBusyLimitPercent}
			options.SemanticWorker = ops.newProgressiveWorker(index, client, refreshConfig, policy, activity.Idle, sampler.Sample, time.Now)
		} else {
			options.SemanticWorker = ops.newSemanticWorker(index, client, refreshConfig)
		}
		if options.Semantic.RefreshOnStartup {
			state, _ := index.State(ctx)
			embeddingRevision, _ := index.EmbeddingRevision(ctx)
			if semanticRefreshNeeded(state.RepoRevision, embeddingRevision) {
				options.SemanticWorker.Enqueue(paths.Repository.CurrentDir, state.RepoRevision, nil, true)
			}
		}
	}
	server = umcp.NewServer("memento")
	if err = ops.register(jobs, server, options.Surface, options.Limits); err != nil {
		return nil, nil, err
	}
	if options.Needle.Enabled {
		resolved := options.Needle.Resolved(ops.lookupEnv)
		var routeInference RouteInference
		var tokenizer *needle.Tokenizer
		var loadErr error
		if ops.buildRoute != nil {
			routeInference, tokenizer, loadErr = ops.buildRoute(resolved)
			if loadErr != nil {
				return nil, nil, loadErr
			}
		} else {
			model, modelErr := ops.loadNeedleModel(resolved.ModelPath)
			loadErr = modelErr
			if loadErr != nil {
				return nil, nil, loadErr
			}
			tokenizer, loadErr = ops.loadNeedleTokenizer(resolved.TokenizerPath)
			if loadErr != nil {
				return nil, nil, loadErr
			}
			router, loadErr := ops.newNeedleRouter(model)
			if loadErr != nil {
				return nil, nil, loadErr
			}
			routeInference = router
		}
		controls.RuntimeCapabilities.NeedleLoaded = true
		if err = ops.registerRoute(jobs, server, options.Surface, options.Limits, routeInference, tokenizer); err != nil {
			return nil, nil, err
		}
	}
	if managed != nil {
		if err = ops.registerAccess(jobs, server); err != nil {
			return nil, nil, err
		}
	}
	runtime.Jobs = jobs
	runtime.Dream = options.Dream
	runtime.ModelClient = options.ModelClient
	runtime.ModelProposals = options.ModelProposals
	runtime.ProtectedReadPrefixes = append([]string{}, config.Authorization.ProtectedReadPrefixes...)
	if managed != nil {
		runtime.AuditPrincipals = func(ctx context.Context) ([]AuditPrincipal, error) {
			items, e := managed.List(ctx)
			if e != nil {
				return nil, e
			}
			result := make([]AuditPrincipal, 0, len(items))
			for _, item := range items {
				result = append(result, AuditPrincipal{item.Name, item.Roles, item.ReadPrefixes})
			}
			return result, nil
		}
	} else {
		runtime.AuditPrincipals = StaticAuditPrincipals(config.Authorization)
	}
	runtime.HTTPHooks = identity.HTTPHooks()
	stagingHTTP := StagingHTTP{Store: staging, Authenticate: identity.AuthenticateHeaders}
	snapshotService := graphdebug.NewSnapshotService(paths.Repository.CurrentDir, paths.DerivedDB, paths.ControlDB)
	var coordinator *graphdebug.RefreshCoordinator
	if options.SemanticWorker != nil {
		runtime.SemanticWorker = options.SemanticWorker
		runtime.Closers = append(runtime.Closers, func() error { options.SemanticWorker.Close(); return nil })
		coordinator = &graphdebug.RefreshCoordinator{Service: snapshotService, Worker: SemanticRefreshAdapter{options.SemanticWorker}, RepositoryRoot: paths.Repository.CurrentDir, RefreshMaxPaths: options.Graph.Cluster.RefreshMaxPaths, DirectNodeLimit: options.Graph.Overview.DirectNodeLimit, EdgeLimit: options.Graph.Overview.EdgeLimit}
		runtime.GraphRefresh = coordinator
	}
	graphHTTP := GraphHTTP{Config: options.Graph, Snapshots: snapshotService, Refresh: coordinator, Policies: &GraphPolicyDirectory{Static: config.Authorization, Managed: graphManaged}}
	runtime.HTTPHooks.Route = func(ctx context.Context, method, path string, headers map[string]string, body []byte, peer string) (*umcp.HTTPResponse, error) {
		response, routeErr := stagingHTTP.Handle(ctx, method, path, headers, body, peer)
		if response != nil || routeErr != nil {
			return response, routeErr
		}
		return graphHTTP.Handle(ctx, method, path, headers, body, peer)
	}
	return runtime, server, nil
}
