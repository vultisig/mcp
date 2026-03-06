package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/thorchain"
)

func newBTCFeeRateTool() mcp.Tool {
	return mcp.NewTool("btc_fee_rate",
		mcp.WithDescription("Get the recommended Bitcoin fee rate in sat/vB from THORChain inbound addresses."),
		WithCategory("fee", "bitcoin"),
	)
}

type feeRateResult struct {
	Chain       string `json:"chain"`
	Ticker      string `json:"ticker"`
	FeeRate     uint64 `json:"fee_rate"`
	FeeRateUnit string `json:"fee_rate_unit"`
}

func handleBTCFeeRate(tcClient *thorchain.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rate, err := tcClient.SatsPerByte(ctx, "BTC")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get BTC fee rate: %v", err)), nil
		}

		result := feeRateResult{
			Chain:       "Bitcoin",
			Ticker:      "BTC",
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
