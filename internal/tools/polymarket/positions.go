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

		// Build summary without hash fields (asset, conditionId, market) to save LLM tokens
		type positionSummary struct {
			Title        string `json:"title"`
			EventSlug    string `json:"event_slug"`
			Outcome      string `json:"outcome"`
			Size         string `json:"size"`
			AvgPrice     string `json:"avg_price"`
			CurPrice     string `json:"cur_price"`
			CurrentValue string `json:"current_value"`
			RealizedPnl  string `json:"realized_pnl"`
			PnlPercent   string `json:"pnl_percent"`
		}

		summaries := make([]positionSummary, len(positions))
		for i, p := range positions {
			size, _ := p.Size.Float64()
			curPrice, _ := p.CurPrice.Float64()
			avgPrice, _ := p.AvgPrice.Float64()

			var pnlPct float64
			if avgPrice > 0 {
				pnlPct = ((curPrice - avgPrice) / avgPrice) * 100
			}

			summaries[i] = positionSummary{
				Title:        p.Title,
				EventSlug:    p.EventSlug,
				Outcome:      p.Outcome,
				Size:         p.Size.String(),
				AvgPrice:     p.AvgPrice.String(),
				CurPrice:     p.CurPrice.String(),
				CurrentValue: fmt.Sprintf("%.2f", size*curPrice),
				RealizedPnl:  p.RealizedPnl.String(),
				PnlPercent:   fmt.Sprintf("%.2f", pnlPct),
			}
		}

		data, err := json.Marshal(summaries)
		if err != nil {
			return nil, fmt.Errorf("marshal positions: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
