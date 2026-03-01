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

func newPolymarketCancelOrderTool() mcp.Tool {
	return mcp.NewTool("polymarket_cancel_order",
		mcp.WithDescription(
			"Cancel an open Polymarket order by its order ID. "+
				"Requires auth signature for CLOB authentication.",
		),
		mcp.WithString("order_id",
			mcp.Description("The order ID to cancel (from polymarket_open_orders or polymarket_submit_order)."),
			mcp.Required(),
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
	)
}

func handlePolymarketCancelOrder(pmClient *polymarket.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		orderID, _ := req.RequireString("order_id")
		authSig, _ := req.RequireString("auth_signature")
		address, _ := req.RequireString("address")
		authTS, _ := req.RequireString("auth_timestamp")

		if orderID == "" || authSig == "" || address == "" || authTS == "" {
			return mcp.NewToolResultError("all parameters are required"), nil
		}

		ts, err := strconv.ParseInt(authTS, 10, 64)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid auth_timestamp: %v", err)), nil
		}

		creds, err := pmClient.DeriveApiCreds(ctx, address, authSig, ts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to derive API credentials: %v", err)), nil
		}

		result, err := pmClient.CancelOrder(ctx, creds, orderID)
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
