package polymarket

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
	pm "github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/toolmeta"
	"github.com/vultisig/mcp/internal/vault"
)

func NewBuildOrderTool() mcp.Tool {
	return mcp.NewTool("polymarket_build_order",
		toolmeta.WithMeta(map[string]any{
			"inject_address": "evm",
		}),
		mcp.WithDescription(
			"INTERNAL — do NOT call directly. Use polymarket_place_bet instead. "+
				"Low-level: builds EIP-712 typed data payloads for a Polymarket order.",
		),
		mcp.WithString("token_id",
			mcp.Description("CLOB token ID for the outcome to trade. Optional if event_slug + outcome are provided."),
		),
		mcp.WithString("event_slug",
			mcp.Description("Event slug from polymarket_search results. MUST come from actual search results — never fabricate or guess. Used with 'outcome' for server-side token resolution."),
		),
		mcp.WithString("outcome",
			mcp.Description("Outcome to bet on. For multi-outcome events: the candidate/option name (e.g. 'Marco Rubio'). For binary markets: 'Yes' or 'No'. Used with event_slug."),
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
			mcp.Description("Number of shares to buy/sell. Mutually exclusive with spend."),
		),
		mcp.WithString("spend",
			mcp.Description("Dollar amount (USDC) to spend on this order. Server calculates shares = spend / price. Mutually exclusive with amount. Preferred when user says 'bet $X'."),
		),
		mcp.WithString("order_type",
			mcp.Description("Order type: GTC (default), GTD, FOK, FAK."),
		),
		mcp.WithString("address",
			mcp.Description("Maker address (0x-prefixed). Optional if vault info is set."),
		),
	)
}

func HandleBuildOrder(pmClient *pm.Client, store *vault.Store, pool *evmclient.Pool, orderStore *pm.OrderStore, authCache *pm.AuthCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sideStr, _ := req.RequireString("side")
		price, _ := req.RequireString("price")
		orderType := req.GetString("order_type", "GTC")

		if sideStr == "" || price == "" {
			return mcp.NewToolResultError("side and price are required"), nil
		}

		// Resolve size from spend (dollars) or amount (shares)
		spendStr := req.GetString("spend", "")
		amountStr := req.GetString("amount", "")

		if spendStr != "" && amountStr != "" {
			return mcp.NewToolResultError("provide spend OR amount, not both"), nil
		}
		if spendStr == "" && amountStr == "" {
			return mcp.NewToolResultError("provide either spend (dollar amount) or amount (shares)"), nil
		}

		var amount string
		if spendStr != "" {
			spendF, err := strconv.ParseFloat(spendStr, 64)
			if err != nil || spendF <= 0 {
				return mcp.NewToolResultError("spend must be a positive number"), nil
			}
			priceF, err := strconv.ParseFloat(price, 64)
			if err != nil || priceF <= 0 {
				return mcp.NewToolResultError("price must be > 0 to calculate shares from spend"), nil
			}
			shares := spendF / priceF
			amount = strconv.FormatFloat(shares, 'f', 2, 64)
		} else {
			amount = amountStr
		}

		// Resolve token ID: prefer event_slug + outcome, fallback to raw token_id
		tokenID := req.GetString("token_id", "")
		eventSlug := req.GetString("event_slug", "")
		outcomeText := req.GetString("outcome", "")

		var resolved *pm.ResolvedToken
		if eventSlug != "" && outcomeText != "" {
			var err error
			resolved, err = pmClient.ResolveToken(ctx, eventSlug, outcomeText)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("token resolution failed: %v", err)), nil
			}
			tokenID = resolved.TokenID
		}

		if tokenID == "" {
			return mcp.NewToolResultError("either token_id or (event_slug + outcome) is required"), nil
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
		var side pm.Side
		switch strings.ToUpper(sideStr) {
		case "BUY":
			side = pm.Buy
		case "SELL":
			side = pm.Sell
		default:
			return mcp.NewToolResultError("side must be BUY or SELL"), nil
		}

		// Pre-checks: $1 minimum and balance
		priceF, _ := strconv.ParseFloat(price, 64)
		sizeF, _ := strconv.ParseFloat(amount, 64)
		cost := priceF * sizeF
		if cost < 1.0 {
			return mcp.NewToolResultError(fmt.Sprintf(
				"Order total $%.2f is below Polymarket's $1 minimum. Increase spend to at least $1.", cost)), nil
		}
		if side == pm.Buy {
			evmClient, _, err := pool.Get(ctx, "Polygon")
			if err == nil {
				tokenBal, err := evmClient.GetTokenBalance(ctx, pm.USDCeAddress, addr)
				if err == nil {
					balF, _ := strconv.ParseFloat(tokenBal.Balance, 64)
					if balF < cost {
						return mcp.NewToolResultError(fmt.Sprintf(
							"Insufficient USDC.e: have $%.2f, need $%.2f. Deposit or swap more USDC.e on Polygon.", balF, cost)), nil
					}
				}
			}
		}

		// Fetch market metadata
		tickSize, err := pmClient.GetTickSize(ctx, tokenID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(
				"Token ID %s is not tradable on the CLOB (market may be closed or have no liquidity). "+
					"Use polymarket_search to find active markets. Error: %v",
				tokenID, err)), nil
		}
		negRisk, err := pmClient.GetNegRisk(ctx, tokenID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get neg_risk: %v", err)), nil
		}
		feeRate, err := pmClient.GetFeeRate(ctx, tokenID)
		if err != nil {
			feeRate = "0"
		}

		params := pm.OrderParams{
			TokenID:    tokenID,
			Side:       side,
			Price:      price,
			Size:       amount,
			OrderType:  pm.OrderType(strings.ToUpper(orderType)),
			NegRisk:    negRisk,
			TickSize:   tickSize,
			FeeRateBps: feeRate,
		}

		result, err := pm.BuildOrder(addr, params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build order: %v", err)), nil
		}

		// Check if we have cached API creds for this address — skip auth signing
		if _, ok := authCache.Get(addr); ok {
			result.AuthEIP712 = pm.EIP712Payload{} // omit auth payload
			result.ClobParams["auth_cached"] = true
		}

		// Store result server-side with random ref to avoid collision
		refN, err := crand.Int(crand.Reader, new(big.Int).SetInt64(1<<62))
		if err != nil {
			return nil, fmt.Errorf("generate order ref: %w", err)
		}
		ref := fmt.Sprintf("ord_%s", refN.String())
		result.OrderRef = ref
		orderStore.Put(ref, addr, result)

		// Include resolution info if resolved server-side
		if resolved != nil {
			result.ClobParams["resolved_question"] = resolved.Question
			result.ClobParams["resolved_outcome"] = resolved.Outcome
			result.ClobParams["resolved_price"] = resolved.Price
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal build order result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
