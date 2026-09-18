package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/internal/pyjson"
	"github.com/rcarmo/memento/go/repository"
)

// ProposalControls owns serialised proposal mutations in one daemon. Its policy
// must come from trusted request identity, never tool arguments. The enclosing
// daemon must hold the writer lease. Rebase shares the repository transaction
// lock with Apply/Recover; direct queue refresh must run under that same lock.
// No MCP endpoints are registered by this helper.
type ProposalControls struct {
	Queue            ProposalQueue
	Random           io.Reader
	DerivedIndexPath string
	MaxConceptBytes  int
	DerivedUpdate    repository.DerivedUpdateCallback
	// ChangedConcepts is optional hot-working-memory tracking. Core queue refresh
	// always runs first; enabled intelligent tiers must supply their own hook.
	ChangedConcepts func(context.Context, access.EffectivePolicy, []string) error
}

// ProposalActor contains resolved authorisation plus trusted client metadata.
type ProposalActor struct {
	Policy           access.EffectivePolicy
	ClientInstanceID *string
	MCPSessionID     *string
	SourceChat       *string
}
type ProposalControlResult struct {
	Data        map[string]any `json:"data"`
	OperationID string         `json:"operation_id"`
}

func (c *ProposalControls) Rebase(ctx context.Context, actor ProposalActor, id, expected, key string) (ProposalControlResult, error) {
	var result ProposalControlResult
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		result, err = c.rebase(ctx, actor, id, expected, key, defaultProposalRepository())
		return err
	})
	return result, err
}
func (c *ProposalControls) rebase(ctx context.Context, actor ProposalActor, id, expected, key string, repo proposalRepository) (ProposalControlResult, error) {
	empty := ProposalControlResult{}
	proposerOrCurator := false
	for _, role := range actor.Policy.Roles {
		if role == "proposer" || role == "curator" {
			proposerOrCurator = true
			break
		}
	}
	if !proposerOrCurator {
		return empty, access.RequireRole(actor.Policy, "proposer")
	}
	q := c.Queue
	proposal, err := q.Proposals.Get(ctx, id)
	if err != nil {
		return empty, err
	}
	if err = RequireProposalAccess(actor.Policy, proposal, true); err != nil {
		return empty, err
	}
	request, replay, err := c.controlReplay(ctx, actor.Policy, key, "memory_proposal_rebase", map[string]any{"proposal_id": id, "expected_revision": expected})
	if err != nil {
		return empty, err
	}
	if replay != nil {
		payload, err := replay.ReplayPayload()
		if err != nil {
			return empty, err
		}
		if payload == nil {
			payload = map[string]any{}
		}
		payload["replayed"] = true
		return ProposalControlResult{payload, replay.OpID}, nil
	}
	// Refresh is a separate committed operation in Python. A later rejected
	// rebase may still leave its stale/expired status and system event persisted.
	proposal, err = q.refresh(ctx, proposal, "", nil, repo)
	if err != nil {
		return empty, err
	}
	revision, err := repo.main(q.Paths)
	if err != nil {
		return empty, err
	}
	if expected != revision {
		return empty, &Error{"needs_rebase", "repository advanced; read current proposal and revision before rebasing"}
	}
	if proposal.Status != control.NeedsRebase && proposal.Status != control.Conflicted {
		return empty, &Error{"conflict", fmt.Sprintf("proposal %s is %s; no rebase available", id, proposal.Status)}
	}
	conflicts, err := q.conflicts(ctx, proposal, "", nil, nil, repo)
	if err != nil {
		return empty, err
	}
	for _, item := range conflicts {
		if item.Status != "clean" {
			return empty, &Error{"conflict", "proposal conflicts with current memory; inspect per-change conflicting_paths"}
		}
	}
	changes, err := proposalChanges(proposal)
	if err != nil {
		return empty, err
	}
	for _, change := range changes {
		if change["kind"] == "trash" {
			return empty, &Error{"needs_rebase", "submit fresh archival proposal with current impact report"}
		}
	}
	if err = ValidateChangeAuthorization(actor.Policy, changes, "write"); err != nil {
		return empty, err
	}
	if err = c.validateSkillBindings(changes, repo); err != nil {
		return empty, err
	}
	result := map[string]any{"proposal_id": id, "previous_base_revision": proposal.BaseRevision, "base_revision": revision, "status": "submitted", "replayed": false}
	var operationID string
	err = control.WithTransaction(ctx, q.Proposals.DB, func(tx *sql.Tx) error {
		if err := q.event(ctx, tx, proposal, actor.Policy.Principal, "rebase", control.Submitted, revision, map[string]any{"reviewed_by": nullableText(proposal.ReviewedBy), "review_comment": nullableText(proposal.ReviewComment)}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE proposals SET base_revision=?,status='submitted',reviewed_by=NULL,review_comment=NULL,updated_at=? WHERE proposal_id=?", revision, q.now(), id); err != nil {
			return err
		}
		var err error
		operationID, err = c.journalControl(ctx, tx, actor, key, "memory_proposal_rebase", request, revision, result)
		return err
	})
	if err != nil {
		return empty, err
	}
	return ProposalControlResult{result, operationID}, nil
}

func nullableText(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func proposalChanges(record control.ProposalRecord) ([]ProposalChange, error) {
	patch, err := record.Patch()
	if err != nil {
		return nil, err
	}
	raw, ok := patch["changes"].([]any)
	if !ok {
		return nil, &ChangeValidationError{"proposal changes must be a list"}
	}
	return NormalizeProposalChanges(raw)
}

func (c *ProposalControls) validateSkillBindings(changes []ProposalChange, repo proposalRepository) error {
	for _, change := range changes {
		if change["kind"] != "attach_asset_pack" || change["asset_kind"] != "skill" {
			continue
		}
		var tags []string
		for _, candidate := range changes {
			if candidate["path"] != change["path"] {
				continue
			}
			if candidate["kind"] == "create" || (candidate["kind"] == "patch" && candidate["tags"] != nil) {
				tags = candidate["tags"].([]string)
			}
		}
		if tags == nil {
			entry, err := repo.read(c.Queue.Paths.CurrentDir, change["path"].(string))
			if err != nil {
				return err
			}
			tags = entry.Document.Frontmatter.Tags
		}
		skill := false
		for _, tag := range tags {
			if tag == "skill" {
				skill = true
				break
			}
		}
		if !skill {
			return &Error{"validation_error", "skill asset concepts must include the 'skill' tag"}
		}
	}
	return nil
}

func (c *ProposalControls) controlReplay(ctx context.Context, policy access.EffectivePolicy, key, method string, args map[string]any) (string, *control.OperationRecord, error) {
	request := map[string]any{"method": method}
	for name, value := range args {
		request[name] = value
	}
	raw, err := pyjson.Dumps(request)
	if err != nil {
		return "", nil, err
	}
	existing, err := (control.Operations{DB: c.Queue.Proposals.DB}).ByIdempotency(ctx, policy.Principal, key)
	if err != nil {
		return "", nil, err
	}
	if existing != nil && existing.RequestHash != (control.OperationRequest{RequestJSON: raw}).RequestHash() {
		return "", nil, &control.IdempotencyConflictError{}
	}
	return raw, existing, nil
}
func (c *ProposalControls) newOperationID() (string, error) {
	random := c.Random
	if random == nil {
		random = rand.Reader
	}
	var bytes [16]byte
	if _, err := io.ReadFull(random, bytes[:]); err != nil {
		return "", err
	}
	bytes[6] = bytes[6]&15 | 64
	bytes[8] = bytes[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:]), nil
}
func (c *ProposalControls) journalControl(ctx context.Context, tx *sql.Tx, actor ProposalActor, key, method, request, revision string, result map[string]any) (string, error) {
	id, err := c.newOperationID()
	if err != nil {
		return "", err
	}
	payload, err := pyjson.Dumps(result)
	if err != nil {
		return "", err
	}
	hash := (control.OperationRequest{RequestJSON: request}).RequestHash()
	now := c.Queue.now()
	_, err = tx.ExecContext(ctx, `INSERT INTO operations(op_id,idempotency_key,principal,client_instance_id,tool_name,request_hash,base_revision,result_revision,state,request_json,result_json,created_at,started_at,finished_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, key, actor.Policy.Principal, actor.ClientInstanceID, method, hash, revision, revision, "succeeded", request, payload, now, now, now)
	return id, err
}
