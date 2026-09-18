package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rcarmo/memento/go/umcp"
)

func testCatalogHandlers(c *Catalog) map[string]CatalogHandler {
	handlers := map[string]CatalogHandler{}
	for _, op := range c.source.Operations {
		handlers[op.Tool] = func(context.Context, map[string]any) (any, error) { return map[string]any{"test_only": true}, nil }
	}
	handlers["memory_help"] = func(context.Context, map[string]any) (any, error) {
		return (ProposalQueue{}).successEnvelope(c.Help(), SuccessOptions{}, fakeRepo(nil))
	}
	handlers["memory_status"] = func(context.Context, map[string]any) (any, error) { return map[string]any{"test_only": "status"}, nil }
	return handlers
}
func TestCatalogResourceProtocolReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Protocol []struct {
			Request  string
			Expected map[string]any
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	c, err := NewCatalog(CatalogConfig{Surface: "read_only", AnswerEnabled: true, RouteEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("resources")
	server.SetNotificationOutput(nil)
	if err = c.Register(server, testCatalogHandlers(c), nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixture.Protocol {
		response, err := server.Process(context.Background(), []byte(tc.Request), umcp.RequestContext{Principal: "actor", Transport: "streamable-http"})
		if err != nil {
			t.Fatal(err)
		}
		got := jsonNormal(response).(map[string]any)
		// Resource text is source sorted JSON. Object-key order/spacing do not
		// change its contract; all payload values, URI/metadata/errors stay exact.
		normalise := func(v map[string]any) {
			if result, ok := v["result"].(map[string]any); ok {
				if contents, ok := result["contents"].([]any); ok {
					for _, raw := range contents {
						item := raw.(map[string]any)
						if item["mimeType"] == "application/json" {
							var value any
							if err := json.Unmarshal([]byte(item["text"].(string)), &value); err != nil {
								t.Fatal(err)
							}
							item["text"] = value
						}
					}
				}
			}
		}
		normalise(got)
		normalise(tc.Expected)
		if !reflect.DeepEqual(got, tc.Expected) {
			t.Fatal(tc.Request, got, tc.Expected)
		}
	}
}
func TestCatalogResourcesHTTPAndHandlerOwnership(t *testing.T) {
	j, _ := jobsTest(t)
	c, _ := NewCatalog(CatalogConfig{Surface: "read_only"})
	handlers := testCatalogHandlers(c)
	// Help/status callbacks, unlike pure contract resources, own live policy.
	handlers["memory_help"] = func(ctx context.Context, _ map[string]any) (any, error) {
		if _, err := j.Identity.Context(ctx); err != nil {
			return nil, err
		}
		return (ProposalQueue{}).successEnvelope(c.Help(), SuccessOptions{}, fakeRepo(nil))
	}
	server := umcp.NewServer("catalog-http")
	server.SetNotificationOutput(nil)
	if err := c.Register(server, handlers, nil); err != nil {
		t.Fatal(err)
	}
	handlers["memory_help"] = func(context.Context, map[string]any) (any, error) { return nil, io.ErrClosedPipe } // must not swap registered callback
	opts := umcp.DefaultHTTPOptions()
	opts.AsyncReference = true
	http, err := umcp.NewStreamableHTTP(server, opts, j.Identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer http.Close()
	request := func(uri, token string) (int, map[string]any) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "resources/read", "params": map[string]any{"uri": uri}})
		req := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		http.ServeHTTP(rec, req)
		var value map[string]any
		if rec.Body.Len() == 0 {
			return rec.Code, nil
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		return rec.Code, value
	}
	if status, _ := request("memory://catalog", ""); status != 401 {
		t.Fatal(status)
	}
	for _, uri := range []string{"memory://catalog", "memory://help", "memory://status", "memory://catalog/propose", "memory://workflow/asset_pack"} {
		status, value := request(uri, "token")
		if status != 200 || value["error"] != nil {
			t.Fatal(status, value)
		}
	}
	// Handler failures remain transport-redacted, never fake success resources.
	failing := testCatalogHandlers(c)
	failing["memory_status"] = func(context.Context, map[string]any) (any, error) { return nil, io.ErrClosedPipe }
	other := umcp.NewServer("failure")
	other.SetNotificationOutput(nil)
	if err = c.Register(other, failing, nil); err != nil {
		t.Fatal(err)
	}
	response, err := other.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"memory://status"}}`), umcp.RequestContext{Transport: "streamable-http", Principal: "actor"})
	if err != nil || response.Error == nil {
		t.Fatal(response, err)
	}
	// Python discovery hiding is not dispatch authorisation: contracts and
	// calls remain addressable, and their handlers must enforce permissions.
	hidden, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_propose","arguments":{"intent":"test","base_revision":"main","changes":[]}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || hidden.Error != nil {
		t.Fatal(hidden, err)
	}
	// Full tool calls also retain captured callback identity.
	result, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_help"}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || result.Error != nil {
		t.Fatal(result, err)
	}
}
