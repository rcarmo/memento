package service

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/internal/repository"
)

// AssetPrune serialises retention planning with repository publication. Versions
// referenced by submitted/approved proposals survive even past proposal expiry:
// the source checks stored status here without refreshing the queue.
func (c *ProposalControls) AssetPrune(ctx context.Context, actor ProposalActor, id, kind string, keep int, expected, key string) (map[string]any, SuccessOptions, error) {
	manager := repository.TransactionManager{Paths: c.Queue.Paths, Operations: control.Operations{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}, Now: c.Queue.Now, DerivedUpdate: c.DerivedUpdate}
	var data map[string]any
	var options SuccessOptions
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		data, options, err = c.assetPrune(ctx, actor, id, kind, keep, expected, key, manager.ApplyUnderLock, defaultMutationIO())
		return err
	})
	return data, options, err
}

// assetPrune requires the caller's repository lock (Jobs supplies it).
func (c *ProposalControls) assetPrune(ctx context.Context, actor ProposalActor, id, kind string, keep any, expected, key string, transaction proposalTransaction, ops mutationIO) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if err := access.RequireRole(actor.Policy, "curator"); err != nil {
		return nil, options, err
	}
	path, err := c.resolvePath(actor.Policy, id, "write")
	if err != nil {
		return nil, options, err
	}
	if _, err = access.AuthorizePath(actor.Policy, path, "write"); err != nil {
		return nil, options, err
	}
	entry, err := repository.ReadBundleEntry(c.Queue.Paths.CurrentDir, path)
	if err != nil {
		return nil, options, err
	}
	concept := entry.Document.Frontmatter.ID
	versions, err := assets.ListAssetVersions(c.Queue.Paths.CurrentDir, concept, kind)
	if err != nil {
		return nil, options, err
	}
	// Python accepts bool as a retention count, but hashes its JSON boolean,
	// not 0/1. Preserve that input in the durable request while sorting by count.
	count, ok := keep.(int)
	if !ok {
		count, err = toolInteger(keep)
		if err != nil {
			return nil, options, &Error{"validation_error", "keep must be a supported integer"}
		}
	}
	_, pruned, err := assets.RetentionPartition(versions, count)
	if err != nil {
		return nil, options, &Error{"validation_error", err.Error()}
	}
	attached, err := c.Queue.Proposals.ListAssets(ctx, control.ProposalAssetQuery{ConceptPath: &path, AssetKind: &kind})
	if err != nil {
		return nil, options, err
	}
	active := map[string]bool{}
	for _, asset := range attached {
		proposal, err := c.Queue.Proposals.Get(ctx, asset.ProposalID)
		if err != nil {
			return nil, options, err
		}
		if proposal.Status == control.Submitted || proposal.Status == control.Approved {
			active[asset.Version] = true
		}
	}
	selected := []string{}
	removing := map[string]bool{}
	for _, version := range pruned {
		if !active[version] {
			selected = append(selected, version)
			removing[version] = true
		}
	}
	kept := []string{}
	for _, version := range versions {
		if !removing[version] {
			kept = append(kept, version)
		}
	}
	payload := map[string]any{"concept_path": path, "asset_kind": kind, "kept_versions": kept, "pruned_versions": selected}
	// No-op precedes expected-revision and idempotency validation in Python.
	if len(selected) == 0 {
		return payload, options, nil
	}
	request, err := pyjson.Dumps(map[string]any{"concept_path": path, "asset_kind": kind, "keep": keep, "expected_revision": expected})
	if err != nil {
		return nil, options, err
	}
	operationID, err := c.newOperationID()
	if err != nil {
		return nil, options, err
	}
	result, err := transaction(ctx, repository.TransactionRequest{Operation: control.OperationRequest{OpID: operationID, Principal: actor.Policy.Principal, IdempotencyKey: key, ToolName: "memory_asset_prune", RequestJSON: request, ClientInstanceID: actor.ClientInstanceID, MCPSessionID: actor.MCPSessionID, SourceChat: actor.SourceChat}, ExpectedRevision: expected, CommitMessage: "memory: prune " + kind + " assets for " + path, AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}, func(_ context.Context, worktree string) ([]string, error) {
		return pruneAssetFiles(worktree, concept, kind, selected, ops)
	})
	if err != nil {
		return nil, options, err
	}
	payload["replayed"] = result.Replayed
	options.RepoRevision = &result.ResultRevision
	options.IndexRevision = &result.ResultRevision
	options.OperationID = &result.Operation.OpID
	return payload, options, nil
}
func pruneAssetFiles(root, concept, kind string, versions []string, ops mutationIO) ([]string, error) {
	changed := []string{}
	for _, version := range versions {
		meta, zip, err := assets.AssetVersionPaths(concept, kind, version)
		if err != nil {
			return nil, err
		}
		for _, path := range []string{meta, zip} {
			info, err := ops.stat(filepath.Join(root, path[1:]))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			// os.Root.Remove also removes empty directories, unlike Path.unlink.
			// Directory targets (including directory symlinks) fail closed.
			if info.IsDir() {
				return nil, &os.PathError{Op: "unlink", Path: path, Err: syscall.EISDIR}
			}
			if err = ops.remove(root, path); err != nil {
				return nil, err
			}
			changed = append(changed, path)
		}
	}
	return changed, nil
}
