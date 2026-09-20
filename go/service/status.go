package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

//go:embed status_defaults.json
var statusDefaults []byte

//go:embed status_tools.json
var statusToolDefinitions []byte

// BuildVersion is set by release builds. An empty value retains the pinned
// source-runtime version used by differential fixtures and development tests.
var BuildVersion string

// ModelsOffMetadata is explicit models-disabled runtime configuration. It does
// not enable semantic/router/dream features or replace full config validation.
type ModelsOffMetadata struct {
	ServiceVersion  string         `json:"service_version"`
	SchemaVersion   int            `json:"schema_version"`
	Limits          map[string]any `json:"limits"`
	NeedleModelPath string         `json:"needle_model_path"`
	Catalog         *Catalog       `json:"-"`
}

func NewModelsOffMetadata(surface string) (*ModelsOffMetadata, error) {
	return modelsOffMetadata(surface, statusDefaults)
}
func modelsOffMetadata(surface string, raw []byte) (*ModelsOffMetadata, error) {
	var meta ModelsOffMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	catalog, err := NewCatalog(CatalogConfig{Surface: surface})
	if err != nil {
		return nil, err
	}
	meta.Catalog = catalog
	if BuildVersion != "" {
		meta.ServiceVersion = BuildVersion
	}
	return &meta, nil
}

type StatusIndex interface {
	Status(context.Context, access.EffectivePolicy) (derived.StatusSnapshot, error)
}
type SemanticStatusIndex interface {
	EmbeddingRevision(context.Context) (string, error)
}

func (c *ProposalControls) Status(ctx context.Context, actor ProposalActor) (map[string]any, SuccessOptions, error) {
	var data map[string]any
	var options SuccessOptions
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		data, options, err = c.status(ctx, actor, defaultProposalRepository())
		return err
	})
	return data, options, err
}
func (c *ProposalControls) status(ctx context.Context, actor ProposalActor, repo proposalRepository) (map[string]any, SuccessOptions, error) {
	return c.statusWithList(ctx, actor, repo, c.Queue.Proposals.List)
}
func (c *ProposalControls) statusWithList(ctx context.Context, actor ProposalActor, repo proposalRepository, list func(context.Context, control.ProposalQuery) ([]control.ProposalRecord, error)) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if c.Metadata == nil {
		return nil, options, errors.New("models-off service metadata is not configured")
	}
	index, ok := c.Index.(StatusIndex)
	if !ok {
		return nil, options, &derived.UnavailableError{Message: "derived index is unavailable"}
	}
	snapshot, err := index.Status(ctx, actor.Policy)
	if err != nil {
		return nil, options, err
	}
	revision, err := repo.main(c.Queue.Paths)
	if err != nil {
		return nil, options, err
	}
	state := snapshot.State
	stale := state.IndexRevision != revision || state.Status != "ready"
	count := snapshot.VisibleConcepts
	if state.Status != "ready" || state.RepoRevision == "" || state.IndexRevision == "" {
		bundle, err := c.readBundle(actor.Policy)
		if err != nil {
			return nil, options, err
		}
		count = len(bundle.Entries)
	}
	if err = c.Queue.refreshAll(ctx, repo); err != nil {
		return nil, options, err
	}
	// Query all records before visibility, matching source malformed-row ordering.
	proposals, err := list(ctx, control.ProposalQuery{})
	if err != nil {
		return nil, options, err
	}
	unresolved := map[control.ProposalStatus]bool{}
	for _, status := range control.UnresolvedProposalStatuses() {
		unresolved[status] = true
	}
	backlog := 0
	for _, proposal := range proposals {
		if !unresolved[proposal.Status] {
			continue
		}
		visible, err := CanAccessProposal(actor.Policy, proposal, false)
		if err != nil {
			return nil, options, err
		}
		if visible {
			backlog++
		}
	}
	limits := copyCatalogObject(c.Metadata.Limits)
	limits["assets"] = assets.RetrievalLimits()
	capabilities := c.RuntimeCapabilities
	var embeddingRevision any = nil
	semanticReady := false
	if capabilities.SemanticEnabled {
		if semanticIndex, ok := c.Index.(SemanticStatusIndex); ok {
			value, stateErr := semanticIndex.EmbeddingRevision(ctx)
			if stateErr != nil {
				return nil, options, stateErr
			}
			if value != "" {
				embeddingRevision = value
			}
			semanticReady = capabilities.SemanticLoaded && value == revision
		}
	}
	var semanticModel any = nil
	var semanticDimensions any = nil
	if capabilities.SemanticEnabled {
		semanticModel = capabilities.SemanticModelID
		semanticDimensions = capabilities.SemanticDimensions
	}
	var needleRuntime any = nil
	if capabilities.NeedleLoaded {
		needleRuntime = capabilities.NeedleRuntime
	}
	needlePath := capabilities.NeedleModelPath
	if needlePath == "" {
		needlePath = c.Metadata.NeedleModelPath
	}
	data := map[string]any{
		"service_version": c.Metadata.ServiceVersion, "schema_version": c.Metadata.SchemaVersion, "repo_revision": revision, "index_revision": state.IndexRevision, "index_stale": stale, "principal": actor.Policy.Principal, "visible_concepts": count, "proposal_backlog": backlog, "limits": limits, "roles": append([]string{}, actor.Policy.Roles...),
		"features":  map[string]any{"resources": true, "streamable_http": true, "proposal_rebase": true, "model_proposals": false, "dream_mode": "disabled", "semantic_search": capabilities.SemanticEnabled, "needle_router": capabilities.NeedleEnabled},
		"readiness": map[string]any{"semantic_search": map[string]any{"ready": semanticReady, "model_id": semanticModel, "dimensions": semanticDimensions, "embedding_revision": embeddingRevision, "sqlite_vector_enabled": false}, "needle_router": map[string]any{"enabled": capabilities.NeedleEnabled, "loaded": capabilities.NeedleLoaded, "runtime": needleRuntime, "model_path": needlePath}},
	}
	options.RepoRevision = &revision
	options.IndexRevision = &state.IndexRevision
	options.IndexStale = stale
	if stale {
		options.Warnings = []string{"derived_index_stale"}
	}
	return data, options, nil
}

// RegisterStatusTools adds real models-off help/status to the 28-tool subset.
// Configured full surfaces still require the remaining operation handlers.
func (j *Jobs) RegisterStatusTools(server *umcp.Server, notify ProposalNotifier) error {
	return j.registerStatusTools(server, notify, statusToolDefinitions)
}
func (j *Jobs) registerStatusTools(server *umcp.Server, notify ProposalNotifier, definitions []byte) error {
	if err := registerProposalTools(server, j.callStatusOrTool, notify, definitions); err != nil {
		return err
	}
	for _, entry := range []struct{ name, title string }{{"help", "Service help"}, {"status", "Service status"}} {
		name := entry.name
		_ = server.Resources.Register(umcp.Resource{URI: "memory://" + name, Name: name, Title: entry.title, MIME: "application/json", Read: func(ctx context.Context, _ map[string]any) (any, error) {
			value, err := j.callStatusOrTool(ctx, "memory_"+name, map[string]any{})
			if err != nil {
				return nil, err
			}
			return catalogResource(value)
		}})
	}
	return nil
}
func (j *Jobs) callStatusOrTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if name != "memory_status" && name != "memory_help" {
		return j.callStagingOrProposalTool(ctx, name, args)
	}
	if name == "memory_help" {
		// _policy is outside the source help exception catch. No worker admission.
		if _, err := j.Identity.Context(ctx); err != nil {
			return nil, err
		}
		if j.Controls.Metadata == nil || j.Controls.Metadata.Catalog == nil {
			return nil, errors.New("help catalog is not configured")
		}
		return j.Controls.Queue.SuccessEnvelope(j.Controls.Metadata.Catalog.Help(), SuccessOptions{})
	}
	// memory_status is a direct wrapper, but policy/snapshot/revision/refresh and
	// envelope creation share the serialized service scope. Busy workers do not
	// reject it; an active writer can still make it wait.
	var result any
	err := repository.WithTransactionLock(ctx, j.Controls.Queue.Paths, func() error {
		actor, err := j.Identity.Context(ctx)
		if err != nil {
			return err
		}
		data, options, err := j.Controls.status(ctx, actor, defaultProposalRepository())
		if err != nil {
			return err
		}
		result, err = j.Controls.Queue.SuccessEnvelope(data, options)
		return err
	})
	if err != nil {
		return FailureEnvelope(err)
	}
	return result, nil
}
