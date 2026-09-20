package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

func adminFixture(t *testing.T) (context.Context, *access.Store, string, AdminHTTP) {
	t.Helper()
	ctx := context.Background()
	q, _ := queueTest(t)
	store, err := access.OpenStore(ctx, q.Proposals.DB, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := store.Create(ctx, "bootstrap", "root", []string{"admin"}, []string{"/"}, []string{"/"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, store, token, AdminHTTP{Store: store, ProtectedReadPrefixes: []string{"/private/"}}
}

func adminCall(t *testing.T, h AdminHTTP, method, path, token string, payload any) (*umcp.HTTPResponse, map[string]any, error) {
	t.Helper()
	var body []byte
	switch value := payload.(type) {
	case nil:
	case []byte:
		body = value
	default:
		var err error
		body, err = json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	headers := map[string]string{}
	if token != "" {
		headers["authorization"] = "Bearer " + token
	}
	response, err := h.Handle(context.Background(), method, path, headers, body, "")
	if err != nil || response == nil || response.ContentType == nil || !strings.HasPrefix(*response.ContentType, "application/json") || len(response.Body) == 0 {
		return response, nil, err
	}
	var parsed map[string]any
	if decodeErr := json.Unmarshal(response.Body, &parsed); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	return response, parsed, err
}

func adminTransportRequest(t *testing.T, transport *umcp.StreamableHTTP, method, path, token string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	transport.ServeHTTP(response, request)
	return response
}

func TestAdminHTTPHandleCRUDAndActivity(t *testing.T) {
	ctx, store, adminToken, handler := adminFixture(t)
	if response, err := handler.Handle(ctx, "GET", "/outside", nil, nil, ""); err != nil || response != nil {
		t.Fatal(response, err)
	}
	_, readerToken, err := store.Create(ctx, "bootstrap", "reader", []string{"reader"}, []string{"/public/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, payload, err := adminCall(t, handler, "GET", "/admin/api/principals", readerToken, nil)
	if err != nil || response.Status != 401 || payload["error"] != "admin bearer credential required" || !reflect.DeepEqual(response.Headers, adminHeaders) {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals", adminToken, map[string]any{"name": "worker", "roles": []string{"reader"}, "read_prefixes": []string{"/"}, "write_prefixes": []string{}})
	if err != nil || response.Status != 201 || !reflect.DeepEqual(response.Headers, adminHeaders) {
		t.Fatal(response, payload, err)
	}
	principal := payload["principal"].(map[string]any)
	if principal["name"] != "worker" || principal["enabled"] != true || principal["revoked"] != false || principal["deleted"] != false {
		t.Fatal(principal)
	}
	warnings := principal["warnings"].([]any)
	if len(warnings) != 1 || warnings[0] != "broad '/' read grant excludes protected namespaces; add explicit read prefixes where access is intended" {
		t.Fatal(warnings)
	}
	credential := payload["credential"].(string)
	worker, err := store.Authenticate(ctx, credential)
	if err != nil || worker == nil || worker.Name != "worker" {
		t.Fatal(worker, err)
	}
	response, payload, err = adminCall(t, handler, "GET", "/admin/api/principals", adminToken, nil)
	if err != nil || response.Status != 200 || !reflect.DeepEqual(response.Headers, adminHeaders) {
		t.Fatal(response, payload, err)
	}
	principals := payload["principals"].([]any)
	if len(principals) != 3 {
		t.Fatal(principals)
	}
	foundWarning, foundRoot := false, false
	for _, raw := range principals {
		item := raw.(map[string]any)
		switch item["name"] {
		case "root":
			foundRoot = true
			if item["warnings"] != nil {
				t.Fatal(item)
			}
		case "worker":
			foundWarning = len(item["warnings"].([]any)) == 1
		}
	}
	if !foundRoot || !foundWarning {
		t.Fatal(principals)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/worker/update", adminToken, map[string]any{"roles": []string{"reader", "proposer"}, "read_prefixes": []string{"/private/", "/work/"}, "write_prefixes": []string{"/work/"}})
	if err != nil || response.Status != 200 || payload["principal"].(map[string]any)["name"] != "worker" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/worker/rename", adminToken, map[string]any{"new_name": "worker-two"})
	if err != nil || response.Status != 200 || payload["principal"].(map[string]any)["name"] != "worker-two" {
		t.Fatal(response, payload, err)
	}
	worker, err = store.Authenticate(ctx, credential)
	if err != nil || worker == nil || worker.Name != "worker-two" {
		t.Fatal(worker, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/worker-two/disable", adminToken, map[string]any{})
	if err != nil || response.Status != 200 || payload["principal"].(map[string]any)["enabled"] != false {
		t.Fatal(response, payload, err)
	}
	if worker, err = store.Authenticate(ctx, credential); err != nil || worker != nil {
		t.Fatal(worker, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/worker-two/enable", adminToken, map[string]any{})
	if err != nil || response.Status != 200 || payload["principal"].(map[string]any)["enabled"] != true {
		t.Fatal(response, payload, err)
	}
	if worker, err = store.Authenticate(ctx, credential); err != nil || worker == nil || worker.Name != "worker-two" {
		t.Fatal(worker, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/worker-two/rotate", adminToken, map[string]any{})
	if err != nil || response.Status != 200 || payload["name"] != "worker-two" {
		t.Fatal(response, payload, err)
	}
	rotated := payload["credential"].(string)
	if worker, err = store.Authenticate(ctx, credential); err != nil || worker != nil {
		t.Fatal(worker, err)
	}
	if worker, err = store.Authenticate(ctx, rotated); err != nil || worker == nil || worker.Name != "worker-two" {
		t.Fatal(worker, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/worker-two/revoke", adminToken, map[string]any{})
	if err != nil || response.Status != 200 {
		t.Fatal(response, payload, err)
	}
	revoked := payload["principal"].(map[string]any)
	if revoked["enabled"] != false || revoked["revoked"] != true {
		t.Fatal(revoked)
	}
	if worker, err = store.Authenticate(ctx, rotated); err != nil || worker != nil {
		t.Fatal(worker, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/worker-two/delete", adminToken, map[string]any{})
	if err != nil || response.Status != 200 || payload["principal"].(map[string]any)["deleted"] != true {
		t.Fatal(response, payload, err)
	}
	for i := 0; i < 55; i++ {
		if _, _, err = store.Create(ctx, "root", fmt.Sprintf("audit-%02d", i), []string{"reader"}, []string{"/public/"}, nil, nil); err != nil {
			t.Fatal(i, err)
		}
	}
	response, payload, err = adminCall(t, handler, "GET", "/admin/api/activity", adminToken, nil)
	if err != nil || response.Status != 200 || !reflect.DeepEqual(response.Headers, adminHeaders) {
		t.Fatal(response, payload, err)
	}
	events := payload["events"].([]any)
	if len(events) != 50 {
		t.Fatal(len(events), events)
	}
	if events[0].(map[string]any)["target"] != "audit-54" || events[49].(map[string]any)["target"] != "audit-05" {
		t.Fatal(events[0], events[49])
	}
}

func TestAdminHTTPThroughUMCP(t *testing.T) {
	ctx, store, adminToken, handler := adminFixture(t)
	if _, _, err := store.Create(ctx, "bootstrap", "through-http", []string{"reader"}, []string{"/public/"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("admin")
	authenticated := false
	transport, err := umcp.NewStreamableHTTP(server, umcp.DefaultHTTPOptions(), umcp.HTTPHooks{Authenticate: func(context.Context, string, string, map[string]string, string) (*umcp.Principal, error) {
		authenticated = true
		return nil, nil
	}, Route: handler.Handle})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	response := adminTransportRequest(t, transport, "GET", "/admin", "", nil, nil)
	if response.Code != 200 || authenticated || response.Header().Get("Content-Type") != "text/html; charset=utf-8" || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Contains(response.Body.Bytes(), []byte("Memento Access")) {
		t.Fatal(response.Code, authenticated, response.Header(), response.Body.String())
	}
	response = adminTransportRequest(t, transport, "GET", "/admin/app.js", "", nil, nil)
	if response.Code != 200 || authenticated || response.Header().Get("Content-Type") != "text/javascript; charset=utf-8" || !bytes.Contains(response.Body.Bytes(), []byte("/admin/api/")) {
		t.Fatal(response.Code, authenticated, response.Header(), response.Body.String())
	}
	response = adminTransportRequest(t, transport, "GET", "/admin/api/principals", "", nil, nil)
	if response.Code != 401 || authenticated {
		t.Fatal(response.Code, authenticated, response.Body.String())
	}
	var payload map[string]any
	if err = json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "admin bearer credential required" {
		t.Fatal(payload)
	}
	response = adminTransportRequest(t, transport, "GET", "/admin/api/principals", adminToken, nil, nil)
	if response.Code != 200 || authenticated || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(response.Code, authenticated, response.Header(), response.Body.String())
	}
	if err = json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload["principals"].([]any)) != 2 {
		t.Fatal(payload)
	}
}
