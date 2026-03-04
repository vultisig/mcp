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

func newDefiGetProtocolTool() mcp.Tool {
	return mcp.NewTool("defi_get_protocol",
		mcp.WithDescription(
			"Get DeFi protocol information from DeFiLlama including total value locked (TVL), "+
				"chain breakdown, category, and recent TVL changes. "+
				"Use the protocol slug (e.g. 'aave', 'uniswap', 'lido', 'curve-dex'). "+
				"If unsure of the slug, try the protocol name in lowercase with hyphens.",
		),
		mcp.WithString("protocol",
			mcp.Description("Protocol slug (e.g. 'aave', 'uniswap', 'lido', 'curve-dex')."),
			mcp.Required(),
		),
	)
}

func handleDefiGetProtocol(dlClient *defillama.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug, err := req.RequireString("protocol")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		slug = strings.ToLower(strings.TrimSpace(slug))

		protocol, err := dlClient.GetProtocol(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("defillama get protocol: %w", err)
		}
		if protocol == nil {
			return mcp.NewToolResultError(
				fmt.Sprintf("protocol '%s' not found on DeFiLlama. Try lowercase slug with hyphens (e.g. 'curve-dex', 'aave-v3').", slug),
			), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Protocol: %s\n", protocol.Name)
		if protocol.Category != "" {
			fmt.Fprintf(&sb, "Category: %s\n", protocol.Category)
		}
		fmt.Fprintf(&sb, "TVL: %s\n", formatMarketCap(protocol.TotalTVL()))
		if protocol.Change1d != nil {
			fmt.Fprintf(&sb, "1d Change: %s\n", formatChange(*protocol.Change1d))
		}
		if protocol.Change7d != nil {
			fmt.Fprintf(&sb, "7d Change: %s\n", formatChange(*protocol.Change7d))
		}

		if len(protocol.CurrentChainTvls) > 0 {
			// Sort chains by TVL descending
			type chainEntry struct {
				name string
				tvl  float64
			}
			var chains []chainEntry
			for name, tvl := range protocol.CurrentChainTvls {
				// Skip aggregated keys like "borrowed", "staking", etc.
				if strings.Contains(name, "-") || name == "borrowed" || name == "staking" || name == "pool2" || name == "vesting" {
					continue
				}
				chains = append(chains, chainEntry{name, tvl})
			}
			sort.Slice(chains, func(i, j int) bool { return chains[i].tvl > chains[j].tvl })

			parts := make([]string, 0, len(chains))
			for _, c := range chains {
				parts = append(parts, fmt.Sprintf("%s (%s)", c.name, formatMarketCap(c.tvl)))
			}
			if len(parts) > 0 {
				sb.WriteString(fmt.Sprintf("Chains: %s\n", strings.Join(parts, ", ")))
			}
		} else if len(protocol.Chains) > 0 {
			sb.WriteString(fmt.Sprintf("Chains: %s\n", strings.Join(protocol.Chains, ", ")))
		}

		if protocol.URL != "" {
			sb.WriteString(fmt.Sprintf("Website: %s\n", protocol.URL))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
