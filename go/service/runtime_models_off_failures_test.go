package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildModelsOffComponentFailures(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	for _, stage := range []string{"revision", "state-rebuild", "rebuild", "expire", "recover", "metadata", "register", "access-open", "access-bootstrap", "access-register", "needle-model", "needle-tokenizer", "needle-router", "needle-build", "needle-register", "semantic-load"} {
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
				ops.loadNeedleModel = func(string) (*needle.Model, error) { return nil, boom }
			case "needle-tokenizer":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				ops.loadNeedleModel = func(string) (*needle.Model, error) { return &needle.Model{}, nil }
				ops.loadNeedleTokenizer = func(string) (*needle.Tokenizer, error) { return nil, boom }
			case "semantic-load":
				model := "/model"
				options.Semantic = DefaultSemanticSearchConfig()
				options.Semantic.Enabled = true
				options.Semantic.ModelPath = &model
				ops.loadSemantic = func(string, string, int, int, int) (*GTESemanticClient, error) { return nil, boom }
			case "needle-register":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				ops.buildRoute = func(NeedleRouterConfig) (RouteInference, *needle.Tokenizer, error) {
					return &routeInferenceStub{}, &needle.Tokenizer{}, nil
				}
				ops.registerRoute = func(*Jobs, *umcp.Server, string, execute.Limits, RouteInference, *needle.Tokenizer) error {
					return boom
				}
			case "needle-build":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				ops.buildRoute = func(NeedleRouterConfig) (RouteInference, *needle.Tokenizer, error) { return nil, nil, boom }
			case "needle-router":
				options.Needle = DefaultNeedleRouterConfig()
				options.Needle.Enabled = true
				ops.newNeedleRouter = func(*needle.Model) (*needle.Router, error) { return nil, boom }
				ops.loadNeedleTokenizer = func(string) (*needle.Tokenizer, error) { return &needle.Tokenizer{}, nil }
				ops.loadNeedleModel = func(string) (*needle.Model, error) { return &needle.Model{}, nil }
			case "access-register":
				ops.lookupEnv = func(name string) (string, bool) { return "master", true }
				ops.registerAccess = func(*Jobs, *umcp.Server) error { return boom }
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
