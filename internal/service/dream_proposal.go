package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/execute"
	"github.com/rcarmo/memento/internal/repository"
	"io"
	"strings"
	"time"
)

type DreamCitation struct{ ID, Path, Revision, Title string }
type DreamProposalDraft struct {
	Intent, Rationale               string
	Consulted                       []map[string]any
	Contradictions, ReciprocalLinks []map[string]any
	Changes                         []map[string]any
}
type DreamProposalLimits struct{ MaxChanges, MaxBodyChars, MaxConsultedConcepts int }

func ParseDreamProposal(raw string) (DreamProposalDraft, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return DreamProposalDraft{}, fmt.Errorf("model output is not valid JSON: %w", err)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return DreamProposalDraft{}, errors.New("model output is not valid JSON: trailing JSON")
	}
	intent := "model proposal"
	if value, ok := object["intent"]; ok {
		intent = fmt.Sprint(value)
	}
	rationale := ""
	if value, ok := object["rationale"]; ok {
		rationale = strings.TrimSpace(fmt.Sprint(value))
	}
	changes, err := execute.ValidateProposalChanges(defaultArray(object["changes"]))
	if err != nil {
		return DreamProposalDraft{}, err
	}
	consulted, err := strictObjectArray(object["consulted_concepts"], []string{"id", "path", "revision", "title"})
	if err != nil {
		return DreamProposalDraft{}, err
	}
	contradictions, err := strictObjectArray(object["contradictions"], []string{"path", "summary"})
	if err != nil {
		return DreamProposalDraft{}, err
	}
	links, err := strictObjectArray(object["reciprocal_links"], []string{"source_path", "target_path", "justification"})
	if err != nil {
		return DreamProposalDraft{}, err
	}
	allowed := map[string]bool{"intent": true, "rationale": true, "changes": true, "consulted_concepts": true, "contradictions": true, "reciprocal_links": true}
	for key := range object {
		if !allowed[key] {
			return DreamProposalDraft{}, fmt.Errorf("unexpected proposal field: %s", key)
		}
	}
	return DreamProposalDraft{intent, rationale, consulted, contradictions, links, changes}, nil
}
func defaultArray(value any) any {
	if value == nil {
		return []any{}
	}
	return value
}
func strictObjectArray(value any, fields []string) ([]map[string]any, error) {
	if value == nil {
		return []map[string]any{}, nil
	}
	rows, ok := value.([]any)
	if !ok {
		return nil, errors.New("proposal field must be an array")
	}
	out := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("proposal record must be an object")
		}
		if len(row) != len(fields) {
			return nil, errors.New("proposal record fields are invalid")
		}
		copy := map[string]any{}
		for _, field := range fields {
			value, ok := row[field].(string)
			if !ok {
				return nil, errors.New("proposal record field must be a string")
			}
			copy[field] = value
		}
		out = append(out, copy)
	}
	return out, nil
}
func ValidateDreamProposal(root, revision string, draft DreamProposalDraft, limits DreamProposalLimits) error {
	if draft.Rationale == "" {
		return errors.New("Dream proposal rationale must not be empty")
	}
	if len(draft.Changes) == 0 {
		return errors.New("Dream proposal must include at least one change")
	}
	if len(draft.Changes) > limits.MaxChanges {
		return errors.New("Dream proposal exceeds configured change limits")
	}
	if len(draft.Consulted) > limits.MaxConsultedConcepts {
		return errors.New("Dream proposal exceeds consulted concept limits")
	}
	seen := map[string]bool{}
	for _, citation := range draft.Consulted {
		id := citation["id"].(string)
		if seen[id] {
			return errors.New("Dream proposal must cite every consulted concept")
		}
		seen[id] = true
		if citation["revision"].(string) != revision {
			return errors.New("Dream proposal citations must match consulted concepts")
		}
	}
	for _, change := range draft.Changes {
		kind := change["kind"].(string)
		if kind != "create" && kind != "patch" {
			return errors.New("Dream may only create normal proposals")
		}
		path := change["path"].(string)
		if _, err := repository.ValidateRepositoryWritePath(root, path); err != nil {
			return err
		}
		if body, ok := change["body"].(string); ok && len([]rune(body)) > limits.MaxBodyChars {
			return errors.New("proposal body exceeds configured limits")
		}
	}
	return nil
}
func StoreDreamProposal(ctx context.Context, db *sql.DB, revision string, draft DreamProposalDraft, signalKeys []string, now time.Time, id func() string) (control.ProposalRecord, error) {
	if id == nil {
		id = uuid.NewString
	}
	proposals := control.Proposals{DB: db, Now: func() time.Time { return now }}
	var record control.ProposalRecord
	err := control.WithTransaction(ctx, db, func(tx *sql.Tx) error {
		rationale := draft.Rationale
		patch := map[string]any{"changes": mapsToAny(draft.Changes), "consulted_concepts": mapsToAny(draft.Consulted), "contradictions": mapsToAny(draft.Contradictions), "reciprocal_links": mapsToAny(draft.ReciprocalLinks), "dream_signal_keys": stringsToAny(signalKeys)}
		var err error
		record, err = proposals.CreateInTx(ctx, tx, control.ProposalRequest{ProposalID: id(), AuthorPrincipal: "dream", BaseRevision: revision, Intent: draft.Intent, Rationale: &rationale, Patch: patch})
		if err != nil {
			return err
		}
		if len(signalKeys) > 0 {
			marks := strings.TrimSuffix(strings.Repeat("?,", len(signalKeys)), ",")
			args := []any{"proposed"}
			for _, key := range signalKeys {
				args = append(args, key)
			}
			_, err = tx.ExecContext(ctx, "UPDATE dream_signals SET status=? WHERE dedupe_key IN ("+marks+")", args...)
		}
		return err
	})
	return record, err
}
func mapsToAny(values []map[string]any) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}
func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}
