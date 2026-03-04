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
			return nil, fmt.Errorf("defillama get chains TVL: %w", err)
		}

		// Sort by TVL descending
		sort.Slice(chains, func(i, j int) bool { return chains[i].TVL > chains[j].TVL })

		if chainFilter != "" {
			for _, c := range chains {
				if strings.EqualFold(c.Name, chainFilter) {
					return mcp.NewToolResultText(fmt.Sprintf("%s TVL: %s", c.Name, formatMarketCap(c.TVL))), nil
				}
			}
			return mcp.NewToolResultError(fmt.Sprintf("chain '%s' not found on DeFiLlama. Try exact name (e.g. 'Ethereum', 'BSC', 'Solana').", chainFilter)), nil
		}

		// Return top 15
		limit := 15
		if len(chains) < limit {
			limit = len(chains)
		}

		var sb strings.Builder
		sb.WriteString("Top DeFi Chains by TVL:\n")
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&sb, "%d. %s: %s\n", i+1, chains[i].Name, formatMarketCap(chains[i].TVL))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
