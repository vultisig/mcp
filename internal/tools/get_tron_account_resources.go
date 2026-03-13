package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/tron"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetTronAccountResourcesTool() mcp.Tool {
	return mcp.NewTool("get_tron_account_resources",
		mcp.WithDescription(
			"Query the bandwidth and energy resources of a TRON account. "+
				"Returns available and used bandwidth/energy limits. "+
				"If no address is provided, derives it from the vault's ECDSA public key (requires set_vault_info first).",
		),
		mcp.WithString("address",
			mcp.Description("TRON address (base58, starts with T). Optional if vault info is set."),
		),
	)
}

func handleGetTronAccountResources(store *vault.Store, tronClient *tron.Client) server.ToolHandlerFunc {
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

		res, err := tronClient.GetAccountResource(ctx, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get TRON account resources: %v", err)), nil
		}

		result := map[string]any{
			"address":         addr,
			"bandwidth_used":  res.FreeNetUsed + res.NetUsed,
			"bandwidth_limit": res.FreeNetLimit + res.NetLimit,
			"energy_used":     res.EnergyUsed,
			"energy_limit":    res.EnergyLimit,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal tron resources result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
