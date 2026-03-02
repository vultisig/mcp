package polymarket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	pm "github.com/vultisig/mcp/internal/polymarket"
)

func NewPriceTool() mcp.Tool {
	return mcp.NewTool("polymarket_price",
		mcp.WithDescription(
			"Get the current price and midpoint for a Polymarket outcome token. "+
				"Price represents the probability of the outcome (e.g. 0.65 = 65% chance). "+
				"Use the CLOB token ID from polymarket_search or polymarket_market_info results. "+
				"Note: polymarket_search already returns outcome_prices — only call this if you need a fresh quote.",
		),
		mcp.WithString("token_id",
			mcp.Description("CLOB token ID for the outcome (from polymarket_search or polymarket_market_info results — never fabricate)."),
			mcp.Required(),
		),
	)
}

func HandlePrice(pmClient *pm.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tokenID, err := req.RequireString("token_id")
		if err != nil {
			return mcp.NewToolResultError("token_id parameter is required"), nil
		}

		price, err := pmClient.GetPrice(ctx, tokenID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("price fetch failed: %v", err)), nil
		}

		data, err := json.Marshal(price)
		if err != nil {
			return nil, fmt.Errorf("marshal price: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
