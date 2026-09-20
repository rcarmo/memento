package service

import (
	"errors"

	"github.com/rcarmo/memento/internal/execute"
	"github.com/rcarmo/memento/umcp"
)

// RegisterModelsOffServer uses the same catalogue as intelligent-tier startup.
// Disabled optional handlers remain callable and report their disabled state;
// discovery is selected by the configured surface, not by registration order.
func (j *Jobs) RegisterModelsOffServer(server *umcp.Server, surface string, limits execute.Limits, notify ProposalNotifier) error {
	return j.registerModelsOffServer(server, surface, limits, notify, nil)
}

func (j *Jobs) registerModelsOffServer(server *umcp.Server, surface string, limits execute.Limits, notify ProposalNotifier, inference NeedleRouteInference) error {
	if server == nil {
		return errors.New("models-off server requires a uMCP server")
	}
	catalogConfig := CatalogConfig{Surface: surface, RouteEnabled: inference != nil}
	catalog, err := NewCatalog(catalogConfig)
	if err != nil {
		return err
	}
	endpoint, err := NewExecuteEndpoint(j, catalog, limits)
	if err != nil {
		return err
	}
	answer := &AnswerEndpoint{Jobs: j}
	proposals := &ModelProposalEndpoint{Jobs: j}
	route := RouteEndpoint{Jobs: j, Router: inference, Execute: endpoint}
	return j.RegisterConfiguredServer(server, ConfiguredServerOptions{
		Catalog: catalogConfig, Limits: limits, Notify: notify,
		ModelHandlers: map[string]CatalogHandler{
			"memory_answer": answer.Call, "memory_route": route.Call,
			"memory_propose_freeform": proposals.Freeform, "memory_propose_update": proposals.Update,
		},
	})
}
