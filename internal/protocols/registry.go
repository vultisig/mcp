package protocols

import (
	"math/big"

	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/protocols/aavev3"
	"github.com/vultisig/mcp/internal/vault"
)

// Protocol represents a DeFi protocol that can register MCP tools.
type Protocol interface {
	Name() string
	SupportsChain(chainID *big.Int) bool
	Register(s *server.MCPServer, store *vault.Store, ethClient *ethereum.Client, chainID *big.Int)
}

// all lists every protocol implementation.
var all = []Protocol{
	&aavev3.Protocol{},
}

// RegisterAll registers tools for every protocol that supports the connected chain.
func RegisterAll(s *server.MCPServer, store *vault.Store, ethClient *ethereum.Client, chainID *big.Int) {
	for _, p := range all {
		if p.SupportsChain(chainID) {
			p.Register(s, store, ethClient, chainID)
		}
	}
}
