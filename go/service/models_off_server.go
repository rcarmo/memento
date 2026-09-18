package service

import (
	"errors"

	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/umcp"
)

// RegisterModelsOffServer composes the verified 29-tool development surface and
// memory_execute. It does not claim the missing configured 34-tool/admin/model
// surface or own transport/daemon lifecycle.
func (j *Jobs) RegisterModelsOffServer(server *umcp.Server, surface string, limits execute.Limits, notify ProposalNotifier) error {
	if server == nil {
		return errors.New("models-off server requires a uMCP server")
	}
	catalog, err := NewCatalog(CatalogConfig{Surface: surface})
	if err != nil {
		return err
	}
	endpoint, err := NewExecuteEndpoint(j, catalog, limits)
	if err != nil {
		return err
	}
	if err = j.RegisterAuditTools(server, notify); err != nil {
		return err
	}
	return endpoint.Register(server)
}
