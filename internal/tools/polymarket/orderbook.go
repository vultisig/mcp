package polymarket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	pm "github.com/vultisig/mcp/internal/polymarket"
)

func NewOrderbookTool() mcp.Tool {
	return mcp.NewTool("polymarket_orderbook",
		mcp.WithDescription(
			"Get the order book (bids and asks) for a Polymarket outcome token. "+
				"Shows available liquidity at each price level. "+
				"Use the CLOB token ID from polymarket_market_info.",
		),
		mcp.WithString("token_id",
			mcp.Description("CLOB token ID for the outcome (from polymarket_market_info clobTokenIds)."),
			mcp.Required(),
		),
	)
}

func HandleOrderbook(pmClient *pm.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tokenID, err := req.RequireString("token_id")
		if err != nil {
			return mcp.NewToolResultError("token_id parameter is required"), nil
		}

		ob, err := pmClient.GetOrderBook(ctx, tokenID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("orderbook fetch failed: %v", err)), nil
		}

		data, err := json.Marshal(ob)
		if err != nil {
			return nil, fmt.Errorf("marshal orderbook: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
