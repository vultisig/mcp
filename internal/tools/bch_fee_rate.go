package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/thorchain"
)

func newBCHFeeRateTool() mcp.Tool {
	return mcp.NewTool("bch_fee_rate",
		mcp.WithDescription("Get the recommended Bitcoin Cash fee rate in sat/vB from THORChain inbound addresses."),
	)
}

func handleBCHFeeRate(tcClient *thorchain.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rate, err := tcClient.SatsPerByte(ctx, "BCH")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get BCH fee rate: %v", err)), nil
		}

		result := feeRateResult{
			Chain:       "Bitcoin-Cash",
			Ticker:      "BCH",
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
