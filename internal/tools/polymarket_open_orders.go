package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/polymarket"
)

func newPolymarketOpenOrdersTool() mcp.Tool {
	return mcp.NewTool("polymarket_open_orders",
		mcp.WithDescription(
			"List open orders on Polymarket for the authenticated user. "+
				"Requires auth signature for CLOB authentication. "+
				"Optionally filter by market.",
		),
		mcp.WithString("auth_signature",
			mcp.Description("EIP-712 auth signature for CLOB access (0x-prefixed hex)."),
			mcp.Required(),
		),
		mcp.WithString("address",
			mcp.Description("The maker's Polygon address (0x-prefixed)."),
			mcp.Required(),
		),
		mcp.WithString("auth_timestamp",
			mcp.Description("The timestamp used in the auth EIP-712 message."),
			mcp.Required(),
		),
		mcp.WithString("market",
			mcp.Description("Optional market condition ID to filter orders."),
		),
	)
}

func handlePolymarketOpenOrders(pmClient *polymarket.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		authSig, _ := req.RequireString("auth_signature")
		address, _ := req.RequireString("address")
		authTS, _ := req.RequireString("auth_timestamp")
		market := req.GetString("market", "")

		if authSig == "" || address == "" || authTS == "" {
			return mcp.NewToolResultError("auth_signature, address, and auth_timestamp are required"), nil
		}

		ts, err := strconv.ParseInt(authTS, 10, 64)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid auth_timestamp: %v", err)), nil
		}

		creds, err := pmClient.DeriveApiCreds(ctx, address, authSig, ts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to derive API credentials: %v", err)), nil
		}

		orders, err := pmClient.GetOpenOrders(ctx, creds, market)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get open orders: %v", err)), nil
		}
		if len(orders) == 0 {
			return mcp.NewToolResultText("No open orders found."), nil
		}

		data, err := json.Marshal(orders)
		if err != nil {
			return nil, fmt.Errorf("marshal open orders: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
