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
	"golang.org/x/sync/errgroup"

	evmclient "github.com/vultisig/mcp/internal/evm"
	pm "github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func NewPlaceBetTool() mcp.Tool {
	return mcp.NewTool("polymarket_place_bet",
		mcp.WithDescription(
			"Place a bet on a Polymarket event. Combines order building + signing payload construction into one call. "+
				"Returns an order summary and order_ref. The backend handles signing details and auto-submits after the user signs. "+
				"PREREQUISITE: Call polymarket_search first to get the event_slug — never fabricate or guess slugs. "+
				"Use 'spend' (dollar amount) when user says '$X'. Use 'amount' only when user says 'X shares'. "+
				"After calling this tool, emit a polymarket_sign_bet action with the returned order_ref.",
		),
		mcp.WithString("event_slug",
			mcp.Description("Event slug from polymarket_search results. MUST come from actual search results — never fabricate."),
			mcp.Required(),
		),
		mcp.WithString("outcome",
			mcp.Description("Outcome to bet on. For multi-outcome events: the candidate/option name. For binary markets: 'Yes' or 'No'."),
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
		mcp.WithString("spend",
			mcp.Description("Dollar amount (USDC) to spend. Server calculates shares = spend / price. Preferred when user says 'bet $X'. Mutually exclusive with amount."),
		),
		mcp.WithString("amount",
			mcp.Description("Number of shares. Mutually exclusive with spend."),
		),
		mcp.WithString("order_type",
			mcp.Description("Order type: FAK (default — partial fill OK, unfilled remainder cancelled), GTC (limit order on the book), FOK (all-or-nothing), GTD."),
		),
		mcp.WithString("address",
			mcp.Description("Maker address (0x-prefixed). Optional if vault info is set."),
		),
	)
}

// placeBetResult is the JSON structure returned to the LLM (and intercepted by the backend).
type placeBetResult struct {
	Status       string `json:"status"`
	OrderRef     string `json:"order_ref"`
	Summary      string `json:"summary"`
	CurrentPrice string `json:"current_price"`
	Shares       string `json:"shares"`
	Cost         string `json:"cost"`
	Fee          string `json:"fee"`
	Address      string `json:"address"`
	USDCeBalance string `json:"usdc_e_balance"`
	// SignAction is the full sign_typed_data action payload.
	// The backend strips this before the LLM sees it.
	SignAction map[string]any `json:"sign_action"`
}

func HandlePlaceBet(pmClient *pm.Client, store *vault.Store, pool *evmclient.Pool, orderStore *pm.OrderStore, authCache *pm.AuthCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		eventSlug, _ := req.RequireString("event_slug")
		outcomeText, _ := req.RequireString("outcome")
		sideStr, _ := req.RequireString("side")
		price, _ := req.RequireString("price")
		orderType := req.GetString("order_type", "FAK")

		if eventSlug == "" || outcomeText == "" || sideStr == "" || price == "" {
			return mcp.NewToolResultError("event_slug, outcome, side, and price are required"), nil
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
			amountF, err := strconv.ParseFloat(amountStr, 64)
			if err != nil || amountF <= 0 {
				return mcp.NewToolResultError("amount must be a positive number"), nil
			}
			amount = amountStr
		}

		// Resolve token
		resolved, err := pmClient.ResolveToken(ctx, eventSlug, outcomeText)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("token resolution failed: %v", err)), nil
		}
		tokenID := resolved.TokenID

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
		var usdceBalance string
		evmClient, _, err := pool.Get(ctx, "Polygon")
		if err == nil {
			tokenBal, err := evmClient.GetTokenBalance(ctx, pm.USDCeAddress, addr)
			if err == nil {
				usdceBalance = tokenBal.Balance
				balF, _ := strconv.ParseFloat(tokenBal.Balance, 64)
				if side == pm.Buy && balF < cost {
					return mcp.NewToolResultError(fmt.Sprintf(
						"Insufficient USDC.e: have $%.2f, need $%.2f. Deposit or swap more USDC.e on Polygon.", balF, cost)), nil
				}
			}
		}

		// Fetch market metadata (parallel)
		var tickSize, feeRate string
		var negRisk bool
		var tickErr, negErr error
		g, gctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			tickSize, tickErr = pmClient.GetTickSize(gctx, tokenID)
			return tickErr
		})
		g.Go(func() error {
			negRisk, negErr = pmClient.GetNegRisk(gctx, tokenID)
			return negErr
		})
		g.Go(func() error {
			feeRate, _ = pmClient.GetFeeRate(gctx, tokenID) // non-fatal
			return nil
		})
		if err := g.Wait(); err != nil {
			if tickErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf(
					"Token ID %s is not tradable on the CLOB (market may be closed or have no liquidity). "+
						"Use polymarket_search to find active markets. Error: %v",
					tokenID, tickErr)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("failed to get market metadata: %v", err)), nil
		}
		if feeRate == "" {
			feeRate = "0"
		}

		params := pm.OrderParams{
			TokenID:    tokenID,
			Side:       side,
			Price:      price,
			Size:       amount,
			Spend:      spendStr, // passed for FOK/FAK BUY — BuildOrder uses it as makerAmount base
			OrderType:  pm.OrderType(strings.ToUpper(orderType)),
			NegRisk:    negRisk,
			TickSize:   tickSize,
			FeeRateBps: feeRate,
		}

		buildResult, err := pm.BuildOrder(addr, params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build order: %v", err)), nil
		}

		// Check if we have cached API creds — skip auth signing
		authCached := false
		if _, ok := authCache.Get(addr); ok {
			buildResult.AuthEIP712 = pm.EIP712Payload{}
			buildResult.ClobParams["auth_cached"] = true
			authCached = true
		}

		// Store result server-side with random ref to avoid collision
		refN, err := crand.Int(crand.Reader, new(big.Int).SetInt64(1<<62))
		if err != nil {
			return nil, fmt.Errorf("generate order ref: %w", err)
		}
		ref := fmt.Sprintf("ord_%s", refN.String())
		buildResult.OrderRef = ref
		orderStore.Put(ref, addr, buildResult)

		// Include resolution info
		buildResult.ClobParams["resolved_question"] = resolved.Question
		buildResult.ClobParams["resolved_outcome"] = resolved.Outcome
		buildResult.ClobParams["resolved_price"] = resolved.Price

		// Construct the sign_typed_data action payload
		var payloads []map[string]any

		// Order payload (always present)
		orderPayload := map[string]any{
			"id":          "order",
			"primaryType": buildResult.OrderEIP712.PrimaryType,
			"domain":      buildResult.OrderEIP712.Domain,
			"types":       buildResult.OrderEIP712.Types,
			"message":     buildResult.OrderEIP712.Message,
			"chain":       "Polygon",
		}
		payloads = append(payloads, orderPayload)

		// Auth payload (only if not cached)
		if !authCached {
			authPayload := map[string]any{
				"id":          "auth",
				"primaryType": buildResult.AuthEIP712.PrimaryType,
				"domain":      buildResult.AuthEIP712.Domain,
				"types":       buildResult.AuthEIP712.Types,
				"message":     buildResult.AuthEIP712.Message,
				"chain":       "Polygon",
			}
			payloads = append(payloads, authPayload)
		}

		signAction := map[string]any{
			"type":  "sign_typed_data",
			"title": "Sign Polymarket Order",
			"params": map[string]any{
				"pm_order_ref": ref,
				"payloads":     payloads,
			},
		}

		sideLabel := "BUY"
		if side == pm.Sell {
			sideLabel = "SELL"
		}
		summary := fmt.Sprintf("%s %.2f shares of '%s' (%s) at $%s ($%.2f spend, ~%s fee)",
			sideLabel, sizeF, resolved.Question, resolved.Outcome, price, cost, buildResult.FeeEstimate)

		result := placeBetResult{
			Status:       "ready",
			OrderRef:     ref,
			Summary:      summary,
			CurrentPrice: resolved.Price,
			Shares:       amount,
			Cost:         fmt.Sprintf("%.2f", cost),
			Fee:          buildResult.FeeEstimate,
			Address:      addr,
			USDCeBalance: usdceBalance,
			SignAction:   signAction,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal place bet result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
