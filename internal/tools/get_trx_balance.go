package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/tron"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetTRXBalanceTool() mcp.Tool {
	return mcp.NewTool("get_trx_balance",
		mcp.WithDescription(
			"Query the native TRX balance of a TRON address. "+
				"If no address is provided, derives it from the vault's ECDSA public key (requires set_vault_info first).",
		),
		mcp.WithString("address",
			mcp.Description("TRON address (base58, starts with T). Optional if vault info is set."),
		),
	)
}

func handleGetTRXBalance(store *vault.Store, tronClient *tron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")
		if explicit != "" {
			err := tron.ValidateAddress(explicit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid TRON address: %v", err)), nil
			}
		}

		addr, err := resolve.ChainAddress(explicit, resolve.ResolveVault(req, ctx, store), "Tron")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		info, err := tronClient.GetAccount(ctx, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get TRX balance: %v", err)), nil
		}

		result := map[string]any{
			"address":     addr,
			"balance_sun": info.Balance,
			"balance_trx": tron.FormatSUN(big.NewInt(info.Balance)),
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal trx balance result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
