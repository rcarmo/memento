package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

func TestAuthenticatedSubmissionEnvelopeOverHTTP(t *testing.T) {
	ctx := context.Background()
	controls, _, base := realApplyTest(t)
	if _, err := controls.Queue.Proposals.DB.Exec("DELETE FROM proposals"); err != nil {
		t.Fatal(err)
	}
	config := access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{
		"proposer": {Roles: []string{"proposer"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}},
		"reader":   {Roles: []string{"reader"}, ReadPrefixes: []string{"/"}},
	}}
	identity, err := NewIdentity([]BearerPrincipal{{"proposer-token", access.Principal{Name: "proposer", Roles: []string{"proposer"}}}, {"reader-token", access.Principal{Name: "reader", Roles: []string{"reader"}}}}, config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("test-only-service-envelope")
	// This is test-only adapter wiring, not advertised production endpoints. Tool
	// schemas/full registry argument validation are a separate implementation.
	finish := func(data map[string]any, err error) (any, error) {
		if err != nil {
			failure, err := FailureEnvelope(err)
			if err != nil {
				return nil, err
			}
			return MCPEnvelope(failure)
		}
		success, err := controls.Queue.SuccessEnvelope(data, SuccessOptions{})
		if err != nil {
			return nil, err
		}
		return MCPEnvelope(success)
	}
	if err = server.Tools.Register(umcp.Tool{Name: "submit_test", Parameters: []umcp.Parameter{{Name: "intent", Types: []umcp.ParamType{umcp.StringParam}}}, Call: func(ctx context.Context, args map[string]any) (any, error) {
		actor, err := identity.Context(ctx)
		if err != nil {
			return nil, err
		}
		data, err := controls.Propose(ctx, actor, args["intent"].(string), base, []any{map[string]any{"kind": "create", "path": "/new.md", "concept_type": "concept", "title": "Title", "body": "public synthetic"}}, nil)
		return finish(data, err)
	}}); err != nil {
		t.Fatal(err)
	}
	if err = server.Tools.Register(umcp.Tool{Name: "failure_test", Call: func(context.Context, map[string]any) (any, error) { return finish(nil, io.ErrClosedPipe) }}); err != nil {
		t.Fatal(err)
	}
	options := umcp.DefaultHTTPOptions()
	options.AsyncReference = true
	transport, err := umcp.NewStreamableHTTP(server, options, identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	request := func(token, body string) (int, map[string]any) {
		t.Helper()
		req, err := http.NewRequest("POST", host.URL+"/mcp", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var parsed map[string]any
		if err = json.NewDecoder(response.Body).Decode(&parsed); err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, parsed
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"submit_test","arguments":{"intent":"synthetic"}}}`
	status, response := request("proposer-token", body)
	if status != 200 {
		t.Fatal(status, response)
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatal(response)
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["status"] != "success" || structured["repo_revision"] != base || structured["index_revision"] != base {
		t.Fatal(structured)
	}
	proposal := structured["data"].(map[string]any)["proposal"].(map[string]any)
	if proposal["author_principal"] != "proposer" {
		t.Fatal(proposal)
	}
	record, err := controls.Queue.Proposals.Get(ctx, proposal["proposal_id"].(string))
	if err != nil || record.AuthorPrincipal != "proposer" {
		t.Fatal(record, err)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	var fromText map[string]any
	if err = json.Unmarshal([]byte(text), &fromText); err != nil || fromText["status"] != "success" {
		t.Fatal(text, err)
	}
	_, response = request("reader-token", body)
	result = response["result"].(map[string]any)
	structured = result["structuredContent"].(map[string]any)
	if structured["status"] != "error" || structured["error_class"] != "forbidden" || structured["repo_revision"] != nil {
		t.Fatal(response)
	}
	// An unrecognised failure is a JSON-RPC failure, not a service data envelope,
	// and network output must not expose the underlying error text.
	_, response = request("proposer-token", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"failure_test","arguments":{}}}`)
	failure, ok := response["error"].(map[string]any)
	if !ok || failure["message"] != "Tool execution failed" {
		t.Fatal(response)
	}
}
