package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/gaia"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetATOMBalanceTool() mcp.Tool {
	return mcp.NewTool("get_atom_balance",
		mcp.WithDescription(
			"Query the native ATOM balance of a Cosmos Hub (Gaia) address. "+
				"If no address is provided, derives it from the vault's ECDSA public key Accepts inline vault keys (ecdsa_public_key, eddsa_public_key, chain_code) or falls back to set_vault_info session state.",
		),
		mcp.WithString("address",
			mcp.Description("Cosmos address (bech32, cosmos1...). Optional if vault info is set."),
		),
	)
}

func handleGetATOMBalance(store *vault.Store, gaiaClient *gaia.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")
		if explicit != "" {
			err := gaia.ValidateAddress(explicit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid Cosmos address: %v", err)), nil
			}
		}

		addr, err := resolve.ChainAddress(explicit, resolve.ResolveVault(ctx, req, store), "Cosmos")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		balanceStr, err := gaiaClient.GetBalance(ctx, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get ATOM balance: %v", err)), nil
		}

		uatom, ok := new(big.Int).SetString(balanceStr, 10)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("invalid balance value: %q", balanceStr)), nil
		}

		result := map[string]any{
			"address":       addr,
			"balance_uatom": balanceStr,
			"balance_atom":  gaia.FormatUATOM(uatom),
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal balance result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
