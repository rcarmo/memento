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
	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/execute"
	"github.com/rcarmo/memento/internal/graphdebug"
	"github.com/rcarmo/memento/internal/needle"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
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
	DeepAnswers    DeepAnswersConfig
	ExactCache     ExactAnswerCacheConfig
	HotMemory      HotWorkingMemoryConfig
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
	migrateLegacySkills  func(string) ([]string, error)
	stat                 func(string) (os.FileInfo, error)
	materialize          func(context.Context, repository.GitRepositoryPaths, string) (repository.MaterializedCheckout, error)
	metadata             func(string) (*ModelsOffMetadata, error)
	register             func(*Jobs, *umcp.Server, string, execute.Limits) error
	openAccess           func(context.Context, *sql.DB, string) (*access.Store, error)
	bootstrapAccess      func(context.Context, *access.Store, []access.ConfiguredPrincipal, map[string]string) error
	lookupEnv            func(string) (string, bool)
	registerAccess       func(*Jobs, *umcp.Server) error
	loadNeedleModel      func(string) (*needle.Model, error)
	loadNeedleTokenizer  func(string) (*needle.Tokenizer, error)
	newNeedleRouter      func(*needle.Model) (*needle.Router, error)
	buildRoute           func(NeedleRouterConfig) (NeedleRouteInference, error)
	registerRoute        func(*Jobs, *umcp.Server, string, execute.Limits, NeedleRouteInference) error
	registerConfigured   func(context.Context, *Runtime, *Jobs, *umcp.Server, ModelsOffRuntimeOptions, NeedleRouteInference) error
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
	}, repository.RepositoryNeedsLegacyBlobMigration, repository.MigrateLegacyBlobsToGit, MigrateLegacySkillPacks, os.Stat, repository.MaterializeCurrentCheckout, NewModelsOffMetadata, func(j *Jobs, s *umcp.Server, surface string, limits execute.Limits) error {
		return j.RegisterModelsOffServer(s, surface, limits, nil)
	}, access.OpenStore, func(ctx context.Context, s *access.Store, p []access.ConfiguredPrincipal, t map[string]string) error {
		return s.Bootstrap(ctx, p, t)
	}, os.LookupEnv, func(j *Jobs, s *umcp.Server) error { return j.RegisterAccessTools(s) }, needle.Load, needle.LoadTokenizer, needle.NewRouter, nil, func(j *Jobs, s *umcp.Server, surface string, limits execute.Limits, inference NeedleRouteInference) error {
		return j.registerModelsOffServer(s, surface, limits, nil, inference)
	}, func(ctx context.Context, runtime *Runtime, jobs *Jobs, server *umcp.Server, options ModelsOffRuntimeOptions, inference NeedleRouteInference) error {
		answers := AnswerStore{DB: runtime.DB}
		if err := answers.Migrate(ctx); err != nil {
			return err
		}
		catalogConfig := CatalogConfig{Surface: options.Surface, AnswerEnabled: options.DeepAnswers.Enabled && options.ModelClient != nil, RouteEnabled: options.Needle.Enabled, ProposalEnabled: options.ModelProposals.Enabled && options.ModelClient != nil}
		catalog, err := NewCatalog(catalogConfig)
		if err != nil {
			return err
		}
		executeEndpoint, err := NewExecuteEndpoint(jobs, catalog, options.Limits)
		if err != nil {
			return err
		}
		answer := &AnswerEndpoint{Jobs: jobs, Client: options.ModelClient, Store: answers, Deep: options.DeepAnswers, Cache: options.ExactCache, Hot: options.HotMemory}
		proposals := &ModelProposalEndpoint{Jobs: jobs, Client: options.ModelClient, Config: options.ModelProposals, Timeout: time.Duration(options.DeepAnswers.Limits.MaxTimeSeconds * float64(time.Second))}
		route := RouteEndpoint{Jobs: jobs, Router: inference, Execute: executeEndpoint}
		return jobs.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: catalogConfig, Limits: options.Limits, ModelHandlers: map[string]CatalogHandler{"memory_answer": answer.Call, "memory_route": route.Call, "memory_propose_freeform": proposals.Freeform, "memory_propose_update": proposals.Update}})
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
	var revision string
	index := &derived.Index{Path: paths.DerivedDB, DeferEmbeddings: options.Semantic.Enabled, MaxInputChars: options.Semantic.MaxInputChars}
	runtime.Metrics = NewRuntimeMetrics()
	manager := repository.TransactionManager{Paths: paths.Repository, Operations: control.Operations{DB: runtime.DB}, DerivedUpdate: func(ctx context.Context, root, revision string, changed []string) error {
		started := time.Now()
		operation := "update"
		var updateErr error
		if len(changed) == 0 {
			operation = "rebuild"
			updateErr = index.Rebuild(ctx, root, revision)
		} else {
			updateErr = index.UpdatePaths(ctx, root, revision, changed)
		}
		runtime.Metrics.ObserveIndex(operation, time.Since(started), updateErr)
		return updateErr
	}}
	recoveryManager := repository.TransactionManager{Paths: paths.Repository, Operations: control.Operations{DB: runtime.DB}}
	if _, err = ops.recover(ctx, &recoveryManager); err != nil {
		return nil, nil, err
	}
	needsMigration, err := ops.needsLegacyMigration(paths.Repository.CurrentDir)
	if err != nil {
		return nil, nil, err
	}
	migrationManager := repository.TransactionManager{Paths: paths.Repository, Operations: control.Operations{DB: runtime.DB}}
	if needsMigration {
		revision, err = ops.revision(paths.Repository)
		if err != nil {
			return nil, nil, err
		}
		request := repository.TransactionRequest{Operation: control.OperationRequest{OpID: "migrate-legacy-blobs-to-git-v1", Principal: "memento-migration", IdempotencyKey: "migrate-legacy-blobs-to-git-v1", ToolName: "internal_legacy_blob_migration", RequestJSON: `{"base_revision":"` + revision + `"}`}, ExpectedRevision: revision, CommitMessage: "memory: migrate legacy assets to ordinary Git blobs", AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}
		if _, err = migrationManager.Apply(ctx, request, func(_ context.Context, worktree string) ([]string, error) {
			return ops.migrateLegacy(worktree, filepath.Join(paths.Repository.BareDir, "lfs", "objects"))
		}); err != nil {
			return nil, nil, err
		}
	}
	legacySkills := filepath.Join(paths.Repository.CurrentDir, "skills", ".versions")
	if info, statErr := ops.stat(legacySkills); statErr == nil && info.IsDir() {
		revision, err = ops.revision(paths.Repository)
		if err != nil {
			return nil, nil, err
		}
		request := repository.TransactionRequest{Operation: control.OperationRequest{OpID: "migrate-generic-asset-packs-v1", Principal: "memento-migration", IdempotencyKey: "migrate-generic-asset-packs-v1", ToolName: "internal_asset_migration", RequestJSON: `{"base_revision":"` + revision + `"}`}, ExpectedRevision: revision, CommitMessage: "memory: migrate skills to generic assets", AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}
		if _, err = migrationManager.Apply(ctx, request, func(_ context.Context, worktree string) ([]string, error) { return ops.migrateLegacySkills(worktree) }); err != nil {
			return nil, nil, err
		}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, nil, statErr
	}
	revision, err = ops.revision(paths.Repository)
	if err != nil {
		return nil, nil, err
	}
	if _, err = ops.materialize(ctx, paths.Repository, revision); err != nil {
		return nil, nil, err
	}
	state, stateErr := ops.state(ctx, index)
	if stateErr != nil || state.IndexRevision != revision || state.LinkResolutionVersion != derived.LinkResolutionVersion {
		started := time.Now()
		err = ops.rebuild(ctx, index, paths.Repository.CurrentDir, revision)
		runtime.Metrics.ObserveIndex("rebuild", time.Since(started), err)
		if err != nil {
			return nil, nil, err
		}
	}
	staging := &assets.StagingStore{DB: runtime.DB, Random: rand.Reader}
	if _, err = ops.expire(ctx, staging); err != nil {
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
	activity := NewActivityClock(nil)
	identity, err := NewIdentity(tokens, config.Authorization, managedIdentity, activity.Touch)
	if err != nil {
		return nil, nil, err
	}
	metadata, err := ops.metadata(options.Surface)
	if err != nil {
		return nil, nil, err
	}
	runtime.ServiceVersion = metadata.ServiceVersion
	if options.Needle.Enabled {
		metadata.Catalog, _ = NewCatalog(CatalogConfig{Surface: options.Surface, RouteEnabled: true})
	}
	needlePath, needleRuntime := options.Needle.ModelPath, "go-scalar"
	if options.Needle.WorkerMode == "subprocess" {
		needlePath, needleRuntime = options.Needle.FP32ModelPath, "go-mmap-subprocess"
	}
	capabilities := RuntimeCapabilities{SemanticEnabled: options.Semantic.Enabled, SemanticModelID: options.Semantic.ModelID, SemanticDimensions: options.Semantic.Dimensions, NeedleEnabled: options.Needle.Enabled, NeedleModelPath: needlePath, NeedleRuntime: needleRuntime}
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
		} else if options.Semantic.WorkerMode == "subprocess" {
			client, loadErr = LoadSubprocessSemanticClient(options.Semantic)
		} else {
			client, loadErr = ops.loadSemantic(*options.Semantic.ModelPath, options.Semantic.ModelID, options.Semantic.Dimensions, options.Semantic.MaxBatchSize, options.Semantic.MaxInputChars)
		}
		if loadErr != nil {
			return nil, nil, loadErr
		}
		_, index.ChunkEmbeddings = client.(derived.SemanticChunkClient)
		index.ConfigureChunkModel(client.ModelInfo())
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
			if index.ChunkEmbeddings || semanticRefreshNeeded(state.RepoRevision, embeddingRevision) {
				options.SemanticWorker.Enqueue(paths.Repository.CurrentDir, state.RepoRevision, nil, true)
			}
		}
	}
	server = umcp.NewServer("memento")
	server.Version = metadata.ServiceVersion
	configured := options.DeepAnswers.Enabled || options.ModelProposals.Enabled
	if !configured {
		if err = ops.register(jobs, server, options.Surface, options.Limits); err != nil {
			return nil, nil, err
		}
	}
	var routeInference NeedleRouteInference
	if options.Needle.Enabled {
		resolved := options.Needle.Resolved(ops.lookupEnv)
		var loadErr error
		if ops.buildRoute != nil {
			routeInference, loadErr = ops.buildRoute(resolved)
		} else if resolved.WorkerMode == "subprocess" {
			routeInference, loadErr = LoadSubprocessNeedleClient(resolved)
		} else {
			model, modelErr := ops.loadNeedleModel(resolved.ModelPath)
			loadErr = modelErr
			if loadErr == nil {
				var tokenizer *needle.Tokenizer
				tokenizer, loadErr = ops.loadNeedleTokenizer(resolved.TokenizerPath)
				if loadErr == nil {
					var router *needle.Router
					router, loadErr = ops.newNeedleRouter(model)
					if loadErr == nil {
						if value, _ := ops.lookupEnv("MEMENTO_SIMD"); strings.TrimSpace(value) != "" {
							loadErr = router.SetSIMD(value)
						}
						if loadErr == nil {
							routeInference = inProcessNeedleClient{Router: router, Tokenizer: tokenizer}
						}
					}
				}
			}
		}
		if loadErr != nil {
			return nil, nil, loadErr
		}
		controls.RuntimeCapabilities.NeedleLoaded = true
		if !configured {
			if err = ops.registerRoute(jobs, server, options.Surface, options.Limits, routeInference); err != nil {
				return nil, nil, err
			}
		}
	}
	if configured {
		if err = ops.registerConfigured(ctx, runtime, jobs, server, options, routeInference); err != nil {
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
	metricsHTTP := LiveMetricsHTTP{Runtime: runtime}
	adminHTTP := AdminHTTP{ProtectedReadPrefixes: config.Authorization.ProtectedReadPrefixes}
	if managed != nil {
		adminHTTP.Store = managed
	}
	runtime.HTTPHooks.Route = func(ctx context.Context, method, path string, headers map[string]string, body []byte, peer string) (*umcp.HTTPResponse, error) {
		response, routeErr := stagingHTTP.Handle(ctx, method, path, headers, body, peer)
		if response != nil || routeErr != nil {
			return response, routeErr
		}
		response, routeErr = adminHTTP.Handle(ctx, method, path, headers, body, peer)
		if response != nil || routeErr != nil {
			return response, routeErr
		}
		response, routeErr = metricsHTTP.Handle(ctx, method, path, headers, body, peer)
		if response != nil || routeErr != nil {
			return response, routeErr
		}
		return graphHTTP.Handle(ctx, method, path, headers, body, peer)
	}
	return runtime, server, nil
}
