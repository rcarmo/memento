package service

import (
	"context"
	"strings"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/repository"
)

// ReadIndex is implemented by the models-off derived.Index lifecycle owner.
// It owns index migration, quarantine and per-call database connections.
type ReadIndex interface {
	SearchLexical(context.Context, access.EffectivePolicy, derived.SearchOptions) (derived.SearchPage, error)
	Graph(context.Context, access.EffectivePolicy, string, derived.GraphOptions) (derived.GraphNeighborhood, error)
}
type SemanticReadIndex interface {
	SearchSemantic(context.Context, access.EffectivePolicy, derived.SemanticSearchOptions, derived.SemanticClient) (derived.SearchPage, error)
}

func readable(policy access.EffectivePolicy, path string) bool {
	_, err := access.AuthorizePath(policy, path, "read")
	return err == nil
}
func (c *ProposalControls) readBundle(policy access.EffectivePolicy) (repository.RepositoryBundle, error) {
	return repository.ScanBundle(c.Queue.Paths.CurrentDir, repository.BundleFilter{IncludePath: func(path string) bool { return readable(policy, path) }})
}
func (c *ProposalControls) resolveReadPath(policy access.EffectivePolicy, id string) (string, error) {
	return c.resolvePath(policy, id, "read")
}
func (c *ProposalControls) resolvePath(policy access.EffectivePolicy, id, action string) (string, error) {
	if strings.HasPrefix(id, "/") {
		return id, nil
	}
	bundle, err := repository.ScanBundle(c.Queue.Paths.CurrentDir, repository.BundleFilter{IncludePath: func(path string) bool { _, err := access.AuthorizePath(policy, path, action); return err == nil }})
	if err != nil {
		return "", err
	}
	for _, entry := range bundle.Entries {
		if entry.Document.Frontmatter.ID == id {
			return entry.BundlePath, nil
		}
	}
	return "", &Error{"not_found", id}
}

// Read does not require a role beyond namespace policy. ID resolution scans all
// readable concepts, so a malformed readable sibling can prevent ID lookup.
func (c *ProposalControls) Read(ctx context.Context, actor ProposalActor, id string) (map[string]any, error) {
	path, err := c.resolveReadPath(actor.Policy, id)
	if err != nil {
		return nil, err
	}
	if _, err = access.AuthorizePath(actor.Policy, path, "read"); err != nil {
		return nil, err
	}
	entry, err := repository.ReadBundleEntry(c.Queue.Paths.CurrentDir, path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": entry.BundlePath, "frontmatter": frontmatterPayload(entry.Document.Frontmatter), "body": entry.Document.Body}, nil
}
func frontmatterPayload(m repository.ConceptFrontmatter) map[string]any {
	return map[string]any{"schema_version": m.SchemaVersion, "id": m.ID, "type": m.Type, "title": m.Title, "status": m.Status, "description": nullableText(m.Description), "aliases": append([]string{}, m.Aliases...), "tags": append([]string{}, m.Tags...), "source_refs": append([]string{}, m.SourceRefs...), "supersedes": append([]string{}, m.Supersedes...), "created_at": modelTimestamp(m.CreatedAt), "updated_at": modelTimestamp(m.UpdatedAt), "updated_by": m.UpdatedBy}
}
func modelTimestamp(value time.Time) string {
	value = value.UTC()
	text := value.Format("2006-01-02T15:04:05")
	if value.Nanosecond()/1000 != 0 {
		text += value.Format(".000000")
	}
	return text + "Z"
}

// List preserves literal startswith filtering, not canonical namespace-prefix
// validation. It parses every readable concept before filtering the prefix.
func (c *ProposalControls) List(ctx context.Context, actor ProposalActor, prefix string) (map[string]any, error) {
	bundle, err := c.readBundle(actor.Policy)
	if err != nil {
		return nil, err
	}
	entries := []any{}
	for _, entry := range bundle.Entries {
		if !strings.HasPrefix(entry.BundlePath, prefix) || (strings.HasPrefix(entry.BundlePath, "/trash/") && !strings.HasPrefix(prefix, "/trash/")) {
			continue
		}
		m := entry.Document.Frontmatter
		entries = append(entries, map[string]any{"path": entry.BundlePath, "id": m.ID, "title": m.Title, "type": m.Type, "status": m.Status, "description": nullableText(m.Description), "aliases": append([]string{}, m.Aliases...), "tags": append([]string{}, m.Tags...)})
	}
	return map[string]any{"entries": entries}, nil
}
func (c *ProposalControls) Search(ctx context.Context, actor ProposalActor, query string, conceptType *string, limit int, cursor, mode *string, syntax string) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	selected := c.DefaultSearchMode
	if selected == "" {
		selected = "lexical"
	}
	if mode != nil && *mode != "" {
		selected = *mode
	}
	if selected != "lexical" && selected != "semantic" && selected != "hybrid" {
		return nil, options, &derived.SearchError{Message: "unsupported search_mode: " + selected}
	}
	if c.Index == nil {
		return nil, options, &derived.UnavailableError{Message: "derived index is unavailable"}
	}
	searchOptions := derived.SearchOptions{Query: query, Syntax: syntax, ConceptType: conceptType, Limit: limit, Cursor: cursor}
	var page derived.SearchPage
	var err error
	semanticIndex, semanticAvailable := c.Index.(SemanticReadIndex)
	if selected != "lexical" && c.SemanticClient != nil && semanticAvailable {
		page, err = semanticIndex.SearchSemantic(ctx, actor.Policy, derived.SemanticSearchOptions{SearchOptions: searchOptions, Hybrid: selected == "hybrid", MaxCandidates: c.SemanticMaxCandidates}, c.SemanticClient)
	} else {
		page, err = c.Index.SearchLexical(ctx, actor.Policy, searchOptions)
	}
	if err != nil {
		return nil, options, err
	}
	results := []any{}
	for _, item := range page.Results {
		results = append(results, map[string]any{"id": item.ConceptID, "path": item.Path, "title": item.Title, "type": item.ConceptType, "status": item.Status, "tags": item.Tags, "score": item.Score, "snippet": item.Snippet})
	}
	warnings := append([]string{}, page.Warnings...)
	if selected != "lexical" && (c.SemanticClient == nil || !semanticAvailable) {
		warnings = append(warnings, "semantic_search_unavailable: semantic search embedding client is unavailable")
	}
	options.RepoRevision = &page.RepoRevision
	options.IndexRevision = &page.IndexRevision
	options.Warnings = warnings
	return map[string]any{"search_mode": selected, "query_syntax": syntax, "results": results, "next_cursor": nullableText(page.NextCursor)}, options, nil
}
func (c *ProposalControls) Graph(ctx context.Context, actor ProposalActor, id string, depth int) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if strings.HasPrefix(id, "/") {
		if _, err := access.AuthorizePath(actor.Policy, id, "read"); err != nil {
			return nil, options, err
		}
		entry, err := repository.ReadBundleEntry(c.Queue.Paths.CurrentDir, id)
		if err != nil {
			return nil, options, err
		}
		id = entry.Document.Frontmatter.ID
	}
	if c.Index == nil {
		return nil, options, &derived.UnavailableError{Message: "derived index is unavailable"}
	}
	graph, err := c.Index.Graph(ctx, actor.Policy, id, derived.GraphOptions{Depth: depth})
	if err != nil {
		return nil, options, err
	}
	options.RepoRevision = &graph.RepoRevision
	options.IndexRevision = &graph.IndexRevision
	return map[string]any{"center_id": graph.CenterID, "outbound": graph.Outbound, "inbound": graph.Inbound, "broken_targets": graph.BrokenTargets}, options, nil
}
