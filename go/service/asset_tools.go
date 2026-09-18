package service

import (
	_ "embed"
	"github.com/rcarmo/memento/go/umcp"
)

//go:embed asset_tools.json
var assetToolDefinitions []byte

// RegisterAssetReadProposalTools registers 16 implemented development tools,
// including accepted-asset retrieval. Configured surfaces are separate work.
func (j *Jobs) RegisterAssetReadProposalTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callStagingOrProposalTool, notify, assetToolDefinitions)
}
