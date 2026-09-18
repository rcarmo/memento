package service

import (
	"context"
	"encoding/json"
	"github.com/rcarmo/memento/go/umcp"
	"testing"
)

func TestModelsOffServer(t *testing.T) {
	j, _ := jobsTest(t)
	server := umcp.NewServer("models-off")
	server.SetNotificationOutput(nil)
	if err := j.RegisterModelsOffServer(server, "standard", endpointLimits(), nil); err != nil {
		t.Fatal(err)
	}
	list, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || list.Error != nil {
		t.Fatal(list, err)
	}
	tools := jsonNormal(list.Result).(map[string]any)["tools"].([]any)
	found := false
	for _, value := range tools {
		if value.(map[string]any)["name"] == "memory_execute" {
			found = true
		}
	}
	if !found {
		t.Fatal("execute undiscoverable")
	}
	call, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"memory_execute","arguments":{"operations":[]}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || call.Error != nil {
		t.Fatal(call, err)
	}
}
func TestModelsOffServerGuards(t *testing.T) {
	j, _ := jobsTest(t)
	if err := j.RegisterModelsOffServer(nil, "standard", endpointLimits(), nil); err == nil {
		t.Fatal("server")
	}
	if err := j.RegisterModelsOffServer(umcp.NewServer("x"), "bad", endpointLimits(), nil); err == nil {
		t.Fatal("surface")
	}
	bad := &Jobs{Controls: j.Controls, DBPath: j.DBPath}
	if err := bad.RegisterModelsOffServer(umcp.NewServer("x"), "standard", endpointLimits(), nil); err == nil {
		t.Fatal("endpoint")
	}
	old := auditToolDefinitions
	auditToolDefinitions = []byte("{")
	if err := j.RegisterModelsOffServer(umcp.NewServer("x"), "standard", endpointLimits(), nil); err == nil {
		t.Fatal("tools")
	}
	auditToolDefinitions = old
	_ = json.Number("0")
}
