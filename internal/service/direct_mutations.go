package service

import (
	"context"
	_ "embed"
	"errors"
	"io/fs"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

//go:embed mutation_tools.json
var mutationToolDefinitions []byte

// RegisterMutationTools exposes the 20 implemented development tools, not a
// configured compact/standard/admin surface.
func (j *Jobs) RegisterMutationTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callStagingOrProposalTool, notify, mutationToolDefinitions)
}

// CommitConceptChange is the direct curator create/patch/rename entrypoint.
// Model construction precedes service envelope handling in Python; callers must
// propagate normalisation failures to uMCP rather than use FailureEnvelope.
func (c *ProposalControls) CommitConceptChange(ctx context.Context, actor ProposalActor, raw map[string]any, expected, key string) (map[string]any, SuccessOptions, error) {
	manager := repository.TransactionManager{Paths: c.Queue.Paths, Operations: control.Operations{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}, Now: c.Queue.Now, DerivedUpdate: c.DerivedUpdate}
	var data map[string]any
	var options SuccessOptions
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		change, err := directChange(raw)
		if err != nil {
			return err
		}
		data, options, err = c.commitConceptChange(ctx, actor, change, expected, key, defaultProposalRepository(), manager.ApplyUnderLock, defaultMutationIO())
		return err
	})
	return data, options, err
}
func directChange(raw map[string]any) (ProposalChange, error) {
	kind, _ := raw["kind"].(string)
	if kind != "create" && kind != "patch" && kind != "rename" {
		return nil, umcp.ArgumentError("unsupported direct mutation")
	}
	// memory_patch constructs ConceptStatus before the Pydantic change model.
	if kind == "patch" {
		if status, ok := raw["status"].(string); ok && status != "active" && status != "deprecated" && status != "tombstone" {
			return nil, umcp.ArgumentError(pythonRepr(status) + " is not a valid ConceptStatus")
		}
	}
	changes, err := NormalizeProposalChanges([]any{raw})
	if err != nil {
		return nil, umcp.ArgumentError(err.Error())
	}
	return changes[0], nil
}

// commitConceptChange requires the repository lock and a normalised change.
func (c *ProposalControls) commitConceptChange(ctx context.Context, actor ProposalActor, change ProposalChange, expected, key string, repo proposalRepository, transaction proposalTransaction, ops mutationIO) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if err := access.RequireRole(actor.Policy, "curator"); err != nil {
		return nil, options, err
	}
	changes := []ProposalChange{change}
	if err := ValidateChangeAuthorization(actor.Policy, changes, "write"); err != nil {
		return nil, options, err
	}
	warnings, err := c.directMutationWarnings(changes, ops)
	if err != nil {
		return nil, options, err
	}
	request, err := pyjson.Dumps(map[string]any{"changes": []any{map[string]any(change)}})
	if err != nil {
		return nil, options, err
	}
	operationID, err := c.newOperationID()
	if err != nil {
		return nil, options, err
	}
	kind, path := change["kind"].(string), change["path"].(string)
	message := "memory: " + kind + " " + path
	if kind == "rename" {
		message += " -> " + change["new_path"].(string)
	}
	mutator := WorktreeMutator{MaxConceptBytes: c.MaxConceptBytes, Proposals: c.Queue.Proposals, Now: c.Queue.Now, Random: c.Random}
	result, err := transaction(ctx, repository.TransactionRequest{Operation: control.OperationRequest{OpID: operationID, Principal: actor.Policy.Principal, IdempotencyKey: key, ToolName: "memory_" + kind, RequestJSON: request, ClientInstanceID: actor.ClientInstanceID, MCPSessionID: actor.MCPSessionID, SourceChat: actor.SourceChat}, ExpectedRevision: expected, CommitMessage: message, AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}, func(ctx context.Context, root string) ([]string, error) {
		return mutator.ApplyChanges(ctx, root, changes, actor.Policy.Principal, actor.Policy, nil)
	})
	if err != nil {
		return nil, options, err
	}
	// Deliberately after publication, also on replay. These failures cannot roll
	// back Git/the succeeded operation; reconciliation must use the original key.
	if err = c.Queue.refreshAll(ctx, repo); err != nil {
		return nil, options, err
	}
	if c.ChangedConcepts != nil {
		if err = c.ChangedConcepts(ctx, actor.Policy, result.ChangedPaths); err != nil {
			return nil, options, err
		}
	}
	preview, err := mutator.previewChanges(c.Queue.Paths.CurrentDir, changes, ops)
	if err != nil {
		return nil, options, err
	}
	options.RepoRevision = &result.ResultRevision
	options.IndexRevision = &result.ResultRevision
	options.OperationID = &result.Operation.OpID
	options.Warnings = warnings
	return map[string]any{"changed_paths": result.ChangedPaths, "diff": preview, "replayed": result.Replayed}, options, nil
}
func (c *ProposalControls) directMutationWarnings(changes []ProposalChange, ops mutationIO) ([]string, error) {
	for _, change := range changes {
		if change["kind"] != "patch" || change["body"] == nil {
			continue
		}
		entry, err := ops.read(c.Queue.Paths.CurrentDir, change["path"].(string))
		if err != nil {
			var bundle *repository.BundleError
			var front *repository.FrontmatterError
			if errors.As(err, &bundle) || errors.As(err, &front) || errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		kinds, err := assets.ListAssetKinds(c.Queue.Paths.CurrentDir, entry.Document.Frontmatter.ID)
		if err != nil {
			return nil, err
		}
		for _, kind := range kinds {
			versions, err := assets.ListAssetVersions(c.Queue.Paths.CurrentDir, entry.Document.Frontmatter.ID, kind)
			if err != nil {
				return nil, err
			}
			if len(versions) > 0 {
				return nil, &Error{"conflict", "direct concept body patch would bypass asset parity review; submit a proposal that updates the concept body together with all corresponding asset packs"}
			}
		}
	}
	return []string{"direct_mutation_bypasses_proposal_review; prefer memory_propose, memory_proposal_review and memory_proposal_apply"}, nil
}
