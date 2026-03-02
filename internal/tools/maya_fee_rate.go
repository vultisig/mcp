package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/mayachain"
)

func newMayaFeeRateTool() mcp.Tool {
	return mcp.NewTool("maya_fee_rate",
		mcp.WithDescription(
			"Get the recommended fee rate for a MayaChain-supported chain in sat/vB. "+
				"Useful for chains not available on THORChain (e.g. ZEC, DASH).",
		),
		mcp.WithString("chain",
			mcp.Description("MayaChain chain identifier. One of: BTC, ETH, ARB, ZEC, DASH, THOR."),
			mcp.Required(),
			mcp.Enum("BTC", "ETH", "ARB", "ZEC", "DASH", "THOR"),
		),
	)
}

func handleMayaFeeRate(mcClient *mayachain.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chain, err := req.RequireString("chain")
		if err != nil {
			return mcp.NewToolResultError("missing chain parameter"), nil
		}

		rate, err := mcClient.SatsPerByte(ctx, chain)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get %s fee rate: %v", chain, err)), nil
		}

		result := feeRateResult{
			Chain:       chain,
			Ticker:      chain,
			FeeRate:     rate,
			FeeRateUnit: "sat/vB",
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal fee rate result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}
