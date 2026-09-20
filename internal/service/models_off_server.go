package service

import (
	"errors"

	"github.com/rcarmo/memento/internal/execute"
	"github.com/rcarmo/memento/umcp"
)

// RegisterModelsOffServer composes the verified 29-tool development surface and
// memory_execute. Optional intelligent-tier tools are registered by their
// runtime owners only after model construction succeeds.
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
