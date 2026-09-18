package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rcarmo/memento/go/internal/pyjson"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func newOperations(t *testing.T) (Operations, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sqlite")
	db, err := Connect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return Operations{DB: db, Now: func() time.Time { return time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC) }}, path
}
func TestOperationPythonReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/control-operations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Input struct {
				Action, Base, Message, Revision string
				OpID                            string `json:"op_id"`
				ErrorClass                      string `json:"error_class"`
				Request                         OperationRequest
				Result                          json.RawMessage
			}
			Record  OperationRecord
			Records []OperationRecord
			Error   string
		}
		Rows []map[string]any
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	operations, path := newOperations(t)
	ctx := context.Background()
	for i, c := range fixture.Cases {
		var got OperationRecord
		var err error
		switch c.Input.Action {
		case "create":
			got, err = operations.Create(ctx, c.Input.Request)
		case "get":
			got, err = operations.Get(ctx, c.Input.OpID)
		case "running":
			got, err = operations.MarkRunning(ctx, c.Input.OpID, c.Input.Base)
		case "failed":
			got, err = operations.MarkFailed(ctx, c.Input.OpID, c.Input.ErrorClass, c.Input.Message)
		case "conflict":
			got, err = operations.MarkConflict(ctx, c.Input.OpID, c.Input.Message)
		case "succeeded":
			result, parseErr := pyjson.Parse(string(c.Input.Result))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			got, err = operations.MarkSucceeded(ctx, c.Input.OpID, c.Input.Revision, result.(map[string]any))
		case "interrupted":
			records, e := operations.Interrupted(ctx)
			if e != nil || !reflect.DeepEqual(records, c.Records) {
				t.Fatal(records, c.Records, e)
			}
			continue
		}
		if c.Error != "" {
			if err == nil {
				t.Fatal(i, "missing error")
			}
			if c.Error == "IdempotencyConflictError" {
				var conflict *IdempotencyConflictError
				if !errors.As(err, &conflict) || err.Error() != "idempotency key already used for a different request" {
					t.Fatal(err)
				}
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, c.Record) {
			t.Fatalf("case %d: %+v != %+v err %v", i, got, c.Record, err)
		}
	}
	operations.DB.Close()
	db, err := Connect(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT * FROM operations ORDER BY op_id")
	if err != nil {
		t.Fatal(err)
	}
	values, err := readRows(rows)
	if err != nil || !reflect.DeepEqual(values, fixture.Rows) {
		t.Fatal(values, fixture.Rows, err)
	}
}
func TestOperationErrorsAndReplay(t *testing.T) {
	operations, _ := newOperations(t)
	ctx := context.Background()
	record, err := operations.Create(ctx, OperationRequest{OpID: "one", Principal: "p", IdempotencyKey: "k", ToolName: "write", RequestJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if payload, err := record.ReplayPayload(); payload != nil || err != nil {
		t.Fatal(payload, err)
	}
	for _, raw := range []string{"[]", "{}", "{", `{"large":123456789012345678901234}`} {
		record.ResultJSON = &raw
		payload, err := record.ReplayPayload()
		if raw == "{" {
			if err == nil {
				t.Fatal("bad result")
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if raw == "[]" && payload != nil {
			t.Fatal(payload)
		}
	}
	if _, err = operations.MarkSucceeded(ctx, "one", "result", map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("invalid JSON")
	}
	if _, err = operations.Get(ctx, "missing"); err == nil || err.Error() != "unknown operation: missing" {
		t.Fatal(err)
	}
	if found, err := operations.ByIdempotency(ctx, "p", "k"); err != nil || found == nil {
		t.Fatal(err)
	}
	_, _ = operations.DB.Exec("UPDATE operations SET state='invalid' WHERE op_id='one'")
	if _, err = operations.Get(ctx, "one"); err == nil {
		t.Fatal("invalid state")
	}
	if _, err = operations.Create(ctx, OperationRequest{Principal: "p", IdempotencyKey: "k"}); err == nil {
		t.Fatal("bad row lookup")
	}
	if _, err = scanOperations(&failedRows{fail: "scan"}); err == nil {
		t.Fatal("scan failure")
	}
	if stamp := (Operations{}).now(); len(stamp) != 20 {
		t.Fatal(stamp)
	}
	operations.DB.Close()
	if _, err = operations.ByIdempotency(ctx, "p", "k"); err == nil {
		t.Fatal("closed lookup")
	}
	if _, err = operations.MarkRunning(ctx, "one", "base"); err == nil {
		t.Fatal("closed update")
	}
	if _, err = operations.Interrupted(ctx); err == nil {
		t.Fatal("closed list")
	}
}
func TestOperationConcurrentUniquenessAndReopen(t *testing.T) {
	operations, path := newOperations(t)
	other, err := Connect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	stores := []Operations{operations, {DB: other}}
	var workers sync.WaitGroup
	records := make(chan OperationRecord, 20)
	for i := range 20 {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			record, err := stores[i%2].Create(context.Background(), OperationRequest{OpID: fmt.Sprint("op-", i), Principal: "p", IdempotencyKey: "same", ToolName: "write", RequestJSON: "{}"})
			if err == nil {
				records <- record
			}
		}(i)
	}
	workers.Wait()
	close(records)
	id := ""
	for record := range records {
		if id != "" && id != record.OpID {
			t.Fatal("duplicate identities")
		}
		id = record.OpID
	}
	if id == "" {
		t.Fatal("no winner")
	}
	var count int
	if err = other.QueryRow("SELECT COUNT(*) FROM operations").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
