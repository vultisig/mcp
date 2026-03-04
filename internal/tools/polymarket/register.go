package polymarket

import (
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
	pm "github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/vault"
)

// RegisterAll adds all Polymarket MCP tools to the server.
func RegisterAll(s *server.MCPServer, store *vault.Store, pool *evmclient.Pool) {
	pmClient := pm.NewClient()
	orderStore := pm.NewOrderStore()
	authCache := pm.NewAuthCache()

	s.AddTool(NewSearchTool(), HandleSearch(pmClient))
	s.AddTool(NewMarketInfoTool(), HandleMarketInfo(pmClient))
	s.AddTool(NewOrderbookTool(), HandleOrderbook(pmClient))
	s.AddTool(NewPriceTool(), HandlePrice(pmClient))
	s.AddTool(NewPositionsTool(), HandlePositions(pmClient, store))
	s.AddTool(NewTradesTool(), HandleTrades(pmClient, store))
	s.AddTool(NewBuildOrderTool(), HandleBuildOrder(pmClient, store, pool, orderStore, authCache))
	s.AddTool(NewSubmitOrderTool(), HandleSubmitOrder(pmClient, orderStore, authCache))
	s.AddTool(NewCheckApprovalsTool(), HandleCheckApprovals(store, pool))
	s.AddTool(NewPlaceBetTool(), HandlePlaceBet(pmClient, store, pool, orderStore, authCache))
	s.AddTool(NewCancelOrderTool(), HandleCancelOrder(pmClient, authCache, store))
	s.AddTool(NewOpenOrdersTool(), HandleOpenOrders(pmClient, authCache, store))
}
