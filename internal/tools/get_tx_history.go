package tools

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/etherscan"
	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetTxHistoryTool() mcp.Tool {
	return mcp.NewTool("get_tx_history",
		mcp.WithDescription(
			"Get transaction history for an address on any EVM chain via Etherscan. "+
				"Returns recent transactions with sender, receiver, value, method name, "+
				"status, and gas used. Useful for reviewing past activity or checking "+
				"if a transaction was sent recently.",
		),
		mcp.WithString("chain",
			mcp.Description("EVM chain name."),
			mcp.DefaultString("Ethereum"),
		),
		mcp.WithString("address",
			mcp.Description("Wallet address to get history for. If omitted, uses the vault-derived address."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Number of transactions to return (default 10, max 50)."),
		),
	)
}

func handleGetTxHistory(store *vault.Store, esClient *etherscan.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chain := req.GetString("chain", "Ethereum")
		explicit := req.GetString("address", "")
		limit := int(req.GetFloat("limit", 10))
		if limit > 50 {
			limit = 50
		}
		if limit < 1 {
			limit = 1
		}

		if _, ok := etherscan.ChainIDs[chain]; !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unsupported chain: %q. Supported: Ethereum, BSC, Polygon, Arbitrum, Optimism, Base, Avalanche, Blast, Mantle, Zksync.", chain)), nil
		}

		if explicit != "" && !addressRegex.MatchString(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address format: %q. Expected 0x-prefixed 40-character hex string.", explicit)), nil
		}

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("could not resolve address: %v", err)), nil
		}

		txs, err := esClient.GetTxList(ctx, chain, addr, 1, limit)
		if err != nil {
			if strings.Contains(err.Error(), "API key required") || strings.Contains(err.Error(), "rate limit") {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return nil, fmt.Errorf("etherscan get tx list: %w", err)
		}

		ticker := evmclient.NativeTicker(chain)

		if len(txs) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No transactions found for %s on %s.", shortenMiddle(addr, 6, 4), chain)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Recent transactions for %s on %s (showing %d):\n\n", shortenMiddle(addr, 6, 4), chain, len(txs))

		for i, tx := range txs {
			truncHash := shortenMiddle(tx.Hash, 6, 4)
			truncFrom := shortenMiddle(tx.From, 6, 4)
			truncTo := ""
			if tx.To != "" {
				truncTo = shortenMiddle(tx.To, 6, 4)
			} else {
				truncTo = "Contract Creation"
			}

			// Status
			status := "Success"
			if tx.IsError == "1" {
				status = "Failed"
			}

			// Timestamp
			ts, _ := strconv.ParseInt(tx.TimeStamp, 10, 64)
			timeAgo := formatTimeAgo(time.Unix(ts, 0))

			// Value in native token
			valueNative := weiToETH(tx.Value)

			// Method
			method := ""
			if tx.FunctionName != "" {
				// Truncate long function signatures
				fn := tx.FunctionName
				if idx := strings.Index(fn, "("); idx > 0 {
					fn = fn[:idx] + "(...)"
				}
				method = " | " + fn
			}

			// Gas
			gasUsed := tx.GasUsed
			gasPrice := tx.GasPrice
			gasCostNative := ""
			if gasUsed != "" && gasPrice != "" {
				gasCostNative = calcGasCost(gasUsed, gasPrice)
			}

			fmt.Fprintf(&sb, "%d. %s | %s | %s\n", i+1, truncHash, timeAgo, status)
			fmt.Fprintf(&sb, "   %s -> %s | %s %s%s\n", truncFrom, truncTo, valueNative, ticker, method)
			if gasCostNative != "" {
				fmt.Fprintf(&sb, "   Gas: %s (%s %s)\n", gasUsed, gasCostNative, ticker)
			}
			sb.WriteString("\n")
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// shortenMiddle truncates a string by keeping head and tail characters with "..." in between.
func shortenMiddle(s string, head, tail int) string {
	if len(s) <= head+tail {
		return s
	}
	return s[:head] + "..." + s[len(s)-tail:]
}

func weiToETH(weiStr string) string {
	wei := new(big.Int)
	if _, ok := wei.SetString(weiStr, 10); !ok {
		return "0"
	}
	if wei.Sign() == 0 {
		return "0"
	}
	// Convert to float for display
	weiF := new(big.Float).SetInt(wei)
	ethF := new(big.Float).Quo(weiF, new(big.Float).SetFloat64(1e18))
	return ethF.Text('f', 6)
}

func calcGasCost(gasUsedStr, gasPriceStr string) string {
	gasUsed := new(big.Int)
	gasPrice := new(big.Int)
	gasUsed.SetString(gasUsedStr, 10)
	gasPrice.SetString(gasPriceStr, 10)

	cost := new(big.Int).Mul(gasUsed, gasPrice)
	costF := new(big.Float).SetInt(cost)
	ethF := new(big.Float).Quo(costF, new(big.Float).SetFloat64(1e18))
	return ethF.Text('f', 6)
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}
