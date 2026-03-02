package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/thorchain"
)

func newDOGEFeeRateTool() mcp.Tool {
	return mcp.NewTool("doge_fee_rate",
		mcp.WithDescription("Get the recommended Dogecoin fee rate in sat/vB from THORChain inbound addresses."),
	)
}

func handleDOGEFeeRate(tcClient *thorchain.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rate, err := tcClient.SatsPerByte(ctx, "DOGE")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get DOGE fee rate: %v", err)), nil
		}

		result := feeRateResult{
			Chain:       "Dogecoin",
			Ticker:      "DOGE",
			FeeRate:     rate,
			FeeRateUnit: "sat/vB",
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal fee rate result: %w", err)
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}
