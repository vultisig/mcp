package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	pm "github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/toolmeta"
	"github.com/vultisig/mcp/internal/vault"
)

func NewOpenOrdersTool() mcp.Tool {
	return mcp.NewTool("polymarket_open_orders",
		toolmeta.WithMeta(map[string]any{
			"inject_address": "evm",
		}),
		mcp.WithDescription(
			"List open orders on Polymarket for the authenticated user. "+
				"If auth credentials were cached from a previous order, "+
				"only address is needed (or omit to use vault). Otherwise pass auth_signature + auth_timestamp. "+
				"Optionally filter by market.",
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
		mcp.WithString("market",
			mcp.Description("Optional market condition ID to filter orders."),
		),
	)
}

func HandleOpenOrders(pmClient *pm.Client, authCache *pm.AuthCache, store *vault.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")
		authSig := req.GetString("auth_signature", "")
		authTS := req.GetString("auth_timestamp", "")
		market := req.GetString("market", "")

		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}
		address, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
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

		// Build summary without hash/sensitive fields (asset_id, owner, maker_address)
		type orderSummary struct {
			ID           string `json:"id"`
			Side         string `json:"side"`
			Outcome      string `json:"outcome"`
			Price        string `json:"price"`
			OriginalSize string `json:"original_size"`
			SizeMatched  string `json:"size_matched"`
			Status       string `json:"status"`
			OrderType    string `json:"type"`
			CreatedAt    string `json:"created_at"`
		}

		summaries := make([]orderSummary, len(orders))
		for i, o := range orders {
			summaries[i] = orderSummary{
				ID:           o.ID,
				Side:         o.Side,
				Outcome:      o.Outcome,
				Price:        o.Price,
				OriginalSize: o.OriginalSize,
				SizeMatched:  o.SizeMatched,
				Status:       o.Status,
				OrderType:    o.OrderType,
				CreatedAt:    o.CreatedAt.String(),
			}
		}

		data, err := json.Marshal(summaries)
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
		log.Printf("[auth] using cached API creds for %s", shortAddr(address))
		return cached, nil
	}

	// Need auth signature to derive
	if authSig == "" {
		return nil, fmt.Errorf("no cached auth credentials for this address. Place an order first (polymarket_place_bet → sign) to cache auth, or provide auth_signature + auth_timestamp")
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
	log.Printf("[auth] cached API creds for %s", shortAddr(address))
	return creds, nil
}
