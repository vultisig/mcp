package protocols

import (
	"math/big"

	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/recipes/sdk/evm"

	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/protocols/aavev3"
	"github.com/vultisig/mcp/internal/vault"
)

type Protocol interface {
	Name() string
	SupportsChain(chainID *big.Int) bool
	Register(s *server.MCPServer, store *vault.Store, ethClient *ethereum.Client, evmSDK *evm.SDK, chainID *big.Int)
}

var all = []Protocol{
	&aavev3.Protocol{},
}

func RegisterAll(s *server.MCPServer, store *vault.Store, ethClient *ethereum.Client, evmSDK *evm.SDK, chainID *big.Int) {
	for _, p := range all {
		if p.SupportsChain(chainID) {
			p.Register(s, store, ethClient, evmSDK, chainID)
		}
	}
}
