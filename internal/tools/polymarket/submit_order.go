package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	pm "github.com/vultisig/mcp/internal/polymarket"
)

const maxAuthAge = 5 * time.Minute

func NewSubmitOrderTool() mcp.Tool {
	return mcp.NewTool("polymarket_submit_order",
		mcp.WithDescription(
			"INTERNAL — do NOT call directly. Used by backend auto-submit after signing. "+
				"Submits a signed Polymarket order to the CLOB.",
		),
		mcp.WithString("order_ref",
			mcp.Description("REQUIRED. The exact order_ref string returned by polymarket_build_order (e.g. 'ord_1740912030123456789'). MUST be copied verbatim — NEVER fabricate or invent an order_ref. Server uses it to retrieve all order data."),
		),
		mcp.WithString("order_signature",
			mcp.Description("The EIP-712 signature of the order payload (0x-prefixed hex)."),
			mcp.Required(),
		),
		mcp.WithString("auth_signature",
			mcp.Description("The EIP-712 signature of the auth payload (0x-prefixed hex). Optional if auth was cached."),
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

func HandleSubmitOrder(pmClient *pm.Client, orderStore *pm.OrderStore, authCache *pm.AuthCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		orderSig, _ := req.RequireString("order_signature")
		authSig := req.GetString("auth_signature", "")
		address, _ := req.RequireString("address")

		if orderSig == "" || address == "" {
			return mcp.NewToolResultError("order_signature and address are required"), nil
		}
		if len(address) < 10 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s (expected 0x-prefixed hex)", address)), nil
		}

		var orderParamsStr string
		var orderType string

		// Try to retrieve stored build result: by order_ref first, then by address
		var stored *pm.BuildOrderResult
		reqRef := req.GetString("order_ref", "")
		if reqRef != "" {
			stored, _ = orderStore.Get(reqRef)
		}
		if stored == nil {
			stored, _ = orderStore.GetByAddress(address)
		}
		log.Printf("[submit_order] order_ref=%q stored=%v auth_sig_provided=%v", reqRef, stored != nil, authSig != "")

		if stored != nil {
			if ot, ok := stored.ClobParams["order_type"].(string); ok {
				orderType = ot
			}
			msgBytes, err := json.Marshal(stored.OrderEIP712.Message)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to serialize stored order params: %v", err)), nil
			}
			orderParamsStr = string(msgBytes)
		} else {
			orderParamsStr = req.GetString("order_params", "")
			orderType = req.GetString("order_type", "")
		}

		if orderParamsStr == "" || orderType == "" {
			return mcp.NewToolResultError("no stored order found for this address and no manual params provided. Call polymarket_build_order first."), nil
		}

		// Resolve API credentials: cached or derive from auth signature
		var creds *pm.ApiCreds
		if cached, ok := authCache.Get(address); ok && authSig == "" {
			// Reuse cached credentials — no auth signing needed
			log.Printf("[submit_order] using cached API creds for %s...%s", address[:6], address[len(address)-4:])
			creds = cached
		} else if authSig != "" {
			// Derive from auth signature
			var authTS string
			if stored != nil {
				if ts, ok := stored.ClobParams["auth_timestamp"].(string); ok {
					authTS = ts
				}
			}
			if authTS == "" {
				authTS = req.GetString("auth_timestamp", "")
			}
			if authTS == "" {
				return mcp.NewToolResultError("auth_signature provided but no auth_timestamp found. Call polymarket_build_order first."), nil
			}

			ts, err := strconv.ParseInt(authTS, 10, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid auth_timestamp: %v", err)), nil
			}

			age := time.Since(time.Unix(ts, 0))
			if age > maxAuthAge {
				return mcp.NewToolResultError(fmt.Sprintf(
					"auth_timestamp is %s old (max %s). Call polymarket_build_order again to get a fresh timestamp and re-sign.",
					age.Round(time.Second), maxAuthAge)), nil
			}

			var err2 error
			creds, err2 = pmClient.DeriveApiCreds(ctx, address, authSig, ts)
			if err2 != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to derive API credentials: %v", err2)), nil
			}

			// Cache creds for future orders
			authCache.Put(address, creds)
			log.Printf("[submit_order] cached API creds for %s...%s", address[:6], address[len(address)-4:])
		} else {
			return mcp.NewToolResultError("no auth_signature provided and no cached credentials found. Call polymarket_build_order to get a fresh auth payload."), nil
		}

		// Parse order params
		var orderMsg map[string]any
		if err := json.Unmarshal([]byte(orderParamsStr), &orderMsg); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid order_params JSON: %v", err)), nil
		}

		// Add the order signature
		orderMsg["signature"] = orderSig

		// Ensure salt is a number (JSON marshal may have stringified it)
		if saltStr, ok := orderMsg["salt"].(string); ok {
			if saltInt, err := strconv.ParseInt(saltStr, 10, 64); err == nil {
				orderMsg["salt"] = saltInt
			}
		}

		// Convert side from EIP-712 integer (0/1) to CLOB API string ("BUY"/"SELL").
		// The EIP-712 message uses uint8 for signing, but POST /order expects a string.
		switch v := orderMsg["side"].(type) {
		case float64:
			if int(v) == 0 {
				orderMsg["side"] = "BUY"
			} else {
				orderMsg["side"] = "SELL"
			}
		case int:
			if v == 0 {
				orderMsg["side"] = "BUY"
			} else {
				orderMsg["side"] = "SELL"
			}
		case string:
			// Already "BUY"/"SELL" — keep as-is
		}

		log.Printf("[submit_order] submitting orderType=%s side=%v tokenId=%v", orderType, orderMsg["side"], orderMsg["tokenId"])

		// Build CLOB order payload
		payload := map[string]any{
			"order":     orderMsg,
			"orderType": orderType,
			"owner":     creds.Key,
		}

		// Include negRisk flag for multi-outcome markets (required by CLOB)
		if stored != nil {
			if negRisk, ok := stored.ClobParams["neg_risk"].(bool); ok && negRisk {
				payload["negRisk"] = true
			}
		}

		// Notify CLOB to re-check on-chain balance/allowance before submitting.
		// The CLOB caches this state; recent approval txns may not be reflected yet.
		// For BUY: refresh COLLATERAL (USDC.e) balance/allowance.
		// For SELL: refresh CONDITIONAL (outcome token) balance/allowance with token_id.
		if err := pmClient.UpdateBalanceAllowance(ctx, address, creds, 0, "COLLATERAL", ""); err != nil { // sigType 0 = EOA
			log.Printf("[submit_order] UpdateBalanceAllowance COLLATERAL failed: %v", err)
		}
		sideVal, _ := orderMsg["side"].(string)
		if sideVal == "SELL" {
			tokenID, _ := orderMsg["tokenId"].(string)
			if err := pmClient.UpdateBalanceAllowance(ctx, address, creds, 0, "CONDITIONAL", tokenID); err != nil {
				log.Printf("[submit_order] UpdateBalanceAllowance CONDITIONAL failed: %v", err)
			}
		}

		// Pass wallet address for POLY_ADDRESS L2 header (distinct from API key)
		result, err := pmClient.SubmitOrder(ctx, address, creds, payload)
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
