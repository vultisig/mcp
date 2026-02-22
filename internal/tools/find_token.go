package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/coingecko"
	"github.com/vultisig/mcp/internal/types"
)

const maxTokenResults = 10

// platformToChain maps CoinGecko asset-platform IDs to Vultisig chain names.
var platformToChain = map[string]string{
	"ethereum":            "Ethereum",
	"binance-smart-chain": "BSC",
	"polygon-pos":         "Polygon",
	"arbitrum-one":        "Arbitrum",
	"optimistic-ethereum": "Optimism",
	"avalanche":           "Avalanche",
	"base":                "Base",
	"solana":              "Solana",
	"thorchain":           "THORChain",
	"cosmos":              "Cosmos",
	"osmosis":             "Osmosis",
	"terra-2":             "Terra",
	"terra":               "TerraClassic",
	"cronos":              "CronosChain",
	"tron":                "Tron",
	"ripple":              "Ripple",
	"sui":                 "Sui",
	"mantle":              "Mantle",
	"blast":               "Blast",
	"zksync":              "Zksync",
	"kujira":              "Kujira",
	"dydx":                "Dydx",
	"noble":               "Noble",
	"maya-protocol":       "MayaChain",
	"bitcoin":             "Bitcoin",
	"litecoin":            "Litecoin",
	"dogecoin":            "Dogecoin",
	"bitcoin-cash":        "Bitcoin-Cash",
	"dash":                "Dash",
	"zcash":               "Zcash",
}

func newFindTokenTool() mcp.Tool {
	return mcp.NewTool("find_token",
		mcp.WithDescription(
			"Search for tokens by ticker symbol, name, or contract address. "+
				"Returns token metadata and all known contract deployments across chains, "+
				"ranked by CoinGecko market cap rank. "+
				"Use this to discover token contract addresses for any supported chain.",
		),
		mcp.WithString("query",
			mcp.Description("Token ticker symbol (e.g. USDC), name (e.g. Uniswap), or contract address (e.g. 0xa0b86991...)"),
			mcp.Required(),
		),
	)
}

func handleFindToken(cgClient *coingecko.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}

		// Step 1: search CoinGecko.
		coins, err := cgClient.Search(ctx, query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("token search failed: %v", err)), nil
		}
		if len(coins) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("no tokens found for %q", query)), nil
		}

		// Limit to top N search hits.
		if len(coins) > maxTokenResults {
			coins = coins[:maxTokenResults]
		}

		// Step 2: enrich each hit with contract addresses (concurrently).
		type enriched struct {
			idx    int
			detail *coingecko.CoinDetail
		}

		var wg sync.WaitGroup
		ch := make(chan enriched, len(coins))

		for i, coin := range coins {
			wg.Add(1)
			go func(idx int, id string) {
				defer wg.Done()
				detail, err := cgClient.CoinDetail(ctx, id)
				if err != nil {
					return // skip failed enrichments
				}
				ch <- enriched{idx: idx, detail: detail}
			}(i, coin.ID)
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		details := make(map[int]*coingecko.CoinDetail)
		for e := range ch {
			details[e.idx] = e.detail
		}

		// Step 3: build structured response.
		var tokens []types.TokenInfo
		for i, coin := range coins {
			detail, ok := details[i]
			if !ok {
				continue
			}

			info := types.TokenInfo{
				ID:            coin.ID,
				Name:          detail.Name,
				Symbol:        detail.Symbol,
				MarketCapRank: coin.MarketCapRank,
				Logo:          detail.Image.Large,
			}

			for platform, pd := range detail.DetailPlatforms {
				if pd.ContractAddress == "" {
					continue
				}
				chain := platformToChain[platform]
				if chain == "" {
					chain = platform // use raw platform ID if no mapping
				}
				decimals := 0
				if pd.DecimalPlace != nil {
					decimals = *pd.DecimalPlace
				}
				info.Deployments = append(info.Deployments, types.TokenDeployment{
					Chain:           chain,
					ContractAddress: pd.ContractAddress,
					Decimals:        decimals,
				})
			}

			// Sort deployments by chain name for stable output.
			sort.Slice(info.Deployments, func(a, b int) bool {
				return info.Deployments[a].Chain < info.Deployments[b].Chain
			})

			tokens = append(tokens, info)
		}

		// Sort by market_cap_rank (lower = better; 0 = unranked → last).
		sort.Slice(tokens, func(i, j int) bool {
			ri, rj := tokens[i].MarketCapRank, tokens[j].MarketCapRank
			if ri == 0 {
				return false
			}
			if rj == 0 {
				return true
			}
			return ri < rj
		})

		result := &types.TokenSearchResult{Tokens: tokens}
		return result.ToToolResult()
	}
}
