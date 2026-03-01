package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newPolymarketBuildOrderTool() mcp.Tool {
	return mcp.NewTool("polymarket_build_order",
		mcp.WithDescription(
			"Build EIP-712 typed data payloads for a Polymarket order. "+
				"Returns two payloads to sign: the order itself and an auth payload for CLOB access. "+
				"Both must be signed via sign_typed_data before calling polymarket_submit_order. "+
				"Automatically fetches tick_size, neg_risk, and fee_rate. "+
				"Load the 'polymarket-trading' skill for token ID selection, balance checks, and two-step signing flow.",
		),
		mcp.WithString("token_id",
			mcp.Description("CLOB token ID for the outcome to trade."),
			mcp.Required(),
		),
		mcp.WithString("side",
			mcp.Description("BUY or SELL."),
			mcp.Required(),
		),
		mcp.WithString("price",
			mcp.Description("Price per share (0.01 to 0.99). Represents probability."),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Number of shares to buy/sell."),
			mcp.Required(),
		),
		mcp.WithString("order_type",
			mcp.Description("Order type: GTC (default), GTD, FOK, FAK."),
		),
		mcp.WithString("address",
			mcp.Description("Maker address (0x-prefixed). Optional if vault info is set."),
		),
	)
}

func handlePolymarketBuildOrder(pmClient *polymarket.Client, store *vault.Store, orderStore *polymarket.OrderStore) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tokenID, _ := req.RequireString("token_id")
		sideStr, _ := req.RequireString("side")
		price, _ := req.RequireString("price")
		amount, _ := req.RequireString("amount")
		orderType := req.GetString("order_type", "GTC")

		if tokenID == "" || sideStr == "" || price == "" || amount == "" {
			return mcp.NewToolResultError("token_id, side, price, and amount are all required"), nil
		}

		// Resolve address
		explicit := req.GetString("address", "")
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}
		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Parse side
		var side polymarket.Side
		switch strings.ToUpper(sideStr) {
		case "BUY":
			side = polymarket.Buy
		case "SELL":
			side = polymarket.Sell
		default:
			return mcp.NewToolResultError("side must be BUY or SELL"), nil
		}

		// Fetch market metadata
		tickSize, err := pmClient.GetTickSize(ctx, tokenID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(
				"Token ID %s is not tradable on the CLOB (market may be closed or have no liquidity). "+
					"Use polymarket_market_info to find active markets with valid CLOB token IDs. Error: %v",
				tokenID, err)), nil
		}
		negRisk, err := pmClient.GetNegRisk(ctx, tokenID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get neg_risk: %v", err)), nil
		}

		params := polymarket.OrderParams{
			TokenID:    tokenID,
			Side:       side,
			Price:      price,
			Size:       amount,
			OrderType:  polymarket.OrderType(strings.ToUpper(orderType)),
			NegRisk:    negRisk,
			TickSize:   tickSize,
			FeeRateBps: "200", // Default, may vary per market
		}

		result, err := polymarket.BuildOrder(addr, params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build order: %v", err)), nil
		}

		// Store result server-side so submit_order can retrieve it by ref
		ref := fmt.Sprintf("ord_%d", time.Now().UnixNano())
		result.OrderRef = ref
		orderStore.Put(ref, result)

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal build order result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
