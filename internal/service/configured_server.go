package service

import (
	"context"
	"errors"

	"github.com/rcarmo/memento/internal/execute"
	"github.com/rcarmo/memento/umcp"
)

type ConfiguredServerOptions struct {
	Catalog       CatalogConfig
	Limits        execute.Limits
	Notify        ProposalNotifier
	ModelHandlers map[string]CatalogHandler
	ManagedAccess bool
}

func (j *Jobs) RegisterConfiguredServer(server *umcp.Server, options ConfiguredServerOptions) error {
	if server == nil {
		return errors.New("configured server requires a uMCP server")
	}
	catalog, err := NewCatalog(options.Catalog)
	if err != nil {
		return err
	}
	endpoint, err := NewExecuteEndpoint(j, catalog, options.Limits)
	if err != nil {
		return err
	}
	handlers := map[string]CatalogHandler{}
	for _, operation := range catalog.source.Operations {
		name := operation.Tool
		switch name {
		case "memory_answer", "memory_route", "memory_propose_freeform", "memory_propose_update":
			handlers[name] = options.ModelHandlers[name]
		case "memory_execute":
			handlers[name] = func(ctx context.Context, args map[string]any) (any, error) { return endpoint.Call(ctx, args) }
		default:
			handlers[name] = func(ctx context.Context, args map[string]any) (any, error) {
				return j.callStatusOrTool(ctx, name, args)
			}
		}
	}
	handlers["memory_help"] = func(ctx context.Context, args map[string]any) (any, error) {
		return j.callStatusOrTool(ctx, "memory_help", args)
	}
	handlers["memory_status"] = func(ctx context.Context, args map[string]any) (any, error) {
		return j.callStatusOrTool(ctx, "memory_status", args)
	}
	if err = catalog.Register(server, handlers, options.Notify); err != nil {
		return err
	}
	// Help and catalog resources must describe this exact registered surface,
	// including enabled model features, not the startup metadata defaults.
	if j.Controls.Metadata != nil {
		j.Controls.Metadata.Catalog = catalog
	}
	if options.ManagedAccess {
		return j.RegisterAccessTools(server)
	}
	return nil
}
