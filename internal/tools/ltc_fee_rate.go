package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/thorchain"
)

func newLTCFeeRateTool() mcp.Tool {
	return mcp.NewTool("ltc_fee_rate",
		mcp.WithDescription("Get the recommended Litecoin fee rate in sat/vB from THORChain inbound addresses."),
		WithCategory("fee"),
	)
}

func handleLTCFeeRate(tcClient *thorchain.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rate, err := tcClient.SatsPerByte(ctx, "LTC")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get LTC fee rate: %v", err)), nil
		}

		result := feeRateResult{
			Chain:       "Litecoin",
			Ticker:      "LTC",
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
