package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcarmo/memento/go/service"
)

func TestRunIntelligentConfigGuards(t *testing.T) {
	oldLoad := loadConfig
	t.Cleanup(func() { loadConfig = oldLoad })
	cases := []struct {
		name  string
		tiers service.IntelligentTiersConfig
		want  string
	}{
		{"deep decode", service.IntelligentTiersConfig{DeepAnswers: json.RawMessage(`{"extra":1}`)}, "unknown field"},
		{"cache decode", service.IntelligentTiersConfig{ExactAnswerCache: json.RawMessage(`{"extra":1}`)}, "unknown field"},
		{"hot decode", service.IntelligentTiersConfig{HotWorkingMemory: json.RawMessage(`{"extra":1}`)}, "unknown field"},
		{"hot provider", service.IntelligentTiersConfig{HotWorkingMemory: json.RawMessage(`{"enabled":true}`)}, "hot_query"},
		{"deep provider", service.IntelligentTiersConfig{DeepAnswers: json.RawMessage(`{"enabled":true}`)}, "deep_query"},
		{"proposal provider", service.IntelligentTiersConfig{ModelProposals: json.RawMessage(`{"enabled":true}`)}, "proposal model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loadConfig = func(string) (service.RuntimeConfig, error) {
				var c service.RuntimeConfig
				c.MCP.ToolSurface = "standard"
				c.IntelligentTiers = tc.tiers
				return c, nil
			}
			var out, stderr bytes.Buffer
			code := runContext(context.Background(), []string{"--config", "x", "status"}, nil, &out, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
		})
	}
}
