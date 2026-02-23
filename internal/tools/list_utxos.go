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

const defaultUTXOLimit = 100

func newListUTXOsTool() mcp.Tool {
	return mcp.NewTool("list_utxos",
		mcp.WithDescription(
			"List unspent transaction outputs (UTXOs) for a UTXO chain address (Bitcoin, Litecoin, Dogecoin, etc.). "+
				"Useful for building transactions. "+
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
			mcp.Description("Maximum number of UTXOs to return (default 100)."),
		),
	)
}

func handleListUTXOs(store *vault.Store, bcClient *blockchair.Client) server.ToolHandlerFunc {
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

		limit := int(req.GetFloat("limit", defaultUTXOLimit))
		if limit <= 0 {
			limit = defaultUTXOLimit
		}

		dashboard, err := bcClient.GetAddressDashboard(ctx, chainName, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get %s UTXOs: %v", chainName, err)), nil
		}

		utxos := dashboard.UTXOs
		if len(utxos) > limit {
			utxos = utxos[:limit]
		}

		type utxoEntry struct {
			TxID           string `json:"txid"`
			Vout           int    `json:"vout"`
			Value          int64  `json:"value"`
			ValueFormatted string `json:"value_formatted"`
			BlockHeight    int64  `json:"block_height"`
		}

		entries := make([]utxoEntry, len(utxos))
		for i, u := range utxos {
			entries[i] = utxoEntry{
				TxID:           u.TransactionHash,
				Vout:           u.Index,
				Value:          u.Value,
				ValueFormatted: formatSatoshis(u.Value, info.Decimals),
				BlockHeight:    u.BlockID,
			}
		}

		result := map[string]any{
			"chain":   chainName,
			"address": addr,
			"utxos":   entries,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal utxos result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
