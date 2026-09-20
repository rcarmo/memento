package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

func TestBuildModelsOffComponentFailures(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	for _, stage := range []string{"revision", "state-rebuild", "rebuild", "expire", "recover", "legacy-detect", "legacy-revision", "legacy-migrate", "legacy-skill-revision", "legacy-skill-migrate", "legacy-stat", "legacy-materialize", "legacy-final-revision", "metadata", "register", "access-open", "access-bootstrap", "access-register", "needle-model", "needle-tokenizer", "needle-router", "needle-build", "needle-register", "semantic-load", "semantic-subprocess", "configured-register"} {
		t.Run(stage, func(t *testing.T) {
			var config RuntimeConfig
			config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
			options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}}
			if strings.HasPrefix(stage, "access-") {
				options.Tokens = nil
			}
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
			case "legacy-detect":
				ops.needsLegacyMigration = func(string) (bool, error) { return false, boom }
			case "legacy-revision":
				ops.needsLegacyMigration = func(string) (bool, error) { return true, nil }
				ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
			case "legacy-migrate":
				ops.needsLegacyMigration = func(string) (bool, error) { return true, nil }
				ops.migrateLegacy = func(string, string) ([]string, error) { return nil, boom }
			case "legacy-skill-revision":
				ops.needsLegacyMigration = func(string) (bool, error) { return false, nil }
				ops.migrateLegacySkills = func(string) ([]string, error) { return nil, nil }
				ops.stat = func(string) (os.FileInfo, error) { return os.Stat(config.Repository.RootPath) }
				ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
			case "legacy-skill-migrate":
				ops.needsLegacyMigration = func(string) (bool, error) { return false, nil }
				ops.stat = func(string) (os.FileInfo, error) { return os.Stat(config.Repository.RootPath) }
				ops.migrateLegacySkills = func(string) ([]string, error) { return nil, boom }
			case "legacy-stat":
				ops.needsLegacyMigration = func(string) (bool, error) { return false, nil }
				ops.stat = func(string) (os.FileInfo, error) { return nil, boom }
			case "legacy-materialize":
				ops.materialize = func(context.Context, repository.GitRepositoryPaths, string) (repository.MaterializedCheckout, error) {
					return repository.MaterializedCheckout{}, boom
				}
			case "legacy-final-revision":
				ops.needsLegacyMigration = func(string) (bool, error) { return false, nil }
				ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
			case "metadata":
				ops.metadata = func(string) (*ModelsOffMetadata, error) { return nil, boom }
			case "register":
				ops.register = func(*Jobs, *umcp.Server, string, execute.Limits) error { return boom }
			case "access-open":
				ops.lookupEnv = func(name string) (string, bool) { return "master", true }
				ops.openAccess = func(context.Context, *sql.DB, string) (*access.Store, error) { return nil, boom }
			case "access-bootstrap":
				ops.lookupEnv = func(name string) (string, bool) { return "master", true }
				ops.bootstrapAccess = func(context.Context, *access.Store, []access.ConfiguredPrincipal, map[string]string) error {
					return boom
				}
			case "needle-model":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				options.Needle.WorkerMode = "in_process"
				ops.loadNeedleModel = func(string) (*needle.Model, error) { return nil, boom }
			case "needle-tokenizer":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				options.Needle.WorkerMode = "in_process"
				ops.loadNeedleModel = func(string) (*needle.Model, error) { return &needle.Model{}, nil }
				ops.loadNeedleTokenizer = func(string) (*needle.Tokenizer, error) { return nil, boom }
			case "semantic-load":
				model := "/model"
				options.Semantic = DefaultSemanticSearchConfig()
				options.Semantic.Enabled = true
				options.Semantic.WorkerMode = "in_process"
				options.Semantic.ModelPath = &model
				ops.loadSemantic = func(string, string, int, int, int) (*GTESemanticClient, error) { return nil, boom }
			case "semantic-subprocess":
				model := filepath.Join(t.TempDir(), "missing")
				options.Semantic = DefaultSemanticSearchConfig()
				options.Semantic.Enabled = true
				options.Semantic.ModelPath = &model
			case "needle-register":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				ops.buildRoute = func(NeedleRouterConfig) (NeedleRouteInference, error) { return &routeInferenceStub{}, nil }
				ops.registerRoute = func(*Jobs, *umcp.Server, string, execute.Limits, NeedleRouteInference) error { return boom }
			case "needle-build":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				ops.buildRoute = func(NeedleRouterConfig) (NeedleRouteInference, error) { return nil, boom }
			case "needle-router":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				options.Needle.WorkerMode = "in_process"
				ops.newNeedleRouter = func(*needle.Model) (*needle.Router, error) { return nil, boom }
				ops.loadNeedleTokenizer = func(string) (*needle.Tokenizer, error) { return &needle.Tokenizer{}, nil }
				ops.loadNeedleModel = func(string) (*needle.Model, error) { return &needle.Model{}, nil }
			case "configured-register":
				options.DeepAnswers = DefaultDeepAnswersConfig()
				options.DeepAnswers.Enabled = true
				options.ModelClient = &stubModelClient{}
				ops.registerConfigured = func(context.Context, *Runtime, *Jobs, *umcp.Server, ModelsOffRuntimeOptions, NeedleRouteInference) error {
					return boom
				}
			case "access-register":
				ops.lookupEnv = func(name string) (string, bool) { return "master", true }
				ops.registerAccess = func(*Jobs, *umcp.Server) error { return boom }
			}
			runtime, server, err := buildModelsOffRuntime(ctx, config, options, ops)
			if stage == "semantic-subprocess" {
				if err == nil || runtime != nil || server != nil {
					t.Fatal(runtime, server, err)
				}
			} else if !errors.Is(err, boom) || runtime != nil || server != nil {
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
