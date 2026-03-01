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

func newPolymarketSubmitOrderTool() mcp.Tool {
	return mcp.NewTool("polymarket_submit_order",
		mcp.WithDescription(
			"Submit a signed Polymarket order to the CLOB. "+
				"Requires the signed order from polymarket_build_order + sign_typed_data. "+
				"Derives ephemeral API credentials from the auth signature (never stored). "+
				"Pass order_ref from build_order to avoid manual data threading (recommended). "+
				"Returns order ID and fill status.",
		),
		mcp.WithString("order_ref",
			mcp.Description("Order reference from polymarket_build_order. If provided, auth_timestamp and order_params are retrieved server-side (preferred)."),
		),
		mcp.WithString("order_signature",
			mcp.Description("The EIP-712 signature of the order payload (0x-prefixed hex)."),
			mcp.Required(),
		),
		mcp.WithString("auth_signature",
			mcp.Description("The EIP-712 signature of the auth payload (0x-prefixed hex)."),
			mcp.Required(),
		),
		mcp.WithString("address",
			mcp.Description("The maker's Polygon address (0x-prefixed)."),
			mcp.Required(),
		),
		mcp.WithString("auth_timestamp",
			mcp.Description("The timestamp used in the auth EIP-712 message. Optional if order_ref is provided."),
		),
		mcp.WithString("order_params",
			mcp.Description("JSON string of the order message fields from polymarket_build_order. Optional if order_ref is provided."),
		),
		mcp.WithString("order_type",
			mcp.Description("Order type: GTC, GTD, FOK, FAK. Optional if order_ref is provided."),
		),
	)
}

func handlePolymarketSubmitOrder(pmClient *polymarket.Client, orderStore *polymarket.OrderStore) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		orderSig, _ := req.RequireString("order_signature")
		authSig, _ := req.RequireString("auth_signature")
		address, _ := req.RequireString("address")

		if orderSig == "" || authSig == "" || address == "" {
			return mcp.NewToolResultError("order_signature, auth_signature, and address are required"), nil
		}

		var authTS string
		var orderParamsStr string
		var orderType string

		// Try to retrieve stored build result: by order_ref first, then by address
		var stored *polymarket.BuildOrderResult
		if ref := req.GetString("order_ref", ""); ref != "" {
			stored, _ = orderStore.Get(ref)
		}
		if stored == nil {
			// Fallback: look up by maker address (always available)
			stored, _ = orderStore.GetByAddress(address)
		}

		if stored != nil {
			if ts, ok := stored.ClobParams["auth_timestamp"].(string); ok {
				authTS = ts
			}
			if ot, ok := stored.ClobParams["order_type"].(string); ok {
				orderType = ot
			}
			msgBytes, err := json.Marshal(stored.OrderEIP712.Message)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to serialize stored order params: %v", err)), nil
			}
			orderParamsStr = string(msgBytes)
		} else {
			// No stored result — use LLM-provided values
			authTS = req.GetString("auth_timestamp", "")
			orderParamsStr = req.GetString("order_params", "")
			orderType = req.GetString("order_type", "")
		}

		if authTS == "" || orderParamsStr == "" || orderType == "" {
			return mcp.NewToolResultError("no stored order found for this address and no manual params provided. Call polymarket_build_order first."), nil
		}

		// Parse auth timestamp
		ts, err := strconv.ParseInt(authTS, 10, 64)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid auth_timestamp: %v", err)), nil
		}

		// Derive ephemeral API credentials
		creds, err := pmClient.DeriveApiCreds(ctx, address, authSig, ts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to derive API credentials: %v", err)), nil
		}

		// Parse order params
		var orderMsg map[string]any
		if err := json.Unmarshal([]byte(orderParamsStr), &orderMsg); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid order_params JSON: %v", err)), nil
		}

		// Transform EIP-712 message fields to CLOB API format
		orderMsg["signature"] = orderSig

		if saltStr, ok := orderMsg["salt"].(string); ok {
			if saltInt, err := strconv.ParseInt(saltStr, 10, 64); err == nil {
				orderMsg["salt"] = saltInt
			}
		}

		// Handle side: can arrive as float64 (JSON number), string ("BUY"/"SELL"), or json.Number
		switch v := orderMsg["side"].(type) {
		case float64:
			if int(v) == 0 {
				orderMsg["side"] = "BUY"
			} else {
				orderMsg["side"] = "SELL"
			}
		case json.Number:
			if v.String() == "0" {
				orderMsg["side"] = "BUY"
			} else {
				orderMsg["side"] = "SELL"
			}
		case string:
			// Already "BUY"/"SELL", keep as-is
		}

		// Remove signatureType — CLOB API doesn't accept it
		delete(orderMsg, "signatureType")

		// Build CLOB order payload
		payload := map[string]any{
			"order":     orderMsg,
			"orderType": orderType,
			"owner":     creds.Key,
		}

		result, err := pmClient.SubmitOrder(ctx, creds, payload)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("order submission failed: %v", err)), nil
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal submit result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
