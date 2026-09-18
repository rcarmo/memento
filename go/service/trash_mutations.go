package service

import (
	"context"
	_ "embed"
	"strings"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/internal/pyjson"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

//go:embed trash_tools.json
var trashToolDefinitions []byte

// RegisterTrashMutationTools exposes the 23 implemented development tools.
func (j *Jobs) RegisterTrashMutationTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callStagingOrProposalTool, notify, trashToolDefinitions)
}

// TrashMutation accepts only trash/restore/purge. Purge confirmation is strict:
// only the Go bool true matches Python's `confirm is True`.
func (c *ProposalControls) TrashMutation(ctx context.Context, actor ProposalActor, action, path, expected, key string, confirm any) (map[string]any, SuccessOptions, error) {
	manager := repository.TransactionManager{Paths: c.Queue.Paths, Operations: control.Operations{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}, Now: c.Queue.Now, DerivedUpdate: c.DerivedUpdate}
	var data map[string]any
	var options SuccessOptions
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		data, options, err = c.trashMutation(ctx, actor, action, path, expected, key, confirm, defaultProposalRepository(), manager.ApplyUnderLock, defaultMutationIO())
		return err
	})
	return data, options, err
}
func trashDestination(action, path string) (string, error) {
	if err := repository.ValidateBundlePath(path); err != nil {
		return "", err
	}
	if action == "trash" {
		if strings.HasPrefix(path, "/trash/") || !strings.HasSuffix(path, ".md") {
			return "", &repository.PathSafetyError{Message: "expected an active Markdown concept path"}
		}
		return "/trash" + path, nil
	}
	if !strings.HasPrefix(path, "/trash/") {
		return "", &repository.PathSafetyError{Message: "item must be in /trash/ first"}
	}
	original := strings.TrimPrefix(path, "/trash")
	// Removing /trash from an already canonical /trash/... path preserves
	// canonicality; repeating ValidateBundlePath cannot add a failure.
	if strings.HasPrefix(original, "/trash/") || !strings.HasSuffix(original, ".md") {
		return "", &repository.PathSafetyError{Message: "invalid trash concept path"}
	}
	return original, nil
}
func (c *ProposalControls) trashMutation(ctx context.Context, actor ProposalActor, action, path, expected, key string, confirm any, repo proposalRepository, transaction proposalTransaction, ops mutationIO) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if action != "trash" && action != "restore" && action != "purge" {
		return nil, options, &Error{"validation_error", "unsupported trash action"}
	}
	if action == "purge" && confirm != true {
		return nil, options, &Error{"validation_error", "permanent deletion requires confirm=true; Git history is retained"}
	}
	if err := access.RequireRole(actor.Policy, "curator"); err != nil {
		return nil, options, err
	}
	destination, err := trashDestination(action, path)
	if err != nil {
		return nil, options, err
	}
	for _, target := range []string{path, destination} {
		for _, permission := range []string{"read", "write"} {
			if _, err = access.AuthorizePath(actor.Policy, target, permission); err != nil {
				return nil, options, err
			}
		}
	}
	request, err := pyjson.Dumps(map[string]any{"action": action, "path": path, "expected_revision": expected})
	if err != nil {
		return nil, options, err
	}
	operationID, err := c.newOperationID()
	if err != nil {
		return nil, options, err
	}
	result, err := transaction(ctx, repository.TransactionRequest{Operation: control.OperationRequest{OpID: operationID, Principal: actor.Policy.Principal, IdempotencyKey: key, ToolName: "memory_" + action, RequestJSON: request, ClientInstanceID: actor.ClientInstanceID, MCPSessionID: actor.MCPSessionID, SourceChat: actor.SourceChat}, ExpectedRevision: expected, CommitMessage: "memory: " + action + " " + path, AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}, func(_ context.Context, root string) ([]string, error) {
		return mutateTrash(root, action, path, destination, ops)
	})
	if err != nil {
		return nil, options, err
	}
	if err = c.Queue.refreshAll(ctx, repo); err != nil {
		return nil, options, err
	}
	if c.ChangedConcepts != nil {
		if err = c.ChangedConcepts(ctx, actor.Policy, result.ChangedPaths); err != nil {
			return nil, options, err
		}
	}
	var target any = destination
	if action == "purge" {
		target = nil
	}
	options.RepoRevision = &result.ResultRevision
	options.IndexRevision = &result.ResultRevision
	options.OperationID = &result.Operation.OpID
	return map[string]any{"changed_paths": result.ChangedPaths, "path": path, "destination": target, "history_retained": true, "replayed": result.Replayed}, options, nil
}
func mutateTrash(root, action, path, destination string, ops mutationIO) ([]string, error) {
	entry, err := ops.read(root, path)
	if err != nil {
		return nil, err
	}
	if _, err = repository.ValidateRepositoryWritePath(root, path); err != nil {
		return nil, err
	}
	if action != "purge" {
		target, err := repository.ValidateRepositoryWritePath(root, destination)
		if err != nil {
			return nil, err
		}
		if targetExists(target.AbsolutePath, ops) {
			return nil, &Error{"conflict", "destination already exists; resolve the collision before moving"}
		}
		if err = ops.mkdir(root, mutationParent(destination)); err != nil {
			return nil, err
		}
		if err = ops.rename(root, path, destination); err != nil {
			return nil, err
		}
		return []string{path, destination}, nil
	}
	changed := []string{path}
	id := entry.Document.Frontmatter.ID
	kinds, err := assets.ListAssetKinds(root, id)
	if err != nil {
		return nil, err
	}
	for _, kind := range kinds {
		versions, err := assets.ListAssetVersions(root, id, kind)
		if err != nil {
			return nil, err
		}
		for _, version := range versions {
			// ListAssetKinds/Versions already validated all three components.
			metadata, zip, _ := assets.AssetVersionPaths(id, kind, version)
			for _, assetPath := range []string{metadata, zip} {
				file, err := repository.ValidateRepositoryWritePath(root, assetPath)
				if err != nil {
					return nil, err
				}
				if targetExists(file.AbsolutePath, ops) {
					if err = ops.remove(root, assetPath); err != nil {
						return nil, err
					}
					changed = append(changed, assetPath)
				}
			}
		}
	}
	if err = ops.remove(root, path); err != nil {
		return nil, err
	}
	return changed, nil
}
