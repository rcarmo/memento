package service

import (
	"context"
	_ "embed"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/umcp"
)

//go:embed staging_tools.json
var stagingToolDefinitions []byte

// RegisterStagingReadProposalTools exposes the 15 implemented development tools;
// full configured surfaces/catalog are not yet advertised as available.
func (j *Jobs) RegisterStagingReadProposalTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callStagingOrProposalTool, notify, stagingToolDefinitions)
}
func (j *Jobs) callStagingOrProposalTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if name != "memory_asset_stage_begin" && name != "memory_asset_stage_status" {
		return j.callProposalTool(ctx, name, args)
	}
	// These two Python wrappers run directly, not through _memory_call. They check
	// the resolved principal roles (not namespace policy) and return no busy state.
	j.stagingMu.Lock()
	defer j.stagingMu.Unlock()
	principal, _, err := j.Identity.requestPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	data, options, err := stageTool(ctx, j.Controls.Staging, principal, name, args)
	if err != nil {
		return FailureEnvelope(err)
	}
	return j.Controls.Queue.SuccessEnvelope(data, options)
}
func stageTool(ctx context.Context, store *assets.StagingStore, principal access.Principal, name string, args map[string]any) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	proposer := false
	for _, role := range principal.Roles {
		if role == "proposer" {
			proposer = true
		}
	}
	if !proposer {
		return nil, options, &Error{"validation_error", "proposer role is required"}
	}
	if store == nil {
		return nil, options, &Error{"validation_error", "asset staging is unavailable"}
	}
	a := toolArguments{values: args}
	key := a.text("idempotency_key")
	var ticket assets.UploadTicket
	var token string
	var err error
	if name == "memory_asset_stage_begin" {
		kind, version := a.text("asset_kind"), a.text("version")
		if a.err != nil {
			return nil, options, a.err
		}
		ticket, token, err = store.BeginUpload(ctx, principal.Name, key, kind, version)
	} else {
		if a.err != nil {
			return nil, options, a.err
		}
		ticket, err = store.TicketStatus(ctx, principal.Name, key)
	}
	if err != nil {
		return nil, options, err
	}
	now := time.Now()
	if store.Now != nil {
		now = store.Now()
	}
	state, err := ticket.State(now.UTC().Truncate(time.Second))
	if err != nil {
		return nil, options, &ChangeValidationError{Message: err.Error()}
	}
	payload := map[string]any{"state": state, "asset_kind": ticket.AssetKind, "version": ticket.Version, "idempotency_key": ticket.IdempotencyKey, "expires_at": ticket.ExpiresAt}
	if name == "memory_asset_stage_begin" {
		payload["upload_path"] = "/assets/staging/upload"
		payload["upload_method"] = "POST"
		payload["upload_content_type"] = "application/zip"
		payload["upload_ticket_header"] = "X-Memento-Upload-Ticket"
		payload["upload_ticket"] = token
		payload["workflow"] = "memory://workflow/asset_pack"
		payload["proposal_contract"] = "memory://catalog/propose"
	} else {
		payload["staged_asset_id"] = nullableText(ticket.StagedAssetID)
		payload["consumed_at"] = nullableText(ticket.ConsumedAt)
		if ticket.StagedAssetID != nil {
			asset, err := store.Get(ctx, principal.Name, *ticket.StagedAssetID, false)
			if err != nil {
				return nil, options, err
			}
			payload["staged_asset"] = stagedPayload(asset)
		}
	}
	options.NextTools = []string{"memory_asset_stage_status", "memory://workflow/asset_pack", "memory://catalog/propose", "memory_execute"}
	return payload, options, nil
}
