package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

//go:embed proposal_tools.json
var proposalToolDefinitions []byte

//go:embed read_tools.json
var readToolDefinitions []byte

// RegisterReadProposalTools registers the 13 implemented models-off read and
// proposal/control tools. Full configured surfaces/catalog remain unfinished.
func (j *Jobs) RegisterReadProposalTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callProposalTool, notify, readToolDefinitions)
}

type ProposalNotifier func(context.Context, string, map[string]any, []string) error

type proposalToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations"`
	Parameters  []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
		Default  any    `json:"default"`
	} `json:"parameters"`
}

// RegisterProposalTools installs only the nine implemented proposal/control
// tools, in source registry order. It is an explicit development subset, not a
// complete compact/standard/admin surface and does not advertise missing tools.
// Notifications use the caller's session-aware transport sink; nil disables it.
func (j *Jobs) RegisterProposalTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callProposalTool, notify, proposalToolDefinitions)
}
func registerProposalTools(server *umcp.Server, call func(context.Context, string, map[string]any) (any, error), notify ProposalNotifier, raw []byte) error {
	var definitions []proposalToolDefinition
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&definitions); err != nil {
		return err
	}
	metadata := []umcp.DiscoveryItem{}
	for _, definition := range definitions {
		name := definition.Name
		parameters := []umcp.Parameter{}
		for _, parameter := range definition.Parameters {
			kind := umcp.StringParam
			switch parameter.Name {
			case "confirm", "include_asset_metadata", "include_files", "execute", "stop_on_error":
				kind = umcp.BooleanParam
			case "limit", "offset", "depth", "keep", "version_limit", "file_limit":
				kind = umcp.IntegerParam
			case "match", "plan":
				kind = umcp.ObjectParam
			case "changes", "selected_change_indexes", "tags", "aliases", "fields", "items", "operations", "returns":
				kind = umcp.ArrayParam
			}
			parameters = append(parameters, umcp.Parameter{Name: parameter.Name, Types: []umcp.ParamType{kind}, HasDefault: !parameter.Required, Default: parameter.Default})
		}
		tool := umcp.Tool{Name: name, Description: definition.Description, InputSchema: definition.Schema, Annotations: definition.Annotations, Parameters: parameters, Call: func(ctx context.Context, args map[string]any) (any, error) {
			result, err := call(ctx, name, args)
			if err != nil {
				return nil, err
			}
			value, err := MCPEnvelope(result)
			if err != nil {
				return nil, err
			}
			if (name == "memory_proposal_apply" || name == "memory_asset_prune" || name == "memory_create" || name == "memory_patch" || name == "memory_rename" || name == "memory_trash" || name == "memory_restore" || name == "memory_purge") && notify != nil {
				if err = notifyAppliedEnvelope(ctx, server, notify, value); err != nil {
					return nil, err
				}
			}
			return value, nil
		}}
		// Register can only reject a nil Call; this literal always supplies one.
		_ = server.Tools.Register(tool)
		metadata = append(metadata, umcp.DiscoveryItem{Identity: []string{name}, Value: map[string]any{"name": name, "description": definition.Description, "inputSchema": definition.Schema, "annotations": definition.Annotations}})
	}
	// Memento overrides discover_tools (not sorted alphabetically like generic
	// uMCP registration), while retaining the generic pagination machinery.
	server.SetHandler("tools/list", func(ctx context.Context, params map[string]any) (any, *umcp.RPCError, error) {
		page, rpcErr := umcp.ListPage(ctx, "tools", "tools", metadata, params, 0)
		return page, rpcErr, nil
	})
	return nil
}
func notifyAppliedEnvelope(ctx context.Context, server *umcp.Server, notify ProposalNotifier, value any) error {
	object, ok := value.(umcp.OrderedObject)
	if !ok {
		return nil
	}
	status := toolMember(object, "status")
	data := toolMember(object, "data")
	body, ok := data.(umcp.OrderedObject)
	if status != "success" || !ok {
		return nil
	}
	paths := toolMember(body, "changed_paths")
	if values, ok := paths.([]any); ok && len(values) > 0 {
		if err := notify(ctx, "notifications/resources/list_changed", map[string]any{}, nil); err != nil {
			return err
		}
	}
	return server.NotifyResourceUpdated("memory://status", func(ids []string, method string, params map[string]any) error {
		return notify(ctx, method, params, ids)
	})
}
func toolMember(object umcp.OrderedObject, key string) any {
	for _, member := range object {
		if member.Name == key {
			return member.Value
		}
	}
	return nil
}
func (j *Jobs) callProposalTool(ctx context.Context, name string, args map[string]any) (any, error) {
	// Python rejects unconfirmed purge before consulting namespace policy, but
	// still runs it through worker admission and the serialised service lock.
	resolve := name != "memory_purge" || args["confirm"] == true
	return j.callWithPolicy(ctx, name, resolve, func(ctx context.Context, c *ProposalControls, actor ProposalActor) (map[string]any, SuccessOptions, error) {
		return runProposalTool(ctx, c, actor, name, args)
	})
}
func runProposalTool(ctx context.Context, c *ProposalControls, actor ProposalActor, name string, args map[string]any) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	a := toolArguments{values: args}
	var data map[string]any
	var err error
	switch name {
	case "memory_propose":
		intent, base, changes, rationale := a.text("intent"), a.text("base_revision"), a.array("changes"), a.optional("rationale")
		if a.err != nil {
			return nil, options, a.err
		}
		data, err = c.propose(ctx, actor, intent, base, changes, rationale, defaultProposalRepository())
	case "memory_proposal_get":
		id, view := a.text("proposal_id"), a.text("view")
		if a.err != nil {
			return nil, options, a.err
		}
		data, err = c.getProposal(ctx, actor, id, view, defaultProposalRepository())
	case "memory_proposal_list":
		status, limit, cursor := a.optional("status"), a.integer("limit"), a.optional("cursor")
		if a.err != nil {
			return nil, options, a.err
		}
		data, err = c.listProposals(ctx, actor, status, limit, cursor, defaultProposalRepository())
	case "memory_proposal_asset_get":
		id, asset, file, offset, limit := a.text("proposal_id"), a.text("asset_id"), a.optional("file_path"), a.integer("offset"), a.integer("limit")
		if a.err != nil {
			return nil, options, a.err
		}
		data, err = c.ProposalAssetGet(ctx, actor, id, asset, file, int64(offset), int64(limit))
	case "memory_proposal_rebase":
		id, revision, key := a.text("proposal_id"), a.text("expected_revision"), a.text("idempotency_key")
		if a.err != nil {
			return nil, options, a.err
		}
		result, failure := c.rebase(ctx, actor, id, revision, key, defaultProposalRepository())
		data, err = result.Data, failure
		options.OperationID = &result.OperationID
	case "memory_proposal_revise":
		id, revision, intent, rationale := a.text("proposal_id"), a.text("expected_revision"), a.optional("intent"), a.optional("rationale")
		indexes := a.indexes("selected_change_indexes")
		if a.err != nil {
			return nil, options, a.err
		}
		data, err = c.revise(ctx, actor, id, indexes, revision, intent, rationale, defaultProposalRepository())
		options.RepoRevision = &revision
	case "memory_proposal_review":
		id, decision, comment, key := a.text("proposal_id"), a.text("decision"), a.optional("comment"), a.optional("idempotency_key")
		if a.err != nil {
			return nil, options, a.err
		}
		value := ""
		if key != nil {
			value = *key
		}
		result, failure := c.review(ctx, actor, id, decision, comment, value, defaultProposalRepository())
		data, err = result.Data, failure
		options.OperationID = &result.OperationID
	case "memory_proposal_apply":
		id, revision, key := a.text("proposal_id"), a.text("expected_revision"), a.text("idempotency_key")
		if a.err != nil {
			return nil, options, a.err
		}
		manager := repository.TransactionManager{Paths: c.Queue.Paths, Operations: control.Operations{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}, Now: c.Queue.Now, DerivedUpdate: c.DerivedUpdate}
		result, failure := c.apply(ctx, actor, id, revision, key, defaultProposalRepository(), manager.ApplyUnderLock)
		data, err = result.Data, failure
		options.RepoRevision = &result.Revision
		options.OperationID = &result.OperationID
	case "memory_operation_get":
		key, id := a.optional("idempotency_key"), a.optional("operation_id")
		if a.err != nil {
			return nil, options, a.err
		}
		result, failure := c.OperationGet(ctx, actor, key, id)
		data, err = result.Data, failure
		options.RepoRevision = result.Revision
		options.OperationID = result.OperationID
	case "memory_trash", "memory_restore", "memory_purge":
		path, expected, key := a.text("path"), a.text("expected_revision"), a.text("idempotency_key")
		if a.err != nil {
			return nil, options, a.err
		}
		manager := repository.TransactionManager{Paths: c.Queue.Paths, Operations: control.Operations{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}, Now: c.Queue.Now, DerivedUpdate: c.DerivedUpdate}
		data, options, err = c.trashMutation(ctx, actor, name[len("memory_"):], path, expected, key, args["confirm"], defaultProposalRepository(), manager.ApplyUnderLock, defaultMutationIO())
	case "memory_create", "memory_patch", "memory_rename":
		expected, key := a.text("expected_revision"), a.text("idempotency_key")
		if a.err != nil {
			return nil, options, a.err
		}
		raw := map[string]any{"kind": name[len("memory_"):]}
		for key, value := range args {
			if key != "expected_revision" && key != "idempotency_key" {
				raw[key] = value
			}
		}
		change, failure := directChange(raw)
		if failure != nil {
			return nil, options, failure
		}
		manager := repository.TransactionManager{Paths: c.Queue.Paths, Operations: control.Operations{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}, Now: c.Queue.Now, DerivedUpdate: c.DerivedUpdate}
		data, options, err = c.commitConceptChange(ctx, actor, change, expected, key, defaultProposalRepository(), manager.ApplyUnderLock, defaultMutationIO())
	case "memory_asset_prune":
		id, kind, keep, expected, key := a.text("id_or_path"), a.text("asset_kind"), a.integer("keep"), a.text("expected_revision"), a.text("idempotency_key")
		if a.err != nil {
			return nil, options, a.err
		}
		manager := repository.TransactionManager{Paths: c.Queue.Paths, Operations: control.Operations{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}, Now: c.Queue.Now, DerivedUpdate: c.DerivedUpdate}
		var retention any = keep
		if value, ok := a.values["keep"].(bool); ok {
			retention = value
		}
		data, options, err = c.assetPrune(ctx, actor, id, kind, retention, expected, key, manager.ApplyUnderLock, defaultMutationIO())
	case "memory_asset_get":
		o := AssetGetOptions{IDOrPath: a.text("id_or_path"), AssetKind: a.text("asset_kind"), Version: a.optional("version"), View: a.text("view"), FilePath: a.optional("file_path"), Offset: int64(a.integer("offset")), ExpectedSHA256: a.optional("expected_sha256")}
		if a.values["limit"] != nil {
			value := int64(a.integer("limit"))
			o.Limit = &value
		}
		// Unlike most integer inputs, Python's asset range validator rejects
		// bool despite isinstance(bool, int). Map to invalid typed ranges so
		// the service preserves role/view/offset/limit validation order.
		if _, ok := a.values["offset"].(bool); ok {
			o.Offset = -1
		}
		if _, ok := a.values["limit"].(bool); ok {
			value := int64(0)
			o.Limit = &value
		}
		if a.err != nil {
			return nil, options, a.err
		}
		data, options, err = c.AssetGet(ctx, actor, o)
	case "memory_read":
		id := a.text("id_or_path")
		if a.err != nil {
			return nil, options, a.err
		}
		data, err = c.Read(ctx, actor, id)
	case "memory_audit":
		o := AuditOptions{Path: a.optional("path"), Rule: a.optional("rule"), Severity: a.optional("severity"), Cursor: a.optional("cursor"), Limit: a.integer("limit")}
		if a.err != nil {
			return nil, options, a.err
		}
		data, options, err = c.Audit(ctx, actor, o)
	case "memory_asset_metadata":
		o := AssetMetadataOptions{IDOrPath: a.optional("id_or_path"), PathPrefix: a.optional("path_prefix"), AssetKind: a.optional("asset_kind"), Version: a.optional("version"), Cursor: a.optional("cursor"), Limit: a.integer("limit"), VersionLimit: a.integer("version_limit"), FileLimit: a.integer("file_limit"), IncludeFiles: args["include_files"]}
		if a.err != nil {
			return nil, options, a.err
		}
		data, options, err = c.AssetMetadata(ctx, actor, o)
	case "memory_compare_manifest":
		prefix, items := a.text("path_prefix"), a.array("items")
		var match map[string]any
		if args["match"] != nil {
			var ok bool
			match, ok = args["match"].(map[string]any)
			if !ok {
				a.err = umcp.ExecutionError{Type: "TypeError", Message: "manifest match must be a mapping"}
			}
		}
		if a.err != nil {
			return nil, options, a.err
		}
		data, options, err = c.CompareManifest(ctx, actor, prefix, items, match, manifestTruthy(args["include_asset_metadata"]))
	case "memory_inventory":
		prefix, limit, cursor := a.text("path_prefix"), a.integer("limit"), a.optional("cursor")
		var fields []string
		if a.values["fields"] != nil {
			fields = []string{}
			var values []any
			if text, ok := a.values["fields"].(string); ok {
				// Python's Sequence annotation does not enforce a list: a string
				// is iterated by code point before field validation.
				for _, r := range text {
					values = append(values, string(r))
				}
			} else {
				values = a.array("fields")
			}
			for _, value := range values {
				field, ok := value.(string)
				if !ok {
					a.err = umcp.ExecutionError{Type: "TypeError", Message: "inventory fields must be strings"}
					break
				}
				fields = append(fields, field)
			}
		}
		if a.err != nil {
			return nil, options, a.err
		}
		data, options, err = c.Inventory(ctx, actor, prefix, fields, limit, cursor)
	case "memory_list":
		prefix := a.text("path_prefix")
		if a.err != nil {
			return nil, options, a.err
		}
		data, err = c.List(ctx, actor, prefix)
	case "memory_search":
		query, kind, limit, cursor, mode, syntax := a.text("query"), a.optional("concept_type"), a.integer("limit"), a.optional("cursor"), a.optional("search_mode"), a.text("query_syntax")
		if a.err != nil {
			return nil, options, a.err
		}
		data, options, err = c.Search(ctx, actor, query, kind, limit, cursor, mode, syntax)
	case "memory_graph":
		id, depth := a.text("id_or_path"), a.integer("depth")
		if a.err != nil {
			return nil, options, a.err
		}
		data, options, err = c.Graph(ctx, actor, id, depth)
	default:
		return nil, options, errors.New("unregistered proposal method")
	}
	return data, options, err
}

// uMCP applies source signature coercion before this typed service boundary.
// Discovery JSON Schema is metadata, not an excuse to reinterpret source calls.
// Non-JSON/malformed dynamic types fail explicitly; full Python diagnostics and
// arbitrary-precision integer service arguments remain a separate parity gap.
type toolArguments struct {
	values map[string]any
	err    error
}

func (a *toolArguments) text(key string) string {
	v, ok := a.values[key].(string)
	if !ok {
		a.err = umcp.ExecutionError{Type: "TypeError", Message: key + " must be a string"}
	}
	return v
}
func (a *toolArguments) optional(key string) *string {
	if a.values[key] == nil {
		return nil
	}
	value := a.text(key)
	return &value
}
func (a *toolArguments) array(key string) []any {
	v, ok := a.values[key].([]any)
	if !ok {
		a.err = umcp.ExecutionError{Type: "TypeError", Message: key + " must be a list"}
	}
	return v
}
func toolInteger(value any) (int, error) {
	if b, ok := value.(bool); ok {
		if b {
			return 1, nil
		}
		return 0, nil
	}
	n, ok := value.(json.Number)
	if !ok {
		return 0, errors.New("integer required")
	}
	parsed, err := strconv.ParseInt(n.String(), 10, 0)
	return int(parsed), err
}
func (a *toolArguments) integer(key string) int {
	value, err := toolInteger(a.values[key])
	if err != nil {
		a.err = umcp.ExecutionError{Type: "TypeError", Message: key + " must be a supported integer"}
	}
	return value
}
func (a *toolArguments) indexes(key string) []int {
	raw := a.array(key)
	indexes := []int{}
	for _, value := range raw {
		index, err := toolInteger(value)
		if err != nil {
			a.err = umcp.ExecutionError{Type: "TypeError", Message: fmt.Sprintf("%s contains an unsupported index", key)}
			return nil
		}
		indexes = append(indexes, index)
	}
	return indexes
}
