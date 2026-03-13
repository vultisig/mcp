package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/pumpfun"
)

func newGetPumpfunTokenInfoTool() mcp.Tool {
	return mcp.NewTool("get_pumpfun_token_info",
		mcp.WithDescription(
			"Query pump.fun bonding curve state for a token. "+
				"Returns price, market cap, graduation progress, and reserves. "+
				"Reads on-chain data directly from Solana RPC.",
		),
		mcp.WithString("mint",
			mcp.Description("Token mint address (base58)."),
			mcp.Required(),
		),
	)
}

func handleGetPumpfunTokenInfo(pfClient *pumpfun.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mint, err := req.RequireString("mint")
		if err != nil {
			return mcp.NewToolResultError("missing mint parameter"), nil
		}

		info, err := pfClient.GetTokenInfo(ctx, mint)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get pump.fun token info: %v", err)), nil
		}

		status := "active"
		if info.Complete {
			status = "graduated"
		}

		result := map[string]any{
			"mint":                    info.Mint,
			"bonding_curve":           info.BondingCurveAddress,
			"virtual_token_reserves":  info.VirtualTokenReserves,
			"virtual_sol_reserves":    info.VirtualSolReserves,
			"real_token_reserves":     info.RealTokenReserves,
			"real_sol_reserves":       info.RealSolReserves,
			"token_total_supply":      info.TokenTotalSupply,
			"complete":                info.Complete,
			"status":                  status,
			"price_per_token_sol":     info.PricePerTokenSOL,
			"market_cap_sol":          info.MarketCapSOL,
			"graduation_progress_pct": info.GraduationProgress,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
