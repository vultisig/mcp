package tools

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/coingecko"
)

// nativeCoinGeckoID maps uppercase ticker symbols to CoinGecko coin IDs.
var nativeCoinGeckoID = map[string]string{
	"ETH":   "ethereum",
	"BTC":   "bitcoin",
	"SOL":   "solana",
	"XRP":   "ripple",
	"BNB":   "binancecoin",
	"MATIC": "matic-network",
	"POL":   "matic-network",
	"AVAX":  "avalanche-2",
	"LTC":   "litecoin",
	"DOGE":  "dogecoin",
	"BCH":   "bitcoin-cash",
	"DASH":  "dash",
	"ZEC":   "zcash",
	"RUNE":  "thorchain",
	"MNT":   "mantle",
}

// chainToPlatform maps Vultisig chain names to CoinGecko asset-platform IDs.
// This is the inverse of platformToChain in search_token.go.
var chainToPlatform = map[string]string{
	"Ethereum":  "ethereum",
	"BSC":       "binance-smart-chain",
	"Polygon":   "polygon-pos",
	"Arbitrum":  "arbitrum-one",
	"Optimism":  "optimistic-ethereum",
	"Avalanche": "avalanche",
	"Base":      "base",
	"Solana":    "solana",
	"Mantle":    "mantle",
	"Blast":     "blast",
	"Zksync":    "zksync",
}

var evmAddressRE = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func newGetPriceTool() mcp.Tool {
	return mcp.NewTool("get_price",
		mcp.WithDescription(
			"Get the current USD price, 24h change, and market cap for a token. "+
				"Accepts a ticker symbol (ETH, BTC, USDC), CoinGecko coin ID (ethereum, bitcoin), "+
				"or an EVM contract address (requires chain parameter). "+
				"Optionally calculates the USD value of a given amount.",
		),
		mcp.WithString("token",
			mcp.Description("Token ticker symbol (e.g. ETH), CoinGecko ID (e.g. ethereum), or contract address (0x-prefixed, 40 hex chars)."),
			mcp.Required(),
		),
		mcp.WithString("chain",
			mcp.Description("EVM chain name. Required when token is a contract address. One of: "+strings.Join(chainPlatformNames(), ", ")),
		),
		mcp.WithString("amount",
			mcp.Description("Optional amount of the token to calculate USD value for (e.g. \"2.5\")."),
		),
	)
}

func handleGetPrice(cgClient *coingecko.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		token, err := req.RequireString("token")
		if err != nil {
			return mcp.NewToolResultError("token parameter is required"), nil
		}

		chain := req.GetString("chain", "")
		amountStr := req.GetString("amount", "")

		var pd *coingecko.PriceData
		var tokenName, tokenSymbol string

		switch {
		case evmAddressRE.MatchString(token):
			// Contract address — need chain param.
			if chain == "" {
				return mcp.NewToolResultError("chain is required when querying by contract address"), nil
			}
			platform, ok := chainToPlatform[chain]
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("unsupported chain %q for token price lookup", chain)), nil
			}
			pd, err = cgClient.GetTokenPrice(ctx, platform, token)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("token price lookup failed: %v", err)), nil
			}
			tokenSymbol = token[:8] + "..."
			tokenName = fmt.Sprintf("Token on %s", chain)

		default:
			// Try native ticker first (uppercase).
			upper := strings.ToUpper(token)
			if cgID, ok := nativeCoinGeckoID[upper]; ok {
				pd, err = cgClient.GetSimplePrice(ctx, cgID)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("price lookup failed: %v", err)), nil
				}
				tokenSymbol = upper
				tokenName = cgID
			} else {
				// Search CoinGecko to get proper name/symbol.
				coins, searchErr := cgClient.Search(ctx, token)
				if searchErr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("token search failed for %q: %v", token, searchErr)), nil
				}
				if len(coins) > 0 {
					pd, err = cgClient.GetSimplePrice(ctx, coins[0].ID)
					if err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("price lookup failed for %q: %v", coins[0].ID, err)), nil
					}
					tokenSymbol = strings.ToUpper(coins[0].Symbol)
					tokenName = coins[0].Name
				} else {
					// Last resort: try as direct CoinGecko ID.
					pd, err = cgClient.GetSimplePrice(ctx, strings.ToLower(token))
					if err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("price lookup failed for %q: %v", token, err)), nil
					}
					tokenSymbol = strings.ToUpper(token)
					tokenName = token
				}
			}
		}

		// Build response.
		resp := fmt.Sprintf("Token: %s\nName: %s\nPrice: %s\n24h Change: %s\nMarket Cap: %s",
			tokenSymbol,
			tokenName,
			formatPrice(pd.USD),
			formatChange(pd.USD24hChange),
			formatMarketCap(pd.USDMarketCap),
		)

		if amountStr != "" {
			amount, parseErr := strconv.ParseFloat(amountStr, 64)
			if parseErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid amount %q: %v", amountStr, parseErr)), nil
			}
			if amount < 0 {
				return mcp.NewToolResultError("amount must be non-negative"), nil
			}
			value := amount * pd.USD
			resp += fmt.Sprintf("\nAmount: %s\nValue: $%s", amountStr, formatUSD(value))
		}

		return mcp.NewToolResultText(resp), nil
	}
}

func chainPlatformNames() []string {
	names := make([]string, 0, len(chainToPlatform))
	for name := range chainToPlatform {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func formatPrice(usd float64) string {
	if usd == 0 {
		return "$0.00"
	}
	if usd < 0.01 {
		return fmt.Sprintf("$%.8f", usd)
	}
	return fmt.Sprintf("$%s", formatUSD(usd))
}

func formatUSD(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func formatChange(change float64) string {
	sign := "+"
	if change < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.2f%%", sign, change)
}

func formatMarketCap(cap float64) string {
	switch {
	case cap >= 1e12:
		return fmt.Sprintf("$%.1fT", cap/1e12)
	case cap >= 1e9:
		return fmt.Sprintf("$%.1fB", cap/1e9)
	case cap >= 1e6:
		return fmt.Sprintf("$%.1fM", cap/1e6)
	case cap >= 1e3:
		return fmt.Sprintf("$%.1fK", cap/1e3)
	case cap == 0:
		return "N/A"
	default:
		return fmt.Sprintf("$%.0f", cap)
	}
}
