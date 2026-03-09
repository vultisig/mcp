package protocols

import (
	"context"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/server"

	evmsdk "github.com/vultisig/recipes/sdk/evm"

	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/protocols/aavev3"
	"github.com/vultisig/mcp/internal/protocols/sky"
	"github.com/vultisig/mcp/internal/vault"
)

type Protocol interface {
	Name() string
	SupportsChain(chainID *big.Int) bool
	Register(s *server.MCPServer, store *vault.Store, ethClient *evmclient.Client, evmSDK *evmsdk.SDK, chainID *big.Int)
}

var all = []Protocol{
	&aavev3.Protocol{},
	&sky.Protocol{},
}

// RegisterAll extracts the Ethereum client from the pool and registers all
// protocol tools that support the connected chain.
func RegisterAll(s *server.MCPServer, store *vault.Store, pool *evmclient.Pool) error {
	ethClient, ethChainID, err := pool.Get(context.Background(), "Ethereum")
	if err != nil {
		return fmt.Errorf("connect to Ethereum: %w", err)
	}

	ethSDK := evmsdk.NewSDK(ethChainID, ethClient.ETH(), ethClient.RawRPC())

	for _, p := range all {
		if p.SupportsChain(ethChainID) {
			p.Register(s, store, ethClient, ethSDK, ethChainID)
		}
	}
	return nil
}
