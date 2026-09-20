package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/umcp"
)

func TestStagingHTTPReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/staging-http.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		ZIP    string `json:"zip"`
		Ticket map[string]any
		Cases  []struct {
			Method, Path, Body string
			Headers            map[string]string
			Advance            int
			AuthCalls          int `json:"auth_calls"`
			Expected           *struct {
				Status      int
				ContentType string `json:"content_type"`
				Headers     [][2]string
				Body        map[string]any
			}
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	q, _ := queueTest(t)
	blob, err := base64.StdEncoding.DecodeString(fixture.ZIP)
	if err != nil {
		t.Fatal(err)
	}
	ticket := fixture.Ticket
	if _, err = q.Proposals.DB.Exec("INSERT INTO asset_upload_tickets(token_digest,principal,idempotency_key,asset_kind,version,expires_at,staged_asset_id,consumed_at,created_at) VALUES(?,?,?,?,?,?,?,?,?)", ticket["token_digest"], ticket["principal"], ticket["idempotency_key"], ticket["asset_kind"], ticket["version"], ticket["expires_at"], ticket["staged_asset_id"], ticket["consumed_at"], ticket["created_at"]); err != nil {
		t.Fatal(err)
	}
	clock := q.Now()
	random := make([]byte, 160)
	for i := range 10 {
		random[i*16+15] = byte(i)
	}
	store := &assets.StagingStore{DB: q.Proposals.DB, Now: func() time.Time { return clock }, Random: bytes.NewReader(random)}
	calls := 0
	handler := StagingHTTP{Store: store, Authenticate: func(_ context.Context, headers map[string]string) (*access.Principal, error) {
		calls++
		name := headers["authorization"]
		if name != "actor" && name != "other" && name != "reader" {
			return nil, nil
		}
		role := "proposer"
		if name == "reader" {
			role = "reader"
		}
		return &access.Principal{Name: name, Roles: []string{role}}, nil
	}}
	for _, c := range fixture.Cases {
		clock = clock.Add(time.Duration(c.Advance) * time.Hour)
		body := []byte{}
		if c.Body == "zip" {
			body = blob
		} else if c.Body == "bad" {
			body = []byte("bad")
		}
		before := calls
		got, err := handler.Handle(ctx, c.Method, c.Path, c.Headers, body, "")
		if err != nil {
			t.Fatal(c, err)
		}
		if calls-before != c.AuthCalls {
			t.Fatal("auth order", c, calls-before)
		}
		if c.Expected == nil {
			if got != nil {
				t.Fatal(got)
			}
			continue
		}
		var payload any
		if err = json.Unmarshal(got.Body, &payload); err != nil {
			t.Fatal(err)
		}
		if got.Status != c.Expected.Status || *got.ContentType != c.Expected.ContentType || !reflect.DeepEqual(got.Headers, c.Expected.Headers) || !reflect.DeepEqual(payload, c.Expected.Body) {
			t.Fatal(c, got, payload, c.Expected)
		}
	}
}
func TestStagingHTTPFailuresAndTransport(t *testing.T) {
	ctx := context.Background()
	q, _ := queueTest(t)
	store := &assets.StagingStore{DB: q.Proposals.DB, Now: q.Now}
	handler := StagingHTTP{Store: store, Authenticate: func(context.Context, map[string]string) (*access.Principal, error) { return nil, io.ErrClosedPipe }}
	if _, err := handler.Handle(ctx, "GET", "http://[invalid/assets/staging", nil, nil, ""); err == nil {
		t.Fatal("bad URL")
	}
	if _, err := handler.Handle(ctx, "GET", "/assets/staging/a", nil, nil, ""); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if _, err := stagingJSON(map[string]any{"bad": make(chan int)}, 200); err == nil {
		t.Fatal("invalid JSON")
	}
	handler.Authenticate = func(context.Context, map[string]string) (*access.Principal, error) {
		return &access.Principal{Name: "actor", Roles: []string{"proposer"}}, nil
	}
	tooBig := make([]byte, assets.MaxZIPBytes+1)
	for _, path := range []string{"/assets/staging", "/assets/staging/upload"} {
		got, err := handler.Handle(ctx, "POST", path, map[string]string{"content-type": "application/zip"}, tooBig, "")
		if err != nil || got.Status != 413 {
			t.Fatal(got, err)
		}
	}
	server := umcp.NewServer("staging-route")
	options := umcp.DefaultHTTPOptions()
	options.MaxRequestBytes = assets.MaxZIPBytes + 1024
	transport, err := umcp.NewStreamableHTTP(server, options, umcp.HTTPHooks{Authenticate: func(context.Context, string, string, map[string]string, string) (*umcp.Principal, error) {
		t.Error("transport auth must not own auxiliary staging routes")
		return nil, nil
	}, Route: handler.Handle})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	_, token, err := store.BeginUpload(ctx, "actor", "ticket", "docs", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	blob := submitZIP(t)
	request := httptest.NewRequest("POST", "http://localhost/assets/staging/upload", bytes.NewReader(blob))
	request.Header.Set("Content-Type", "application/zip")
	request.Header.Set("X-Memento-Upload-Ticket", token)
	response := httptest.NewRecorder()
	transport.ServeHTTP(response, request)
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(response.Header())
	}
	q.Proposals.DB.Close()
	if _, err := handler.Handle(ctx, "GET", "/assets/staging/a", nil, nil, ""); err == nil {
		t.Fatal("DB error swallowed")
	}
	if !pythonHTTPWhitespace('\x1c') || pythonHTTPWhitespace('x') {
		t.Fatal("whitespace")
	}
}
