package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/mayachain"
)

func newDASHFeeRateTool() mcp.Tool {
	return mcp.NewTool("dash_fee_rate",
		mcp.WithDescription("Get the recommended Dash fee rate in sat/vB from MayaChain inbound addresses."),
	)
}

func handleDASHFeeRate(mcClient *mayachain.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rate, err := mcClient.SatsPerByte(ctx, "DASH")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get DASH fee rate: %v", err)), nil
		}

		result := feeRateResult{
			Chain:       "Dash",
			Ticker:      "DASH",
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
