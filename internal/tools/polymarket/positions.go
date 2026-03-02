package polymarket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	pm "github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func NewPositionsTool() mcp.Tool {
	return mcp.NewTool("polymarket_positions",
		mcp.WithDescription(
			"Get Polymarket positions for an address. "+
				"Shows outcome tokens held, average entry price, current price, and P&L. "+
				"Returns currentValue and pnlPercent computed fields. "+
				"If no address is provided, derives it from the vault's ECDSA key. "+
				"Load 'polymarket-trading' skill for display format.",
		),
		mcp.WithString("address",
			mcp.Description("Polygon address (0x-prefixed). Optional if vault info is set."),
		),
	)
}

func HandlePositions(pmClient *pm.Client, store *vault.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		positions, err := pmClient.GetPositions(ctx, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("positions fetch failed: %v", err)), nil
		}
		if len(positions) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No Polymarket positions found for %s", addr)), nil
		}

		for i := range positions {
			size, _ := positions[i].Size.Float64()
			curPrice, _ := positions[i].CurPrice.Float64()
			avgPrice, _ := positions[i].AvgPrice.Float64()

			positions[i].CurrentValue = json.Number(fmt.Sprintf("%.2f", size*curPrice))

			if avgPrice > 0 {
				positions[i].PnlPercent = json.Number(fmt.Sprintf("%.2f", ((curPrice-avgPrice)/avgPrice)*100))
			} else {
				positions[i].PnlPercent = json.Number("0")
			}
		}

		data, err := json.Marshal(positions)
		if err != nil {
			return nil, fmt.Errorf("marshal positions: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
