package umcp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Resource is an explicitly registered static URI or URI template. The caller
// chooses Template; parameters are raw captured segments without URL unescaping.
type Resource struct {
	URI, Name, Title, Description, MIME string
	Size                                *int
	Annotations                         map[string]any
	Template                            bool
	Read                                func(context.Context, map[string]any) (any, error)
	Parameters                          []Parameter
	InputSchema                         map[string]any
}
type resourceEntry struct {
	resource Resource
	pattern  *regexp.Regexp
	names    []string
}

// ResourceRegistry implements dynamic discovery/read and subscription state.
// Session IDs supplied here must be bound/authenticated by the transport.
type ResourceRegistry struct {
	mu              sync.RWMutex
	static          map[string]resourceEntry
	templates       []resourceEntry
	subscriptions   map[string]map[string]bool
	DefaultPageSize int
}

var placeholder = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func templatePattern(uri string) (*regexp.Regexp, []string, error) {
	matches := placeholder.FindAllStringSubmatchIndex(uri, -1)
	names := []string{}
	seen := map[string]bool{}
	var pattern strings.Builder
	pattern.WriteString("^")
	cursor := 0
	for _, m := range matches {
		name := uri[m[2]:m[3]]
		if seen[name] {
			return nil, nil, fmt.Errorf("redefinition of group name '%s'", name)
		}
		seen[name] = true
		pattern.WriteString(regexp.QuoteMeta(uri[cursor:m[0]]))
		pattern.WriteString("([^/]+)")
		names = append(names, name)
		cursor = m[1]
	}
	pattern.WriteString(regexp.QuoteMeta(uri[cursor:]))
	pattern.WriteString("$")
	// Only generated captures and quoted literals enter the expression.
	compiled := regexp.MustCompile(pattern.String())
	return compiled, names, nil
}
func resourceMetadata(r Resource) map[string]any {
	key := "uri"
	if r.Template {
		key = "uriTemplate"
	}
	metadata := map[string]any{key: r.URI, "name": r.Name}
	if r.Title != "" {
		metadata["title"] = r.Title
	}
	if r.Description != "" {
		metadata["description"] = r.Description
	}
	if r.MIME != "" {
		metadata["mimeType"] = r.MIME
	}
	if r.Size != nil {
		metadata["size"] = *r.Size
	}
	if len(r.Annotations) > 0 {
		metadata["annotations"] = cloneMap(r.Annotations)
	}
	return metadata
}

// Register replaces static entries and appends templates in dispatch order.
func (r *ResourceRegistry) Register(resource Resource) error {
	if resource.Read == nil {
		return errors.New("resource handler is required")
	}
	if resource.Name == "" {
		resource.Name = resource.URI
	}
	resource.Annotations = cloneMap(resource.Annotations)
	resource.InputSchema = cloneMap(resource.InputSchema)
	resource.Parameters = cloneTool(Tool{Parameters: resource.Parameters}).Parameters
	if resource.Size != nil {
		size := *resource.Size
		resource.Size = &size
	}
	entry := resourceEntry{resource: resource}
	if resource.Template {
		var err error
		entry.pattern, entry.names, err = templatePattern(resource.URI)
		if err != nil {
			return err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if resource.Template {
		r.templates = append(r.templates, entry)
	} else {
		if r.static == nil {
			r.static = make(map[string]resourceEntry)
		}
		r.static[resource.URI] = entry
	}
	return nil
}
func (r *ResourceRegistry) list(ctx context.Context, params map[string]any, templates bool) (any, *RPCError, error) {
	r.mu.RLock()
	entries := []resourceEntry{}
	if templates {
		entries = append(entries, r.templates...)
	} else {
		for _, entry := range r.static {
			entries = append(entries, entry)
		}
	}
	pageSize := r.DefaultPageSize
	r.mu.RUnlock()
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].resource, entries[j].resource
		if a.Name == b.Name {
			return a.URI < b.URI
		}
		return a.Name < b.Name
	})
	items := make([]DiscoveryItem, 0, len(entries))
	for _, entry := range entries {
		res := entry.resource
		items = append(items, DiscoveryItem{Identity: []string{res.URI, res.Name}, Value: resourceMetadata(res)})
	}
	label, key := "resources", "resources"
	if templates {
		label = "resourceTemplates"
		key = "resourceTemplates"
	}
	result, err := ListPage(ctx, label, key, items, params, pageSize)
	return result, err, nil
}

// List handles resources/list.
func (r *ResourceRegistry) List(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	return r.list(ctx, params, false)
}

// ListTemplates handles resources/templates/list.
func (r *ResourceRegistry) ListTemplates(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	return r.list(ctx, params, true)
}

// ResourceContents normalises source text/binary/object/list/scalar results.
// Go byte slices represent bytes/bytearray/memoryview. Unsupported native Go
// values must be explicitly adapted by the resource handler.
func ResourceContents(uri, mime string, value any) ([]any, error) {
	switch v := value.(type) {
	case []byte:
		if mime == "" {
			mime = "application/octet-stream"
		}
		return []any{map[string]any{"uri": uri, "mimeType": mime, "blob": base64.StdEncoding.EncodeToString(v)}}, nil
	case string:
		if mime == "" {
			mime = "text/plain"
		}
		return []any{map[string]any{"uri": uri, "mimeType": mime, "text": v}}, nil
	case OrderedObject:
		entry := cloneJSON(v).(OrderedObject)
		if _, ok := lookup(entry, "uri"); !ok {
			entry = append(entry, Member{"uri", uri})
		}
		if _, ok := lookup(entry, "mimeType"); !ok && mime != "" {
			entry = append(entry, Member{"mimeType", mime})
		}
		return []any{entry}, nil
	case map[string]any:
		converted := orderedSchemaValue(v)
		if _, ok := converted.(OrderedObject); !ok {
			return nil, fmt.Errorf("unsupported resource mapping")
		}
		return ResourceContents(uri, mime, converted)
	case []any:
		out := []any{}
		for _, item := range v {
			entries, err := ResourceContents(uri, mime, item)
			if err != nil {
				return nil, err
			}
			out = append(out, entries...)
		}
		return out, nil
	default:
		text := pythonValueRepr(value)
		if text == "<unsupported>" {
			return nil, fmt.Errorf("unsupported resource value %T", value)
		}
		if mime == "" {
			mime = "text/plain"
		}
		return []any{map[string]any{"uri": uri, "mimeType": mime, "text": text}}, nil
	}
}

// Read resolves static resources before templates, with network-safe errors.
func (r *ResourceRegistry) Read(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	value := params["uri"]
	if value == nil || value == "" || value == false {
		return nil, &RPCError{Code: -32602, Message: "Missing 'uri' parameter"}, nil
	}
	uri, ok := value.(string)
	if !ok {
		return nil, nil, errors.New("resource URI must be a string")
	}
	r.mu.RLock()
	entry, found := r.static[uri]
	templates := append([]resourceEntry{}, r.templates...)
	r.mu.RUnlock()
	args := map[string]any{}
	if !found {
		for _, candidate := range templates {
			captures := candidate.pattern.FindStringSubmatch(uri)
			if captures == nil {
				continue
			}
			entry = candidate
			found = true
			for i, name := range candidate.names {
				args[name] = captures[i+1]
			}
			break
		}
	}
	if !found {
		return nil, &RPCError{Code: -32002, Message: "Resource not found", Data: map[string]any{"uri": uri}}, nil
	}
	output, err := invokeTool(ctx, entry.resource.Read, args)
	if errors.Is(err, ErrRequestCancelled) {
		return nil, nil, err
	}
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: RemoteSafeFailure(ctx, "Resource read failed", "Resource read failed: "+err.Error())}, nil
	}
	contents, err := ResourceContents(uri, entry.resource.MIME, output)
	if err != nil {
		return nil, nil, err
	}
	return map[string]any{"contents": contents}, nil, nil
}
func (r *ResourceRegistry) subscription(params map[string]any, remove bool) (any, *RPCError, error) {
	value := params["uri"]
	if value == nil || value == "" || value == false {
		return nil, &RPCError{Code: -32602, Message: "Missing 'uri' parameter"}, nil
	}
	uri, ok := value.(string)
	if !ok {
		return nil, nil, errors.New("resource URI must be a string")
	}
	session, _ := params["_session_id"].(string)
	r.mu.Lock()
	defer r.mu.Unlock()
	if remove {
		delete(r.subscriptions[session], uri)
		if session != "" && len(r.subscriptions[session]) == 0 {
			delete(r.subscriptions, session)
		}
	} else {
		if r.subscriptions == nil {
			r.subscriptions = make(map[string]map[string]bool)
		}
		if r.subscriptions[session] == nil {
			r.subscriptions[session] = make(map[string]bool)
		}
		r.subscriptions[session][uri] = true
	}
	return map[string]any{}, nil, nil
}

// Subscribe/Unsubscribe retain separate legacy-global and session registries.
func (r *ResourceRegistry) Subscribe(_ context.Context, p map[string]any) (any, *RPCError, error) {
	return r.subscription(p, false)
}
func (r *ResourceRegistry) Unsubscribe(_ context.Context, p map[string]any) (any, *RPCError, error) {
	return r.subscription(p, true)
}

// Subscribers returns sorted session IDs; an empty ID denotes legacy-global scope.
func (r *ResourceRegistry) Subscribers(uri string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []string{}
	for id, uris := range r.subscriptions {
		if uris[uri] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// DropSession clears subscription state when a transport expires/deletes a session.
func (r *ResourceRegistry) DropSession(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subscriptions, id)
}
