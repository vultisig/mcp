package tools

import (
	"math/big"

	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/coingecko"
	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/protocols"
	"github.com/vultisig/mcp/internal/vault"
)

// RegisterAll registers all MCP tools on the given server.
func RegisterAll(s *server.MCPServer, store *vault.Store, ethClient *ethereum.Client, chainID *big.Int, cgClient *coingecko.Client, bcClient *blockchair.Client) {
	s.AddTool(newSetVaultInfoTool(), handleSetVaultInfo(store))
	s.AddTool(newGetAddressTool(), handleGetAddress(store))
	s.AddTool(newGetETHBalanceTool(), handleGetETHBalance(store, ethClient))
	s.AddTool(newGetTokenBalanceTool(), handleGetTokenBalance(store, ethClient))
	s.AddTool(newFindTokenTool(), handleFindToken(cgClient))
	s.AddTool(newGetUTXOBalanceTool(), handleGetUTXOBalance(store, bcClient))
	s.AddTool(newGetUTXOTransactionsTool(), handleGetUTXOTransactions(store, bcClient))
	s.AddTool(newListUTXOsTool(), handleListUTXOs(store, bcClient))

	protocols.RegisterAll(s, store, ethClient, chainID)
}
