package polymarket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	pm "github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/toolmeta"
	"github.com/vultisig/mcp/internal/vault"
)

func NewCancelOrderTool() mcp.Tool {
	return mcp.NewTool("polymarket_cancel_order",
		toolmeta.WithMeta(map[string]any{
			"inject_address": "evm",
		}),
		mcp.WithDescription(
			"Cancel an open Polymarket order by its order ID. "+
				"If auth credentials were cached from a previous order, "+
				"only order_id is needed (address resolved from vault). Otherwise pass auth_signature + auth_timestamp.",
		),
		mcp.WithString("order_id",
			mcp.Description("The order ID to cancel (from polymarket_open_orders or polymarket_submit_order)."),
			mcp.Required(),
		),
		mcp.WithString("address",
			mcp.Description("The maker's Polygon address (0x-prefixed). Optional if vault info is set."),
		),
		mcp.WithString("auth_signature",
			mcp.Description("EIP-712 auth signature for CLOB access (0x-prefixed hex). Optional if auth was cached from a prior order."),
		),
		mcp.WithString("auth_timestamp",
			mcp.Description("The timestamp used in the auth EIP-712 message. Required if auth_signature is provided."),
		),
	)
}

func HandleCancelOrder(pmClient *pm.Client, authCache *pm.AuthCache, store *vault.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		orderID, _ := req.RequireString("order_id")
		explicit := req.GetString("address", "")
		authSig := req.GetString("auth_signature", "")
		authTS := req.GetString("auth_timestamp", "")

		if orderID == "" {
			return mcp.NewToolResultError("order_id is required"), nil
		}
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}
		address, err := resolve.EVMAddress(explicit, resolve.ResolveVault(req, ctx, store))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		creds, err := resolveAuthCreds(ctx, pmClient, authCache, address, authSig, authTS)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := pmClient.CancelOrder(ctx, address, creds, orderID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("cancel failed: %v", err)), nil
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal cancel result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
