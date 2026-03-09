package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildBTCSendTool() mcp.Tool {
	return mcp.NewTool("build_btc_send",
		mcp.WithDescription(
			"Return Bitcoin transaction arguments for a send or swap. "+
				"Validates addresses and returns parameters for the client to fetch UTXOs and build the PSBT. "+
				"For THORChain swaps, provide the memo parameter. "+
				"Requires set_vault_info to be called first.",
		),
		mcp.WithString("to_address",
			mcp.Description("Recipient Bitcoin address (or THORChain vault address for swaps)"),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount to send in satoshis (decimal string)"),
			mcp.Required(),
		),
		mcp.WithNumber("fee_rate",
			mcp.Description("Fee rate in sat/vB (use btc_fee_rate tool to get recommended rate)"),
			mcp.Required(),
		),
		mcp.WithString("memo",
			mcp.Description("Optional OP_RETURN memo (e.g. THORChain swap instruction, max 80 bytes)"),
		),
		mcp.WithString("address",
			mcp.Description("Sender Bitcoin address. Falls back to vault-derived address if omitted."),
		),
	)
}

func handleBuildBTCSend(store *vault.Store, _ *blockchair.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toAddress, err := req.RequireString("to_address")
		if err != nil {
			return mcp.NewToolResultError("missing to_address"), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		amount, parseErr := strconv.ParseInt(amountStr, 10, 64)
		if parseErr != nil || amount <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %q", amountStr)), nil
		}

		feeRateFloat := req.GetFloat("fee_rate", 0)
		if math.IsNaN(feeRateFloat) || math.IsInf(feeRateFloat, 0) || feeRateFloat <= 0 {
			return mcp.NewToolResultError("fee_rate must be a valid positive number"), nil
		}
		feeRate := uint64(math.Ceil(feeRateFloat))

		memo := req.GetString("memo", "")
		if memo != "" && len(memo) > 80 {
			return mcp.NewToolResultError(fmt.Sprintf("memo too long: %d bytes (max 80)", len(memo))), nil
		}

		explicitAddr := req.GetString("address", "")

		sessionID := resolve.SessionIDFromCtx(ctx)
		v, ok := store.Get(sessionID)
		if !ok {
			return mcp.NewToolResultError("no vault info set — call set_vault_info first"), nil
		}

		senderAddr, _, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.Bitcoin)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive Bitcoin address: %v", err)), nil
		}
		if explicitAddr != "" && explicitAddr != senderAddr {
			return mcp.NewToolResultError(fmt.Sprintf(
				"address %q does not match vault-derived address %q", explicitAddr, senderAddr)), nil
		}

		btcChain := utxoChains["Bitcoin"]
		_, err = btcChain.addressToPkScript(toAddress)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid to_address: %v", err)), nil
		}

		action := "transfer"
		if memo != "" {
			action = "swap"
		}

		result := map[string]any{
			"chain":       "Bitcoin",
			"action":      action,
			"from":        senderAddr,
			"to":          toAddress,
			"amount":      amount,
			"fee_rate":    feeRate,
			"memo":        memo,
			"tx_encoding": "psbt",
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
