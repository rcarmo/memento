package control

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rcarmo/memento/go/internal/pyjson"
)

type OperationState string

const (
	Queued     OperationState = "queued"
	Running    OperationState = "running"
	Succeeded  OperationState = "succeeded"
	Failed     OperationState = "failed"
	Conflict   OperationState = "conflict"
	Recovering OperationState = "recovering"
)

type IdempotencyConflictError struct{}

func (*IdempotencyConflictError) Error() string {
	return "idempotency key already used for a different request"
}

type OperationNotFoundError struct{ OpID string }

func (e *OperationNotFoundError) Error() string { return "unknown operation: " + e.OpID }

type OperationRequest struct {
	OpID             string  `json:"op_id"`
	Principal        string  `json:"principal"`
	IdempotencyKey   string  `json:"idempotency_key"`
	ToolName         string  `json:"tool_name"`
	RequestJSON      string  `json:"request_json"`
	ClientInstanceID *string `json:"client_instance_id"`
	MCPSessionID     *string `json:"mcp_session_id"`
	SourceChat       *string `json:"source_chat"`
}

func (r OperationRequest) RequestHash() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(r.RequestJSON)))
}

type OperationRecord struct {
	OpID           string         `json:"op_id"`
	Principal      string         `json:"principal"`
	IdempotencyKey string         `json:"idempotency_key"`
	ToolName       string         `json:"tool_name"`
	RequestHash    string         `json:"request_hash"`
	State          OperationState `json:"state"`
	RequestJSON    string         `json:"request_json"`
	BaseRevision   *string        `json:"base_revision"`
	ResultRevision *string        `json:"result_revision"`
	ResultJSON     *string        `json:"result_json"`
	ErrorClass     *string        `json:"error_class"`
	ErrorMessage   *string        `json:"error_message"`
}

func (r OperationRecord) ReplayPayload() (map[string]any, error) {
	if r.ResultJSON == nil {
		return nil, nil
	}
	value, err := pyjson.Parse(*r.ResultJSON)
	if err != nil {
		return nil, err
	}
	payload, _ := value.(map[string]any)
	return payload, nil
}

// Operations preserves the reference state transitions, including blind updates
// and retrying failed rows. Identity is the exact request JSON hash, scoped by
// principal/key. Service authorisation must occur before calling these methods.
// Separate handles may race a SELECT/INSERT and get a uniqueness error, just as
// in Python; transaction serialization is supplied by the transaction manager.
type Operations struct {
	DB  *sql.DB
	Now func() time.Time
}

func (o Operations) now() string {
	if o.Now != nil {
		return o.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	}
	return time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
}

const operationColumns = "op_id,principal,idempotency_key,tool_name,request_hash,state,request_json,base_revision,result_revision,result_json,error_class,error_message"

func (o Operations) Create(ctx context.Context, request OperationRequest) (OperationRecord, error) {
	existing, err := o.ByIdempotency(ctx, request.Principal, request.IdempotencyKey)
	if err != nil {
		return OperationRecord{}, err
	}
	if existing != nil {
		if existing.RequestHash != request.RequestHash() {
			return OperationRecord{}, &IdempotencyConflictError{}
		}
		return *existing, nil
	}
	_, err = o.DB.ExecContext(ctx, `INSERT INTO operations(op_id,idempotency_key,principal,client_instance_id,mcp_session_id,source_chat,tool_name,request_hash,state,request_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, request.OpID, request.IdempotencyKey, request.Principal, request.ClientInstanceID, request.MCPSessionID, request.SourceChat, request.ToolName, request.RequestHash(), Queued, request.RequestJSON, o.now())
	if err != nil {
		return OperationRecord{}, err
	}
	return o.Get(ctx, request.OpID)
}
func (o Operations) Get(ctx context.Context, opID string) (OperationRecord, error) {
	record, err := scanOperation(o.DB.QueryRowContext(ctx, "SELECT "+operationColumns+" FROM operations WHERE op_id = ?", opID))
	if errors.Is(err, sql.ErrNoRows) {
		return OperationRecord{}, &OperationNotFoundError{OpID: opID}
	}
	return record, err
}
func (o Operations) ByIdempotency(ctx context.Context, principal, key string) (*OperationRecord, error) {
	record, err := scanOperation(o.DB.QueryRowContext(ctx, "SELECT "+operationColumns+" FROM operations WHERE principal = ? AND idempotency_key = ?", principal, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}
func (o Operations) Interrupted(ctx context.Context) ([]OperationRecord, error) {
	rows, err := o.DB.QueryContext(ctx, "SELECT "+operationColumns+" FROM operations WHERE state IN (?, ?, ?) ORDER BY created_at, op_id", Queued, Running, Recovering)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOperations(rows)
}

type operationRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanOperations(rows operationRows) ([]OperationRecord, error) {
	out := []OperationRecord{}
	for rows.Next() {
		record, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanOperation(row rowScanner) (OperationRecord, error) {
	var record OperationRecord
	if err := row.Scan(&record.OpID, &record.Principal, &record.IdempotencyKey, &record.ToolName, &record.RequestHash, &record.State, &record.RequestJSON, &record.BaseRevision, &record.ResultRevision, &record.ResultJSON, &record.ErrorClass, &record.ErrorMessage); err != nil {
		return OperationRecord{}, err
	}
	switch record.State {
	case Queued, Running, Succeeded, Failed, Conflict, Recovering:
		return record, nil
	default:
		return OperationRecord{}, fmt.Errorf("invalid operation state: %s", record.State)
	}
}
func (o Operations) MarkRunning(ctx context.Context, opID, base string) (OperationRecord, error) {
	return o.update(ctx, opID, "state=?, base_revision=?, started_at=?", Running, base, o.now())
}
func (o Operations) MarkSucceeded(ctx context.Context, opID, revision string, result map[string]any) (OperationRecord, error) {
	value, err := pyjson.Dumps(result)
	if err != nil {
		return OperationRecord{}, err
	}
	return o.update(ctx, opID, "state=?, result_revision=?, result_json=?, error_class=?, error_message=?, finished_at=?", Succeeded, revision, value, nil, nil, o.now())
}
func (o Operations) MarkConflict(ctx context.Context, opID, message string) (OperationRecord, error) {
	return o.update(ctx, opID, "state=?, error_class=?, error_message=?, finished_at=?", Conflict, "conflict", message, o.now())
}
func (o Operations) MarkFailed(ctx context.Context, opID, errorClass, message string) (OperationRecord, error) {
	return o.update(ctx, opID, "state=?, error_class=?, error_message=?, finished_at=?", Failed, errorClass, message, o.now())
}
func (o Operations) update(ctx context.Context, opID, assignments string, values ...any) (OperationRecord, error) {
	values = append(values, opID)
	if _, err := o.DB.ExecContext(ctx, "UPDATE operations SET "+assignments+" WHERE op_id=?", values...); err != nil {
		return OperationRecord{}, err
	}
	return o.Get(ctx, opID)
}
