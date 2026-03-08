package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/defillama"
)

func newDefiChainTVLTool() mcp.Tool {
	return mcp.NewTool("defi_chain_tvl",
		mcp.WithDescription(
			"Get total value locked (TVL) across DeFi chains. "+
				"Without parameters, returns top chains by TVL. "+
				"With a chain name, returns TVL for that specific chain.",
		),
		mcp.WithString("chain",
			mcp.Description("Specific chain name (e.g. 'Ethereum', 'Solana', 'Arbitrum'). If omitted, returns top 15 chains."),
		),
	)
}

func handleDefiChainTVL(dlClient *defillama.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainFilter := strings.TrimSpace(req.GetString("chain", ""))

		chains, err := dlClient.GetChainsTVL(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch chain TVL from DeFiLlama: %v", err)), nil
		}

		// Copy to avoid mutating cached slice
		sorted := make([]defillama.ChainTVL, len(chains))
		copy(sorted, chains)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].TVL > sorted[j].TVL })

		if chainFilter != "" {
			for _, c := range sorted {
				if strings.EqualFold(c.Name, chainFilter) {
					return mcp.NewToolResultText(fmt.Sprintf("%s TVL: %s", c.Name, formatMarketCap(c.TVL))), nil
				}
			}
			return mcp.NewToolResultError(fmt.Sprintf("chain '%s' not found on DeFiLlama. Try exact name (e.g. 'Ethereum', 'BSC', 'Solana').", chainFilter)), nil
		}

		// Return top 15
		limit := 15
		if len(sorted) < limit {
			limit = len(sorted)
		}

		var sb strings.Builder
		sb.WriteString("Top DeFi Chains by TVL:\n")
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&sb, "%d. %s: %s\n", i+1, sorted[i].Name, formatMarketCap(sorted[i].TVL))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
