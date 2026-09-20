package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
)

// Error is a service policy failure, distinct from malformed persisted input.
type Error struct {
	Class   string
	Message string
}

func (e *Error) Error() string { return e.Message }

// ChangeValidationError corresponds to source model-validation failures. These
// propagate from proposal visibility rather than being converted into a denial.
// Diagnostic text is not yet Pydantic-compatible.
type ChangeValidationError struct{ Message string }

func (e *ChangeValidationError) Error() string { return e.Message }

// ProposalChange contains the JSON-mode values of a validated change. Construct
// changes with NormalizeProposalChanges before passing them to policy helpers.
type ProposalChange map[string]any

// NormalizeProposalChanges preserves the source's defaults, nulls and list order.
// It validates shape only: path, concept type, assets and body policy are checked
// later by the service operations that consume these changes.
func NormalizeProposalChanges(values []any) ([]ProposalChange, error) {
	result := make([]ProposalChange, 0, len(values))
	for _, value := range values {
		raw, ok := value.(map[string]any)
		if !ok {
			return nil, &ChangeValidationError{"proposal change must be an object"}
		}
		kind, _ := raw["kind"].(string)
		required := []string{"kind", "path"}
		optional := []string{}
		lists := []string{}
		manifest := false
		switch kind {
		case "create":
			required = append(required, "concept_type", "title", "body")
			optional = []string{"description"}
			lists = []string{"tags", "aliases"}
		case "patch":
			optional = []string{"title", "description", "body", "status"}
			lists = []string{"tags", "aliases"}
		case "rename":
			required = append(required, "new_path")
		case "trash":
		case "attach_asset_pack":
			required = append(required, "asset_kind", "version", "asset_id", "zip_sha256")
			manifest = true
		default:
			label := fmt.Sprint(raw["kind"])
			if raw["kind"] == nil {
				label = "None"
			}
			return nil, &Error{"validation_error", "unsupported change kind: " + label}
		}
		change := ProposalChange{}
		for _, field := range required {
			v, ok := raw[field].(string)
			if !ok {
				return nil, &ChangeValidationError{field + " must be a string"}
			}
			change[field] = v
		}
		for _, field := range optional {
			v := raw[field]
			if v != nil {
				if _, ok := v.(string); !ok {
					return nil, &ChangeValidationError{field + " must be a string or null"}
				}
			}
			change[field] = v
		}
		if value := change["status"]; value != nil && value != "active" && value != "deprecated" && value != "tombstone" {
			return nil, &ChangeValidationError{"invalid concept status"}
		}
		for _, field := range lists {
			v, exists := raw[field]
			if v == nil && kind == "patch" {
				change[field] = nil
				continue
			}
			if !exists {
				change[field] = []string{}
				continue
			}
			items, ok := v.([]any)
			if !ok {
				return nil, &ChangeValidationError{field + " must be a string list"}
			}
			normalized := make([]string, 0, len(items))
			for _, item := range items {
				str, ok := item.(string)
				if !ok {
					return nil, &ChangeValidationError{field + " entries must be strings"}
				}
				normalized = append(normalized, str)
			}
			change[field] = normalized
		}
		if manifest {
			v, ok := raw["manifest"].(map[string]any)
			if !ok {
				return nil, &ChangeValidationError{"manifest must be an object"}
			}
			change["manifest"] = v
		}
		for field := range raw {
			if _, ok := change[field]; !ok {
				return nil, &ChangeValidationError{"unknown change field: " + field}
			}
		}
		result = append(result, change)
	}
	return result, nil
}

func validateArchivalBatch(changes []ProposalChange) error {
	trash := 0
	seen := map[string]bool{}
	for _, change := range changes {
		if change["kind"] == "trash" {
			trash++
		}
		seen[change["path"].(string)] = true
	}
	if trash > 0 {
		if len(changes) > 20 || trash != len(changes) {
			return &Error{"validation_error", "archival proposals require 1-20 trash changes without mixed mutations"}
		}
		if len(seen) != len(changes) {
			return &Error{"validation_error", "duplicate archival target"}
		}
	}
	return nil
}

// ValidateChangeAuthorization requires already normalised changes. As in the
// source, any action other than read/write requests both permissions.
func ValidateChangeAuthorization(policy access.EffectivePolicy, changes []ProposalChange, action string) error {
	if err := validateArchivalBatch(changes); err != nil {
		return err
	}
	for _, change := range changes {
		paths := []string{change["path"].(string)}
		if change["kind"] == "rename" {
			paths = append(paths, change["new_path"].(string))
		}
		for _, path := range paths {
			if strings.HasPrefix(path, "/trash/") {
				return &Error{"validation_error", "use trash, restore or purge for items in /trash/"}
			}
		}
		for _, path := range paths {
			if !strings.HasSuffix(path, ".md") {
				return &Error{"validation_error", "concept paths must end with .md"}
			}
		}
		actions := []string{action}
		if action != "read" && action != "write" {
			actions = []string{"read", "write"}
		}
		for _, action := range actions {
			for _, path := range paths {
				if _, err := access.AuthorizePath(policy, path, action); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// CanAccessProposal checks both author/curator ownership and every changed path.
// Admin alone does not bypass ownership, and review additionally requires writes.
func CanAccessProposal(policy access.EffectivePolicy, proposal control.ProposalRecord, requireWrite bool) (bool, error) {
	curator := false
	for _, role := range policy.Roles {
		if role == "curator" {
			curator = true
			break
		}
	}
	if proposal.AuthorPrincipal != policy.Principal && !curator {
		return false, nil
	}
	changes, err := proposalChanges(proposal)
	if err == nil {
		err = ValidateChangeAuthorization(policy, changes, "read")
	}
	if err == nil && requireWrite {
		err = ValidateChangeAuthorization(policy, changes, "write")
	}
	if err != nil {
		var serviceErr *Error
		var authErr *access.AuthorizationError
		if errors.As(err, &serviceErr) || errors.As(err, &authErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func RequireProposalAccess(policy access.EffectivePolicy, proposal control.ProposalRecord, requireWrite bool) error {
	allowed, err := CanAccessProposal(policy, proposal, requireWrite)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	action := "read"
	if requireWrite {
		action = "review"
	}
	return &Error{"forbidden", fmt.Sprintf("principal %s cannot %s proposal %s", policy.Principal, action, proposal.ProposalID)}
}

func (q ProposalQueue) VisibleProposal(ctx context.Context, policy access.EffectivePolicy, id string) (control.ProposalRecord, error) {
	proposal, err := q.Proposals.Get(ctx, id)
	if err != nil {
		return control.ProposalRecord{}, err
	}
	if err = RequireProposalAccess(policy, proposal, false); err != nil {
		return control.ProposalRecord{}, err
	}
	return proposal, nil
}
