package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRuntimeBaseDefaults(t *testing.T) {
	l := defaultLimitsConfig()
	m := defaultMCPConfig()
	if l.MaxConceptBytes != 262144 || l.MaxSearchResults != 100 || m.ToolSurface != "compact" || !m.CompactAnswerEnabled || m.MaxRequestBytes != 72*1024*1024 {
		t.Fatal(l, m)
	}
	got := m.ExecuteLimits()
	if got.MaxOperations != 12 || got.MaxIntermediates != 12 || got.MaxRecords != 50 || got.MaxOutputBytes != "65536" || got.MaxTimeSeconds != 3 {
		t.Fatal(got)
	}
	m.AllowedOrigins = []string{" https://b.example ", "http://a.example", "http://a.example"}
	if !reflect.DeepEqual(m.NormalizedOrigins(), []string{"http://a.example", "https://b.example"}) {
		t.Fatal(m.NormalizedOrigins())
	}
}
func TestMCPConfigValidation(t *testing.T) {
	base := defaultMCPConfig()
	for i, mutate := range []func(*MCPConfig){func(c *MCPConfig) { c.ToolSurface = "bad" }, func(c *MCPConfig) { c.MaxRequestBytes = 1 }, func(c *MCPConfig) { c.Execute.MaxOperations = 0 }, func(c *MCPConfig) { c.Execute.MaxIntermediates = 65 }, func(c *MCPConfig) { c.Execute.MaxRecords = 501 }, func(c *MCPConfig) { c.Execute.MaxOutputBytes = 511 }, func(c *MCPConfig) { c.Execute.MaxTimeSeconds = 31 }, func(c *MCPConfig) { c.AllowedOrigins = []string{"ftp://x"} }, func(c *MCPConfig) { c.AllowedOrigins = []string{"https://x/path"} }, func(c *MCPConfig) { c.AllowedOrigins = []string{"https://x?q=1"} }, func(c *MCPConfig) { c.AllowedOrigins = []string{"https://x#f"} }} {
		c := base
		mutate(&c)
		if c.Validate() == nil {
			t.Fatal(i, c)
		}
	}
	for _, surface := range []string{"compact", "standard", "read_only", "curator", "admin"} {
		c := base
		c.ToolSurface = surface
		if err := c.Validate(); err != nil {
			t.Fatal(surface, err)
		}
	}
}
func TestIntelligentTiersModelsOff(t *testing.T) {
	var c IntelligentTiersConfig
	if err := c.ValidateModelsOff(); err != nil {
		t.Fatal(err)
	}
	disabled := []byte(`{"enabled":false,"limits":{"x":1}}`)
	c = IntelligentTiersConfig{DeepAnswers: disabled, ExactAnswerCache: disabled, HotWorkingMemory: disabled, ModelProposals: disabled, Dream: []byte(`{"mode":"disabled","scanner":{}}`), SemanticSearch: disabled, NeedleRouter: disabled, ModelProviderSlots: []byte(`{"hot_query":{}}`)}
	if err := c.ValidateModelsOff(); err != nil {
		t.Fatal(err)
	}
	for i, field := range []string{"deep_answers", "exact_answer_cache", "hot_working_memory", "model_proposals", "semantic_search", "needle_router"} {
		raw := map[string]json.RawMessage{field: []byte(`{"enabled":true}`)}
		encoded, _ := json.Marshal(raw)
		var active IntelligentTiersConfig
		_ = json.Unmarshal(encoded, &active)
		if active.ValidateModelsOff() == nil {
			t.Fatal(i, field)
		}
	}
	c = IntelligentTiersConfig{Dream: []byte(`{"mode":"propose"}`)}
	if c.ValidateModelsOff() == nil {
		t.Fatal("dream")
	}
	c = IntelligentTiersConfig{DeepAnswers: []byte(`{"enabled":"yes"}`)}
	if c.ValidateModelsOff() == nil {
		t.Fatal("type")
	}
	for _, raw := range []json.RawMessage{[]byte(`{}`), []byte(`null`)} {
		c = IntelligentTiersConfig{DeepAnswers: raw}
		if err := c.ValidateModelsOff(); err != nil {
			t.Fatal(err)
		}
	}
	c = IntelligentTiersConfig{Dream: []byte(`{"mode":1}`)}
	if c.ValidateModelsOff() == nil {
		t.Fatal("mode type")
	}
	c = IntelligentTiersConfig{DeepAnswers: []byte(`bad`)}
	if c.ValidateModelsOff() == nil {
		t.Fatal("object type")
	}
}
func TestRuntimeTopLevelValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cases := []string{`{"schema_version":1,"repository":{"root_path":"x"}}`, `{"schema_version":2,"repository":{"root_path":"x","bundle_root":"bad"}}`, `{"schema_version":2,"repository":{"root_path":"x","bundle_root":"/"},"limits":{"max_concept_bytes":0,"max_search_results":1}}`, `{"schema_version":2,"repository":{"root_path":"x","bundle_root":"/"},"mcp":{"tool_surface":"bad"}}`, `{"schema_version":2,"repository":{"root_path":"x","bundle_root":"/"},"intelligent_tiers":{"semantic_search":{"enabled":true}}}`}
	for _, raw := range cases {
		_ = os.WriteFile(path, []byte(raw), 0600)
		if _, err := LoadRuntimeConfig(path); err == nil {
			t.Fatal(raw)
		}
	}
}

func TestFullModelsOffConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"schema_version":2,"repository":{"root_path":"/tmp/runtime","bundle_root":"/"},"authorization":{"principals":{},"protected_read_prefixes":[]},"limits":{"max_concept_bytes":123,"max_search_results":4},"mcp":{"tool_surface":"admin","compact_answer_enabled":false,"max_request_bytes":4194304,"allowed_origins":["https://example.com"],"execute":{"max_operations":1,"max_intermediates":2,"max_records":3,"max_output_bytes":512,"max_time_seconds":1}},"intelligent_tiers":{"deep_answers":{"enabled":false,"limits":{"max_steps":8}},"model_provider_slots":{"hot_query":{"fallbacks":[]}},"semantic_search":{"enabled":false},"needle_router":{"enabled":false}},"observability":{"graph_explorer":{"enabled":false}}}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadRuntimeConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.SchemaVersion != 2 || config.Repository.BundleRoot != "/" || config.Limits.MaxConceptBytes != 123 || config.MCP.ToolSurface != "admin" {
		t.Fatal(config)
	}
}
