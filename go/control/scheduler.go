package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/rcarmo/memento/go/internal/pyjson"
	"time"
)

type ModelAttempt struct {
	Model   string `json:"model"`
	Outcome string `json:"outcome"`
}
type SchedulerRunRecord struct {
	RunID                      string `json:"run_id"`
	JobName                    string `json:"job_name"`
	WindowKey                  string `json:"window_key"`
	BaseRevision, EndRevision  *string
	State                      string `json:"state"`
	SignalCount, ProposalCount int
	ModelChain                 []ModelAttempt
	StartedAt                  string
	FinishedAt, ErrorMessage   *string
}
type SchedulerClaim struct {
	Created bool
	Record  SchedulerRunRecord
}
type SchedulerConflictError struct{ JobName string }

func (e *SchedulerConflictError) Error() string { return "job " + e.JobName + " is already running" }

type Scheduler struct {
	DB   *sql.DB
	Now  func() time.Time
	UUID func() string
}

func (s Scheduler) now() string {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return now.UTC().Truncate(time.Second).Format(time.RFC3339)
}
func (s Scheduler) id() string {
	if s.UUID != nil {
		return s.UUID()
	}
	return uuid.NewString()
}

const schedulerColumns = "run_id,job_name,window_key,base_revision,end_revision,state,signal_count,proposal_count,model_chain_json,started_at,finished_at,error_message"

func (s Scheduler) Claim(ctx context.Context, job, window string, base *string) (SchedulerClaim, error) {
	if _, err := scanScheduler(s.DB.QueryRowContext(ctx, "SELECT "+schedulerColumns+" FROM scheduler_runs WHERE job_name=? AND state='running' LIMIT 1", job)); err == nil {
		return SchedulerClaim{}, &SchedulerConflictError{job}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SchedulerClaim{}, err
	}
	if row, err := scanScheduler(s.DB.QueryRowContext(ctx, "SELECT "+schedulerColumns+" FROM scheduler_runs WHERE job_name=? AND window_key=?", job, window)); err == nil {
		return SchedulerClaim{false, row}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SchedulerClaim{}, err
	}
	id, started := s.id(), s.now()
	if _, err := s.DB.ExecContext(ctx, "INSERT INTO scheduler_runs(run_id,job_name,window_key,base_revision,state,started_at) VALUES(?,?,?,?,'running',?)", id, job, window, base, started); err != nil {
		return SchedulerClaim{}, err
	}
	record, err := s.Get(ctx, id)
	return SchedulerClaim{true, record}, err
}
func (s Scheduler) Get(ctx context.Context, id string) (SchedulerRunRecord, error) {
	return scanScheduler(s.DB.QueryRowContext(ctx, "SELECT "+schedulerColumns+" FROM scheduler_runs WHERE run_id=?", id))
}
func (s Scheduler) Finish(ctx context.Context, id, state string, end *string, signals, proposals int, chain []ModelAttempt, message *string) (SchedulerRunRecord, error) {
	values := make([]any, 0, len(chain))
	for _, attempt := range chain {
		values = append(values, map[string]any{"model": attempt.Model, "outcome": attempt.Outcome})
	}
	raw, _ := pyjson.Dumps(values)
	var err error
	if _, err = s.DB.ExecContext(ctx, "UPDATE scheduler_runs SET state=?,end_revision=?,signal_count=?,proposal_count=?,model_chain_json=?,error_message=?,finished_at=? WHERE run_id=?", state, end, signals, proposals, raw, message, s.now(), id); err != nil {
		return SchedulerRunRecord{}, err
	}
	return s.Get(ctx, id)
}
func scanScheduler(row rowScanner) (SchedulerRunRecord, error) {
	var record SchedulerRunRecord
	var raw *string
	if err := row.Scan(&record.RunID, &record.JobName, &record.WindowKey, &record.BaseRevision, &record.EndRevision, &record.State, &record.SignalCount, &record.ProposalCount, &raw, &record.StartedAt, &record.FinishedAt, &record.ErrorMessage); err != nil {
		return record, err
	}
	record.ModelChain = []ModelAttempt{}
	if raw == nil {
		return record, nil
	}
	var values []any
	if err := json.Unmarshal([]byte(*raw), &values); err != nil {
		return record, err
	}
	for _, value := range values {
		switch item := value.(type) {
		case string:
			record.ModelChain = append(record.ModelChain, ModelAttempt{item, "success"})
		case map[string]any:
			model, modelOK := item["model"].(string)
			outcome, outcomeOK := item["outcome"].(string)
			if !modelOK || !outcomeOK {
				return record, errors.New("invalid scheduler model chain")
			}
			record.ModelChain = append(record.ModelChain, ModelAttempt{model, outcome})
		default:
			return record, errors.New("invalid scheduler model chain")
		}
	}
	return record, nil
}
