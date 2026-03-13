package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	addresscodec "github.com/xyield/xrpl-go/address-codec"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
	xrpclient "github.com/vultisig/mcp/internal/xrp"
)

func newGetXRPBalanceTool() mcp.Tool {
	return mcp.NewTool("get_xrp_balance",
		mcp.WithDescription(
			"Query the native XRP balance of an XRP Ledger address. "+
				"If no address is provided, derives it from the vault's ECDSA public key Accepts inline vault keys (ecdsa_public_key, eddsa_public_key, chain_code) or falls back to set_vault_info session state.",
		),
		mcp.WithString("address",
			mcp.Description("XRP address. Optional if vault info is set."),
		),
	)
}

func handleGetXRPBalance(store *vault.Store, xrpClient *xrpclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")
		if explicit != "" && !addresscodec.IsValidClassicAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid XRP address: %q", explicit)), nil
		}

		addr, err := resolve.ChainAddress(explicit, resolve.ResolveVault(ctx, req, store), "Ripple")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		info, err := xrpClient.GetAccountInfo(ctx, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get XRP balance: %v", err)), nil
		}

		drops, err := strconv.ParseUint(info.Balance, 10, 64)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse balance: %v", err)), nil
		}

		xrpWhole := float64(drops) / 1_000_000

		result := map[string]any{
			"address":       addr,
			"balance_drops": drops,
			"balance_xrp":   fmt.Sprintf("%.6f", xrpWhole),
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal xrp balance result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
