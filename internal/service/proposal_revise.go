package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

// Revise creates a new curator-owned proposal from a clean subset. The source
// record/assets remain intact (apart from the separately committed refresh).
// Like Propose, this operation has no durable idempotency key.
func (c *ProposalControls) Revise(ctx context.Context, actor ProposalActor, id string, selected []int, expected string, intent, rationale *string) (map[string]any, error) {
	var result map[string]any
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		result, err = c.revise(ctx, actor, id, selected, expected, intent, rationale, defaultProposalRepository())
		return err
	})
	return result, err
}
func (c *ProposalControls) revise(ctx context.Context, actor ProposalActor, id string, selected []int, expected string, intent, rationale *string, repo proposalRepository) (map[string]any, error) {
	if err := access.RequireRole(actor.Policy, "curator"); err != nil {
		return nil, err
	}
	source, err := c.Queue.Proposals.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = RequireProposalAccess(actor.Policy, source, true); err != nil {
		return nil, err
	}
	source, err = c.Queue.refresh(ctx, source, "", nil, repo)
	if err != nil {
		return nil, err
	}
	if source.Status != control.NeedsRebase && source.Status != control.Conflicted {
		return nil, &Error{"conflict", "only proposals needing rebase can be revised"}
	}
	revision, err := repo.main(c.Queue.Paths)
	if err != nil {
		return nil, err
	}
	if expected != revision {
		return nil, &Error{"conflict", fmt.Sprintf("expected revision %s does not match %s", expected, revision)}
	}
	changes, err := proposalChanges(source)
	if err != nil {
		return nil, err
	}
	indexes, err := selectedIndexes(selected, len(changes))
	if err != nil {
		return nil, err
	}
	conflicts, err := c.Queue.conflicts(ctx, source, "", nil, nil, repo)
	if err != nil {
		return nil, err
	}
	blocked := []string{}
	for _, i := range indexes {
		if conflicts[i].Status != "clean" {
			blocked = append(blocked, strconv.Itoa(i))
		}
	}
	if len(blocked) > 0 {
		return nil, &Error{"conflict", "selected changes conflict with current memory: " + strings.Join(blocked, ", ")}
	}
	for _, i := range indexes {
		if changes[i]["kind"] == "trash" {
			return nil, &Error{"conflict", "submit a fresh archival proposal with a current impact report"}
		}
	}
	if err = validateSelectedAssetPairs(changes, indexes); err != nil {
		return nil, err
	}
	chosen := make([]ProposalChange, 0, len(indexes))
	for _, i := range indexes {
		chosen = append(chosen, changes[i])
	}
	return c.createRevised(ctx, actor, id, revision, chosen, indexes, intent, rationale, repo)
}
func selectedIndexes(selected []int, count int) ([]int, error) {
	seen := map[int]bool{}
	for _, i := range selected {
		seen[i] = true
	}
	indexes := make([]int, 0, len(seen))
	for i := range seen {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)
	if len(indexes) == 0 {
		return nil, &Error{"validation_error", "selected_change_indexes must not be empty"}
	}
	for _, i := range indexes {
		if i < 0 || i >= count {
			return nil, &Error{"validation_error", "selected_change_indexes contains an invalid index"}
		}
	}
	return indexes, nil
}
func validateSelectedAssetPairs(changes []ProposalChange, indexes []int) error {
	selected := map[int]bool{}
	for _, i := range indexes {
		selected[i] = true
	}
	for assetIndex, asset := range changes {
		if asset["kind"] != "attach_asset_pack" {
			continue
		}
		pair := []int{assetIndex}
		for i, candidate := range changes {
			if candidate["path"] == asset["path"] && (candidate["kind"] == "create" || candidate["kind"] == "patch") {
				if _, ok := candidate["body"].(string); ok {
					pair = append(pair, i)
				}
			}
		}
		if len(pair) == 1 {
			continue
		}
		any, all := false, true
		for _, i := range pair {
			any = any || selected[i]
			all = all && selected[i]
		}
		if any && !all {
			sort.Ints(pair)
			text := make([]string, len(pair))
			for i, v := range pair {
				text[i] = strconv.Itoa(v)
			}
			return &Error{"conflict", "concept body and matching asset pack must be selected together: " + strings.Join(text, ", ")}
		}
	}
	return nil
}
func (c *ProposalControls) createRevised(ctx context.Context, actor ProposalActor, id, revision string, changes []ProposalChange, indexes []int, intent, rationale *string, repo proposalRepository) (map[string]any, error) {
	if err := ValidateChangeAuthorization(actor.Policy, changes, "write"); err != nil {
		return nil, err
	}
	if err := c.validateSkillBindings(changes, repo); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	stored := make([]any, 0, len(changes))
	for _, change := range changes {
		stored = append(stored, map[string]any(change))
		if change["kind"] == "attach_asset_pack" {
			ids[change["asset_id"].(string)] = true
		}
	}
	sortedIDs := make([]string, 0, len(ids))
	for id := range ids {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)
	copied := []control.ProposalAssetInput{}
	for _, assetID := range sortedIDs {
		asset, err := c.Queue.Proposals.GetAsset(ctx, id, assetID)
		if err != nil {
			return nil, err
		}
		copied = append(copied, asset.ProposalAssetInput)
	}
	newID, err := c.newOperationID()
	if err != nil {
		return nil, err
	}
	title := "Revise proposal " + id
	if intent != nil && *intent != "" {
		title = *intent
	}
	reason := "Selected clean changes from stale proposal " + id + "."
	if rationale != nil && *rationale != "" {
		reason = *rationale
	}
	positions := make([]any, len(indexes))
	for i, index := range indexes {
		positions[i] = index
	}
	revised, err := c.Queue.Proposals.Create(ctx, control.ProposalRequest{ProposalID: newID, AuthorPrincipal: actor.Policy.Principal, ClientInstanceID: actor.ClientInstanceID, BaseRevision: revision, Intent: title, Rationale: &reason, Patch: map[string]any{"changes": stored, "source_proposal_id": id, "source_change_indexes": positions}, Assets: copied})
	if err != nil {
		return nil, err
	}
	// Preview/payload failure occurs after Create commits, matching Python.
	preview, err := (WorktreeMutator{MaxConceptBytes: c.MaxConceptBytes}).PreviewChanges(c.Queue.Paths.CurrentDir, changes)
	if err != nil {
		return nil, err
	}
	payload, err := c.Queue.payload(ctx, revised, preview, repo)
	if err != nil {
		return nil, err
	}
	return map[string]any{"proposal": payload, "source_proposal_id": id, "selected_change_indexes": positions}, nil
}
