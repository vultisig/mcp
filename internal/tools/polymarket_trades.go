package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newPolymarketTradesTool() mcp.Tool {
	return mcp.NewTool("polymarket_trades",
		mcp.WithDescription(
			"Get recent trade history for an address on Polymarket. "+
				"Shows executed trades with prices, sizes, and timestamps. "+
				"If no address is provided, derives it from the vault's ECDSA key.",
		),
		mcp.WithString("address",
			mcp.Description("Polygon address (0x-prefixed). Optional if vault info is set."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of trades to return. Default 20."),
		),
	)
}

func handlePolymarketTrades(pmClient *polymarket.Client, store *vault.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		limit := 20
		if v := req.GetFloat("limit", 0); v > 0 {
			limit = int(v)
		}

		trades, err := pmClient.GetTrades(ctx, addr, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("trades fetch failed: %v", err)), nil
		}
		if len(trades) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No Polymarket trades found for %s", addr)), nil
		}

		data, err := json.Marshal(trades)
		if err != nil {
			return nil, fmt.Errorf("marshal trades: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
