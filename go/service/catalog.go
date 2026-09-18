package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rcarmo/memento/go/umcp"
)

//go:embed catalog_data.json
var catalogData []byte

//go:embed catalog_tools.json
var catalogToolDefinitions []byte

type CatalogConfig struct {
	Surface                     string
	AnswerEnabled, RouteEnabled bool
	ExecuteLimits               map[string]any
}
type catalogOperation struct {
	Name     string   `json:"op_name"`
	Tool     string   `json:"tool_name"`
	Surfaces []string `json:"discovery_surfaces"`
}
type catalogSource struct {
	Operations           []catalogOperation        `json:"operations"`
	Contracts            map[string]map[string]any `json:"contracts"`
	Workflows            map[string]map[string]any `json:"workflows"`
	ExecuteCapable       []string                  `json:"execute_capable"`
	Limits               map[string]any            `json:"limits"`
	Resources, Templates []struct {
		URI                      string `json:"uri"`
		Template                 string `json:"uriTemplate"`
		Name, Title, Description string
		MIME                     string `json:"mimeType"`
	}
	Prompts []struct {
		Name, Description string
		Schema            map[string]any `json:"inputSchema"`
	}
	PromptDescription string   `json:"prompt_description"`
	OperationValues   []string `json:"operation_values"`
	WorkflowValues    []string `json:"workflow_values"`
}

// Catalog owns immutable source-derived contracts. Configured discovery is not
// registered until every advertised or execute-only operation has a handler.
type Catalog struct {
	config  CatalogConfig
	source  catalogSource
	visible map[string]bool
	execute bool
}

func NewCatalog(config CatalogConfig) (*Catalog, error) { return newCatalog(config, catalogData) }
func newCatalog(config CatalogConfig, raw []byte) (*Catalog, error) {
	if config.Surface != "compact" && config.Surface != "standard" && config.Surface != "read_only" && config.Surface != "curator" && config.Surface != "admin" {
		return nil, fmt.Errorf("unsupported tool surface: %s", config.Surface)
	}
	var source catalogSource
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&source); err != nil {
		return nil, err
	}
	c := &Catalog{config: config, source: source, visible: map[string]bool{}}
	c.config.ExecuteLimits = copyCatalogObject(config.ExecuteLimits)
	if config.ExecuteLimits == nil {
		c.config.ExecuteLimits = copyCatalogObject(source.Limits)
	}
	for _, op := range source.Operations {
		included := false
		for _, surface := range op.Surfaces {
			if surface == config.Surface {
				included = true
			}
		}
		if op.Tool == "memory_answer" && (config.Surface == "compact" || config.Surface == "curator") && !config.AnswerEnabled {
			included = false
		}
		if op.Tool == "memory_route" && !config.RouteEnabled {
			included = false
		}
		if included {
			c.visible[op.Name] = true
		}
	}
	c.execute = c.visible["execute"]
	return c, nil
}

// Copy JSON-domain contract data without losing json.Number or aliasing maps.
func copyCatalog(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range v {
			out[k] = copyCatalog(x)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = copyCatalog(x)
		}
		return out
	default:
		return v
	}
}
func copyCatalogObject(value map[string]any) map[string]any {
	return copyCatalog(value).(map[string]any)
}
func (c *Catalog) ToolNames() []string {
	names := []string{}
	for _, op := range c.source.Operations {
		if c.visible[op.Name] {
			names = append(names, op.Tool)
		}
	}
	return names
}
func (c *Catalog) executeOnly() []string {
	names := []string{}
	if !c.execute {
		return names
	}
	capable := map[string]bool{}
	for _, name := range c.source.ExecuteCapable {
		capable[name] = true
	}
	for _, op := range c.source.Operations {
		if !c.visible[op.Name] && op.Name != "execute" && capable[op.Name] {
			names = append(names, op.Name)
		}
	}
	return names
}
func (c *Catalog) Operation(name string) (map[string]any, error) {
	source, ok := c.source.Contracts[name]
	if !ok {
		return nil, fmt.Errorf("unknown operation: %s", name)
	}
	result := copyCatalogObject(source)
	visible := c.visible[name]
	result["direct_tool_available"] = visible
	execute := false
	for _, operation := range c.source.ExecuteCapable {
		if operation == name {
			execute = true
		}
	}
	result["available_via_execute"] = !visible && c.execute && execute
	return result, nil
}

// Workflow returns one generated, source-verified workflow contract.
func (c *Catalog) Workflow(goal string) (map[string]any, error) {
	meta, ok := c.source.Workflows[goal]
	if !ok {
		return nil, fmt.Errorf("unknown workflow: %s", goal)
	}
	direct, execute := []any{}, []any{}
	for _, raw := range meta["operations"].([]any) {
		name := raw.(string)
		contract, _ := c.Operation(name)
		if c.visible[name] {
			direct = append(direct, contract)
		} else if c.execute {
			execute = append(execute, contract)
		}
	}
	payload := map[string]any{"goal": goal, "uri": "memory://workflow/" + goal, "description": meta["description"], "operations": direct, "execute_only_operations": execute}
	for _, key := range []string{"profile", "steps", "staging_fallback"} {
		if value, ok := meta[key]; ok {
			payload[key] = copyCatalog(value)
		}
	}
	return payload, nil
}
func (c *Catalog) Payload() map[string]any {
	direct := []any{}
	for _, op := range c.source.Operations {
		if c.visible[op.Name] {
			contract, _ := c.Operation(op.Name)
			direct = append(direct, contract)
		}
	}
	workflows := map[string]any{}
	for goal := range c.source.Workflows {
		workflows[goal], _ = c.Workflow(goal)
	}
	payload := map[string]any{"tool_surface": c.config.Surface, "operations": direct, "workflows": workflows}
	hidden := c.executeOnly()
	if len(hidden) > 0 {
		items := []any{}
		for _, name := range hidden {
			contract, _ := c.Operation(name)
			items = append(items, contract)
		}
		payload["execute_only_operations"] = items
	}
	return payload
}
func (c *Catalog) Help() map[string]any {
	goals := map[string]any{}
	groups := map[string][]string{
		"read": {"memory_search", "memory_read", "memory_graph", "memory_answer"}, "browse": {"memory_list", "memory_read"},
		"propose": {"memory_propose", "memory_propose_freeform", "memory_propose_update", "memory_proposal_get"},
		"curate":  {"memory_proposal_list", "memory_proposal_get", "memory_proposal_asset_get", "memory_proposal_revise", "memory_operation_get", "memory_proposal_review", "memory_proposal_apply", "memory_create", "memory_patch", "memory_rename"},
		"skills":  {"memory_search", "memory_read"}, "compact": c.ToolNames(),
	}
	visible := map[string]bool{}
	for _, name := range c.ToolNames() {
		visible[name] = true
	}
	for goal, tools := range groups {
		filtered := []string{}
		for _, tool := range tools {
			if visible[tool] {
				filtered = append(filtered, tool)
			}
		}
		if len(filtered) > 0 {
			goals[goal] = filtered
		}
	}
	only := map[string]any{}
	if c.execute {
		for goal, meta := range c.source.Workflows {
			extra := []string{}
			for _, v := range meta["operations"].([]any) {
				name := v.(string)
				if !c.visible[name] {
					extra = append(extra, name)
				}
			}
			if len(extra) > 0 {
				only[goal] = extra
			}
		}
	}
	operations := []string{}
	for _, op := range c.source.Operations {
		operations = append(operations, op.Name)
	}
	// Source dict order is part of this tuple, unlike ordinary JSON object keys.
	workflows := []string{"inspect", "propose", "curate", "trash", "asset_pack"}
	direct := c.ToolNames()
	sort.Strings(direct)
	return map[string]any{"goals": goals, "formats": []string{"summary", "detailed"}, "answer_sources": []string{"exact_cache", "hot_memory", "deep_agent", "disabled"}, "search_modes": []string{"lexical", "semantic", "hybrid"}, "catalog": map[string]any{"resources": []string{"memory://help", "memory://status", "memory://catalog"}, "templates": []string{"memory://catalog/{operation}", "memory://workflow/{goal}"}, "operation_values": operations, "workflow_values": workflows, "asset_publication": map[string]any{"workflow": "memory://workflow/asset_pack", "proposal_contract": "memory://catalog/propose", "prompt": "publish_asset_pack"}}, "mcp": map[string]any{"tool_surface": c.config.Surface, "direct_tools": direct, "compact_instructions": catalogInstructions, "execute_limits": copyCatalogObject(c.config.ExecuteLimits), "execute_only_operations": only}}
}

const catalogInstructions = "Use memory_search, then memory_read, or use memory_execute with saved references like $hits.results.0.path. For staged ZIP uploads, read memory://workflow/asset_pack and memory://catalog/propose before submitting an attach_asset_pack change. Do not use scripts or hand-built/raw MCP protocol calls to work around interaction gaps; use supported MCP tools and documented upload workflows, and file a GitHub issue at https://github.com/rcarmo/memento/issues when an interaction mode is missing. If enabled, memory_route can classify one shallow read request into a deterministic action."

type CatalogHandler func(context.Context, map[string]any) (any, error)

// Register installs source discovery, resources and prompt only on a fresh
// server. All operation handlers must exist before any mutation: Python hides
// tools from discovery, not dispatch. Callers own authentication, policy,
// execute normalisation/admission and envelopes. Managed access_* tools are
// a separate, still-unported registration layer.
func (c *Catalog) Register(server *umcp.Server, handlers map[string]CatalogHandler, notify ProposalNotifier) error {
	required := map[string]bool{"memory_help": true, "memory_status": true}
	for _, op := range c.source.Operations {
		required[op.Tool] = true
	}
	missing := []string{}
	for name := range required {
		if handlers[name] == nil {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("incomplete %s surface: missing handlers: %s", c.config.Surface, strings.Join(missing, ", "))
	}
	return c.register(server, handlers, notify, catalogToolDefinitions)
}
func (c *Catalog) register(server *umcp.Server, handlers map[string]CatalogHandler, notify ProposalNotifier, raw []byte) error {
	var definitions []proposalToolDefinition
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&definitions); err != nil {
		return err
	}
	selected := []proposalToolDefinition{}
	names := map[string]bool{}
	for _, name := range c.ToolNames() {
		names[name] = true
	}
	for _, definition := range definitions {
		if names[definition.Name] {
			selected = append(selected, definition)
		}
	}
	encoded, _ := json.Marshal(selected) // fixed JSON-domain definitions
	// Capture caller callbacks; mutating the caller map later cannot swap auth.
	owned := map[string]CatalogHandler{}
	for name, handler := range handlers {
		owned[name] = handler
	}
	// Discovery never authorises a call. Python getattr can invoke hidden tools.
	// Register all callbacks, then replace only discovery with the visible list.
	_ = registerProposalTools(server, func(ctx context.Context, name string, args map[string]any) (any, error) {
		return owned[name](ctx, args)
	}, notify, raw)
	// A successfully decoded definition list re-encodes to a valid JSON array;
	// registerProposalTools has no other failure path for generated definitions.
	_ = registerProposalTools(server, func(ctx context.Context, name string, args map[string]any) (any, error) {
		return owned[name](ctx, args)
	}, notify, encoded)
	return c.registerCatalogResources(server, owned)
}
