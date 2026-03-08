package polymarket

import (
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
	pm "github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/toolmeta"
	"github.com/vultisig/mcp/internal/vault"
)

// RegisterAll adds all Polymarket MCP tools to the server.
func RegisterAll(s *server.MCPServer, store *vault.Store, pool *evmclient.Pool) {
	pmClient := pm.NewClient()
	orderStore := pm.NewOrderStore()
	authCache := pm.NewAuthCache()

	toolmeta.Register(s, NewSearchTool(), HandleSearch(pmClient), "polymarket")
	toolmeta.Register(s, NewMarketInfoTool(), HandleMarketInfo(pmClient), "polymarket")
	toolmeta.Register(s, NewOrderbookTool(), HandleOrderbook(pmClient), "polymarket")
	toolmeta.Register(s, NewPriceTool(), HandlePrice(pmClient), "polymarket")
	toolmeta.Register(s, NewPositionsTool(), HandlePositions(pmClient, store), "polymarket")
	toolmeta.Register(s, NewTradesTool(), HandleTrades(pmClient, store), "polymarket")
	toolmeta.Register(s, NewBuildOrderTool(), HandleBuildOrder(pmClient, store, pool, orderStore, authCache), "polymarket", "send")
	toolmeta.Register(s, NewSubmitOrderTool(), HandleSubmitOrder(pmClient, orderStore, authCache), "polymarket", "send")
	toolmeta.Register(s, NewCheckApprovalsTool(), HandleCheckApprovals(store, pool), "polymarket", "contract")
	toolmeta.Register(s, NewPlaceBetTool(), HandlePlaceBet(pmClient, store, pool, orderStore, authCache), "polymarket", "send")
	toolmeta.Register(s, NewCancelOrderTool(), HandleCancelOrder(pmClient, authCache, store), "polymarket")
	toolmeta.Register(s, NewOpenOrdersTool(), HandleOpenOrders(pmClient, authCache, store), "polymarket")
}
