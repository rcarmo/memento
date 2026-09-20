package service

import (
	"context"
	_ "embed"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"syscall"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
	"modernc.org/sqlite"
)

//go:embed audit_tools.json
var auditToolDefinitions []byte

// RegisterAuditTools exposes 29 implemented development tools. Graph snapshots
// must come from a policy-scoped provider; nil reports not_configured.
func (j *Jobs) RegisterAuditTools(server *umcp.Server, notify ProposalNotifier) error {
	return j.registerStatusTools(server, notify, auditToolDefinitions)
}

type AuditOptions struct {
	Path, Rule, Severity, Cursor *string
	Limit                        int
}
type AuditGraphNode struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}
type AuditDiagnostic struct {
	ID         string         `json:"id"`
	Rule       string         `json:"rule"`
	Severity   string         `json:"severity"`
	ConceptIDs []string       `json:"concept_ids"`
	Message    string         `json:"message"`
	Measured   map[string]any `json:"measured"`
	Threshold  map[string]any `json:"threshold"`
	Derived    bool           `json:"derived"`
}
type AuditOverview struct {
	IndexRevision string            `json:"index_revision"`
	Stale         bool              `json:"stale"`
	Nodes         []AuditGraphNode  `json:"nodes"`
	Diagnostics   []AuditDiagnostic `json:"diagnostics"`
}
type AuditGraphSnapshots interface {
	Overview(context.Context, access.EffectivePolicy) (AuditOverview, error)
}
type GraphSnapshotError struct{ Message string }

func (e *GraphSnapshotError) Error() string { return e.Message }

func writableAuditPolicy(policy access.EffectivePolicy) access.EffectivePolicy {
	candidates := map[string]bool{}
	for _, write := range policy.WritePrefixes {
		for _, read := range policy.ReadPrefixes {
			if strings.HasPrefix(write, read) {
				candidates[write] = true
			} else if strings.HasPrefix(read, write) {
				candidates[read] = true
			}
		}
	}
	allowed := []string{}
	for candidate := range candidates {
		if _, err := access.AuthorizePath(policy, candidate, "read"); err != nil {
			continue
		}
		if _, err := access.AuthorizePath(policy, candidate, "write"); err != nil {
			continue
		}
		allowed = append(allowed, candidate)
	}
	sort.Strings(allowed)
	policy.ReadPrefixes = allowed
	policy.WritePrefixes = append([]string{}, allowed...)
	return policy
}
func (c *ProposalControls) Audit(ctx context.Context, actor ProposalActor, o AuditOptions) (map[string]any, SuccessOptions, error) {
	return c.audit(ctx, actor, o, defaultProposalRepository(), repository.AuditRepository)
}
func (c *ProposalControls) audit(ctx context.Context, actor ProposalActor, o AuditOptions, repo proposalRepository, audit func(string, func(string) bool) (repository.RepositoryAudit, error)) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if err := access.RequireRole(actor.Policy, "proposer"); err != nil {
		return nil, options, err
	}
	scope := writableAuditPolicy(actor.Policy)
	if o.Path != nil {
		for _, action := range []string{"read", "write"} {
			if _, err := access.AuthorizePath(actor.Policy, *o.Path, action); err != nil {
				return nil, options, err
			}
		}
	}
	if o.Limit < 1 || o.Limit > 200 {
		return nil, options, &Error{"validation_error", "limit must be between 1 and 200"}
	}
	report, err := audit(c.Queue.Paths.CurrentDir, func(path string) bool { return readable(scope, path) })
	if err != nil {
		return nil, options, err
	}
	issues := []repository.AuditIssue{}
	for _, issue := range report.Issues {
		if o.Path == nil || issue.BundlePath == *o.Path {
			issues = append(issues, issue)
		}
	}
	revision, err := repo.main(c.Queue.Paths)
	if err != nil {
		return nil, options, err
	}
	graph, err := c.auditGraph(ctx, scope, revision, o)
	if err != nil {
		return nil, options, err
	}
	options.RepoRevision = &revision
	if value, ok := graph["index_revision"].(string); ok {
		options.IndexRevision = &value
	}
	// Source _success substitutes repository revision for a None index_revision,
	// while graph_diagnostics.index_revision itself remains null.
	return map[string]any{"ok": len(issues) == 0 && len(graph["diagnostics"].([]any)) == 0, "issues": issues, "graph_diagnostics": graph}, options, nil
}
func auditFilters(o AuditOptions) map[string]any {
	return map[string]any{"path": nullableText(o.Path), "rule": nullableText(o.Rule), "severity": nullableText(o.Severity)}
}
func (c *ProposalControls) auditGraph(ctx context.Context, policy access.EffectivePolicy, revision string, o AuditOptions) (map[string]any, error) {
	filters := auditFilters(o)
	empty := map[string]any{"available": false, "reason": "not_configured", "repository_revision": revision, "index_revision": nil, "filters": filters, "diagnostics": []any{}, "next_cursor": nil}
	if len(policy.ReadPrefixes) == 0 {
		empty["available"] = true
		empty["reason"] = nil
		empty["scope_empty"] = true
		return empty, nil
	}
	if c.AuditGraph == nil {
		return empty, nil
	}
	overview, err := c.AuditGraph.Overview(ctx, policy)
	if err != nil {
		var graph *GraphSnapshotError
		var ioerr *fs.PathError
		var sqlerr *sqlite.Error
		var errno syscall.Errno
		switch {
		case errors.As(err, &graph):
			reason := "index_unavailable"
			if strings.Contains(strings.ToLower(err.Error()), "stale") {
				reason = "index_stale"
			}
			empty["reason"] = reason
		case errors.As(err, &ioerr) || errors.As(err, &sqlerr) || errors.As(err, &errno):
			empty["reason"] = "index_unavailable"
		default:
			return nil, err
		}
		return empty, nil
	}
	if overview.Stale || overview.IndexRevision != revision {
		empty["reason"] = "index_stale"
		empty["index_revision"] = overview.IndexRevision
		return empty, nil
	}
	nodePaths := map[string]string{}
	for _, node := range overview.Nodes {
		nodePaths[node.ID] = node.Path
	}
	type row struct {
		key   [3]string
		item  AuditDiagnostic
		paths []string
	}
	filtered := []row{}
	for _, item := range overview.Diagnostics {
		seen := map[string]bool{}
		for _, id := range item.ConceptIDs {
			if path, ok := nodePaths[id]; ok {
				seen[path] = true
			}
		}
		paths := []string{}
		for path := range seen {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		if o.Path != nil && !seen[*o.Path] || o.Rule != nil && item.Rule != *o.Rule || o.Severity != nil && item.Severity != *o.Severity {
			continue
		}
		filtered = append(filtered, row{[3]string{item.Severity, item.Rule, item.ID}, item, paths})
	}
	sort.SliceStable(filtered, func(i, j int) bool { return auditKeyLess(filtered[i].key, filtered[j].key) })
	after, err := decodeAuditCursor(o.Cursor, revision, filters)
	if err != nil {
		return nil, err
	}
	candidates := []row{}
	for _, row := range filtered {
		if after == nil || auditKeyLess(*after, row.key) {
			candidates = append(candidates, row)
		}
	}
	selected := candidates[:min(o.Limit, len(candidates))]
	var next any
	if len(candidates) > len(selected) && len(selected) > 0 {
		next, err = encodeAuditCursor(selected[len(selected)-1].key, revision, filters)
		if err != nil {
			return nil, err
		}
	}
	payloads := []any{}
	for _, row := range selected {
		item := row.item
		measured := copyCatalogObject(item.Measured)
		threshold := copyCatalogObject(item.Threshold)
		payloads = append(payloads, map[string]any{"id": item.ID, "rule": item.Rule, "severity": item.Severity, "concept_ids": append([]string{}, item.ConceptIDs...), "message": item.Message, "measured": measured, "threshold": threshold, "derived": item.Derived, "paths": row.paths, "repair_guidance": graphRepairGuidance(item, row.paths)})
	}
	return map[string]any{"available": true, "reason": nil, "scope_empty": false, "repository_revision": revision, "index_revision": overview.IndexRevision, "filters": filters, "diagnostics": payloads, "next_cursor": next}, nil
}
func auditKeyLess(a, b [3]string) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
func graphRepairGuidance(item AuditDiagnostic, paths []string) map[string]any {
	actions := []any{}
	for _, path := range paths[:min(2, len(paths))] {
		actions = append(actions, map[string]any{"tool": "memory_read", "arguments": map[string]any{"id_or_path": path}})
	}
	if item.Rule == "pending_proposals" {
		actions = append(actions, map[string]any{"tool": "memory_proposal_list", "arguments": map[string]any{"status": "submitted"}})
	} else if item.Rule == "embedding_health" {
		actions = append(actions, map[string]any{"tool": "memory_status", "arguments": map[string]any{}})
	} else if len(paths) > 0 {
		actions = append(actions, map[string]any{"tool": "memory_propose_update", "arguments": map[string]any{"instruction": "Address graph diagnostic " + item.ID + ". Read the current memory first; preserve unaffected content and make the smallest local merge or patch.", "target_hint": paths[0]}})
	}
	return map[string]any{"read_only": true, "summary": "Inspect current memory and related proposals first. Draft a normal proposal for the smallest justified local repair; curator review and apply remain required.", "actions": actions[:min(4, len(actions))]}
}
