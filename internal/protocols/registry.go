package protocols

import (
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/vault"
)

// RegisterAll is a no-op placeholder. Protocol-specific tools have been
// replaced by skill files that teach agents to use generic EVM tools.
// Kept so callers don't need to change.
func RegisterAll(s *server.MCPServer, store *vault.Store, pool *evmclient.Pool) error {
	return nil
}
