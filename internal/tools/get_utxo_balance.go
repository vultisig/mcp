package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetUTXOBalanceTool() mcp.Tool {
	return mcp.NewTool("get_utxo_balance",
		mcp.WithDescription(
			"Query the balance and address stats for a UTXO chain address (Bitcoin, Litecoin, Dogecoin, etc.). "+
				"If no address is provided, derives it from the vault's ECDSA public key (requires set_vault_info first).",
		),
		mcp.WithString("chain",
			mcp.Description("UTXO blockchain network"),
			mcp.Required(),
			mcp.Enum(blockchair.ChainNames...),
		),
		mcp.WithString("address",
			mcp.Description("Address to query. Optional if vault info is set."),
		),
	)
}

func handleGetUTXOBalance(store *vault.Store, bcClient *blockchair.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainName, err := req.RequireString("chain")
		if err != nil {
			return mcp.NewToolResultError("missing chain parameter"), nil
		}

		info, ok := blockchair.SupportedChains[chainName]
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unsupported UTXO chain: %s", chainName)), nil
		}

		explicit := req.GetString("address", "")
		addr, err := resolve.ChainAddress(explicit, resolve.SessionIDFromCtx(ctx), store, chainName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dashboard, err := bcClient.GetAddressDashboard(ctx, chainName, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get %s balance: %v", chainName, err)), nil
		}

		balance := formatSatoshis(dashboard.Address.Balance, info.Decimals)

		result := map[string]any{
			"chain":                chainName,
			"address":              addr,
			"ticker":               info.Ticker,
			"balance":              balance,
			"balance_sats":         dashboard.Address.Balance,
			"balance_usd":          dashboard.Address.BalanceUSD,
			"transaction_count":    dashboard.Address.TransactionCount,
			"unspent_output_count": dashboard.Address.UnspentOutputCount,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal balance result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// formatSatoshis converts a satoshi-denominated integer to a human-readable
// decimal string with the given number of decimal places.
func formatSatoshis(sats int64, decimals int) string {
	divisor := int64(1)
	for i := 0; i < decimals; i++ {
		divisor *= 10
	}

	whole := sats / divisor
	frac := sats % divisor
	if frac < 0 {
		frac = -frac
	}

	return fmt.Sprintf("%d.%0*d", whole, decimals, frac)
}
