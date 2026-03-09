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

func newDefiSearchYieldsTool() mcp.Tool {
	return mcp.NewTool("defi_search_yields",
		mcp.WithDescription(
			"Search DeFi yield opportunities across protocols and chains. "+
				"Returns top pools sorted by APY with TVL and protocol info. "+
				"Filter by chain name, token symbol, or protocol name. "+
				"Only returns pools with TVL above the minimum threshold (default $100k).",
		),
		mcp.WithString("chain",
			mcp.Description("Filter by chain (e.g. 'Ethereum', 'Arbitrum', 'Solana'). Case-insensitive."),
		),
		mcp.WithString("token",
			mcp.Description("Filter by token symbol in pool name (e.g. 'USDC', 'ETH', 'WBTC'). Case-insensitive."),
		),
		mcp.WithString("protocol",
			mcp.Description("Filter by protocol name (e.g. 'aave-v3', 'compound-v3', 'lido'). Case-insensitive."),
		),
		mcp.WithNumber("min_tvl",
			mcp.Description("Minimum pool TVL in USD. Default 100000. Set lower to see smaller pools."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return. Default 10, max 25."),
		),
	)
}

func handleDefiSearchYields(dlClient *defillama.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chain := strings.ToLower(strings.TrimSpace(req.GetString("chain", "")))
		token := strings.ToUpper(strings.TrimSpace(req.GetString("token", "")))
		protocol := strings.ToLower(strings.TrimSpace(req.GetString("protocol", "")))
		minTVL := req.GetFloat("min_tvl", 100000)
		limit := int(req.GetFloat("limit", 10))
		switch {
		case limit <= 0:
			limit = 10
		case limit > 25:
			limit = 25
		}

		pools, err := dlClient.GetYieldPools(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch yield pools from DeFiLlama: %v", err)), nil
		}

		var filtered []defillama.Pool
		for _, p := range pools {
			if p.TVLUsd < minTVL {
				continue
			}
			if chain != "" && !strings.EqualFold(p.Chain, chain) {
				continue
			}
			if token != "" && !strings.Contains(strings.ToUpper(p.Symbol), token) {
				continue
			}
			if protocol != "" && !strings.Contains(strings.ToLower(p.Project), protocol) {
				continue
			}
			filtered = append(filtered, p)
		}

		if len(filtered) == 0 {
			return mcp.NewToolResultError("no yield pools found matching your criteria. Try broader filters or lower min_tvl."), nil
		}

		// Sort by APY descending
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].APY > filtered[j].APY })

		if len(filtered) > limit {
			filtered = filtered[:limit]
		}

		// Build filter description
		var filters []string
		if chain != "" {
			filters = append(filters, "chain="+chain)
		}
		if token != "" {
			filters = append(filters, "token="+token)
		}
		if protocol != "" {
			filters = append(filters, "protocol="+protocol)
		}
		filterStr := ""
		if len(filters) > 0 {
			filterStr = " matching " + strings.Join(filters, ", ")
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d yield pools%s:\n\n", len(filtered), filterStr)

		for i, p := range filtered {
			apyDetail := fmt.Sprintf("%.1f%%", p.APY)
			if p.APYBase > 0 || p.APYReward > 0 {
				parts := make([]string, 0, 2)
				if p.APYBase > 0 {
					parts = append(parts, fmt.Sprintf("base: %.1f%%", p.APYBase))
				}
				if p.APYReward > 0 {
					parts = append(parts, fmt.Sprintf("reward: %.1f%%", p.APYReward))
				}
				apyDetail = fmt.Sprintf("%.1f%% (%s)", p.APY, strings.Join(parts, ", "))
			}

			line := fmt.Sprintf("%d. %s | %s | %s | APY: %s | TVL: %s",
				i+1, p.Project, p.Symbol, p.Chain, apyDetail, formatMarketCap(p.TVLUsd))

			if p.ILRisk == "yes" {
				line += " | IL Risk"
			}
			fmt.Fprintln(&sb, line)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
