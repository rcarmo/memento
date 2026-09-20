package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/umcp"
)

func TestStagingToolDispatchReference(t *testing.T) {
	testToolReference(t, "staging-tool-dispatch.json", stagingToolDefinitions)
}
func TestStagingToolReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/staging-tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		ZIP   string `json:"zip"`
		Cases []struct {
			Action, Principal, Key, Version string
			Kind                            string `json:"asset_kind"`
			Roles                           []string
			Expected                        map[string]any
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	q, _ := queueTest(t)
	clock := q.Now()
	store := &assets.StagingStore{DB: q.Proposals.DB, Now: func() time.Time { return clock }, Random: bytes.NewReader(append(make([]byte, 48), bytes.Repeat([]byte{1}, 32)...))}
	blob, err := base64.StdEncoding.DecodeString(fixture.ZIP)
	if err != nil {
		t.Fatal(err)
	}
	token := ""
	for _, c := range fixture.Cases {
		if c.Action == "upload" {
			if _, _, err := store.PutWithTicket(ctx, token, blob); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if c.Action == "expire" {
			clock = clock.Add(25 * time.Hour)
			continue
		}
		active := store
		if c.Action == "unavailable" || c.Action == "unavailable_status" {
			active = nil
		}
		name := "memory_asset_stage_begin"
		if c.Action == "status" || c.Action == "unavailable_status" {
			name = "memory_asset_stage_status"
		}
		data, options, err := stageTool(ctx, active, access.Principal{Name: c.Principal, Roles: c.Roles}, name, map[string]any{"idempotency_key": c.Key, "asset_kind": c.Kind, "version": c.Version})
		var got any
		if err != nil {
			got, err = FailureEnvelope(err)
		} else {
			if value, ok := data["upload_ticket"].(string); ok {
				token = value
			}
			got, err = q.successEnvelope(data, options, fakeRepo(nil))
		}
		if err != nil || !reflect.DeepEqual(jsonNormal(got), c.Expected) {
			t.Fatal(c.Action, c.Key, got, c.Expected, err)
		}
	}
}
func TestStagingToolFailurePaths(t *testing.T) {
	ctx := context.Background()
	q, _ := queueTest(t)
	store := &assets.StagingStore{DB: q.Proposals.DB, Now: q.Now}
	principal := access.Principal{Name: "actor", Roles: []string{"proposer"}}
	for _, name := range []string{"memory_asset_stage_begin", "memory_asset_stage_status"} {
		if _, _, err := stageTool(ctx, store, principal, name, map[string]any{"idempotency_key": 1}); err == nil {
			t.Fatal(name)
		}
	}
	_, _, err := store.BeginUpload(ctx, "actor", "key", "docs", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.Proposals.DB.Exec("UPDATE asset_upload_tickets SET expires_at='invalid'"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = stageTool(ctx, store, principal, "memory_asset_stage_status", map[string]any{"idempotency_key": "key"}); err == nil {
		t.Fatal("invalid timestamp")
	}
	if _, err = q.Proposals.DB.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	if _, err = q.Proposals.DB.Exec("UPDATE asset_upload_tickets SET expires_at='2099-01-01T00:00:00Z',staged_asset_id='missing'"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = stageTool(ctx, store, principal, "memory_asset_stage_status", map[string]any{"idempotency_key": "key"}); err == nil {
		t.Fatal("missing staged")
	}
	store.Now = nil
	if _, err = q.Proposals.DB.Exec("UPDATE asset_upload_tickets SET staged_asset_id=NULL"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = stageTool(ctx, store, principal, "memory_asset_stage_status", map[string]any{"idempotency_key": "key"}); err != nil {
		t.Fatal(err)
	}
}

// Direct Python wrappers cannot interleave their synchronous begin/status DB
// sequences. Concurrent Go requests must not leak a UNIQUE-constraint failure.
func TestStagingDirectConcurrentBegin(t *testing.T) {
	j, _ := jobsTest(t)
	j.Controls.Staging = &assets.StagingStore{DB: j.Controls.Queue.Proposals.DB, Now: j.Controls.Queue.Now}
	j.Workers.SetExecuteBusy(true)
	start := make(chan struct{})
	type outcome struct {
		value any
		err   error
	}
	results := make(chan outcome, 16)
	d := umcp.Dispatcher{Handlers: map[string]umcp.Handler{"begin": func(ctx context.Context, _ map[string]any) (any, *umcp.RPCError, error) {
		v, err := j.callStagingOrProposalTool(ctx, "memory_asset_stage_begin", map[string]any{"idempotency_key": "one", "asset_kind": "docs", "version": "1.0.0"})
		return v, nil, err
	}}}
	for range 16 {
		go func() {
			<-start
			v, err := d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"begin"}`), umcp.RequestContext{Principal: "actor"})
			results <- outcome{v, err}
		}()
	}
	close(start)
	successes := 0
	for range 16 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		response := jsonNormal(result.value).(map[string]any)
		if response["error"] != nil {
			t.Fatal(response)
		}
		value := response["result"].(map[string]any)
		if value["status"] == "success" {
			successes++
		} else if value["error_class"] != "validation_error" || value["message"] != "upload ticket already issued: pending" {
			t.Fatal(value)
		}
	}
	if successes != 1 {
		t.Fatal("ticket issuances", successes)
	}
	var count int
	if err := j.Controls.Queue.Proposals.DB.QueryRow("SELECT count(*) FROM asset_upload_tickets").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
