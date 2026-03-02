package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	pm "github.com/vultisig/mcp/internal/polymarket"
)

func NewOpenOrdersTool() mcp.Tool {
	return mcp.NewTool("polymarket_open_orders",
		mcp.WithDescription(
			"List open orders on Polymarket for the authenticated user. "+
				"If auth credentials were cached from a previous build_order/submit_order, "+
				"only address is needed. Otherwise pass auth_signature + auth_timestamp. "+
				"Optionally filter by market.",
		),
		mcp.WithString("address",
			mcp.Description("The maker's Polygon address (0x-prefixed)."),
			mcp.Required(),
		),
		mcp.WithString("auth_signature",
			mcp.Description("EIP-712 auth signature for CLOB access (0x-prefixed hex). Optional if auth was cached from a prior order."),
		),
		mcp.WithString("auth_timestamp",
			mcp.Description("The timestamp used in the auth EIP-712 message. Required if auth_signature is provided."),
		),
		mcp.WithString("market",
			mcp.Description("Optional market condition ID to filter orders."),
		),
	)
}

func HandleOpenOrders(pmClient *pm.Client, authCache *pm.AuthCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		address, _ := req.RequireString("address")
		authSig := req.GetString("auth_signature", "")
		authTS := req.GetString("auth_timestamp", "")
		market := req.GetString("market", "")

		if address == "" {
			return mcp.NewToolResultError("address is required"), nil
		}

		// Resolve API credentials: cached first, then derive
		creds, err := resolveAuthCreds(ctx, pmClient, authCache, address, authSig, authTS)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		orders, err := pmClient.GetOpenOrders(ctx, address, creds, market)
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

// resolveAuthCreds returns cached API credentials or derives them from a fresh signature.
func resolveAuthCreds(ctx context.Context, pmClient *pm.Client, authCache *pm.AuthCache, address, authSig, authTS string) (*pm.ApiCreds, error) {
	// Try cache first
	if cached, ok := authCache.Get(address); ok {
		log.Printf("[auth] using cached API creds for %s", address)
		return cached, nil
	}

	// Need auth signature to derive
	if authSig == "" {
		return nil, fmt.Errorf("no cached auth credentials for this address. Place an order first (polymarket_build_order → sign → submit) to cache auth, or provide auth_signature + auth_timestamp")
	}
	if authTS == "" {
		return nil, fmt.Errorf("auth_signature provided but auth_timestamp is missing")
	}

	ts, err := strconv.ParseInt(authTS, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid auth_timestamp: %v", err)
	}

	creds, err := pmClient.DeriveApiCreds(ctx, address, authSig, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to derive API credentials: %v", err)
	}

	// Cache for future use
	authCache.Put(address, creds)
	log.Printf("[auth] cached API creds for %s", address)
	return creds, nil
}
