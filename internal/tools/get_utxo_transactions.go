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

const (
	defaultTxLimit = 50
	maxTxLimit     = 100
)

func newGetUTXOTransactionsTool() mcp.Tool {
	return mcp.NewTool("get_utxo_transactions",
		mcp.WithDescription(
			"List recent transaction hashes for a UTXO chain address (Bitcoin, Litecoin, Dogecoin, etc.). "+
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
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of transaction hashes to return (default 50, max 100)."),
		),
	)
}

func handleGetUTXOTransactions(store *vault.Store, bcClient *blockchair.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainName, err := req.RequireString("chain")
		if err != nil {
			return mcp.NewToolResultError("missing chain parameter"), nil
		}

		if _, ok := blockchair.SupportedChains[chainName]; !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unsupported UTXO chain: %s", chainName)), nil
		}

		explicit := req.GetString("address", "")
		addr, err := resolve.ChainAddress(explicit, resolve.SessionIDFromCtx(ctx), store, chainName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		limit := int(req.GetFloat("limit", defaultTxLimit))
		if limit <= 0 {
			limit = defaultTxLimit
		}
		if limit > maxTxLimit {
			limit = maxTxLimit
		}

		dashboard, err := bcClient.GetAddressDashboard(ctx, chainName, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get %s transactions: %v", chainName, err)), nil
		}

		txs := dashboard.Transactions
		if len(txs) > limit {
			txs = txs[:limit]
		}

		result := map[string]any{
			"chain":             chainName,
			"address":           addr,
			"transaction_count": dashboard.Address.TransactionCount,
			"transactions":      txs,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal transactions result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
