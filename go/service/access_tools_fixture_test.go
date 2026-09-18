package service

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestAccessToolDefinitions(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/access-tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Count int
		Tools []map[string]any
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	var generated []map[string]any
	decoder = json.NewDecoder(bytes.NewReader(accessToolDefinitions))
	decoder.UseNumber()
	if err = decoder.Decode(&generated); err != nil {
		t.Fatal(err)
	}
	if fixture.Count != 10 || len(generated) != 10 || !reflect.DeepEqual(fixture.Tools, generated) {
		t.Fatal(fixture.Count, len(generated))
	}
	names := []string{}
	for _, tool := range generated {
		names = append(names, tool["name"].(string))
	}
	if names[9] != "access_principal_delete" {
		t.Fatal(names)
	}
}
