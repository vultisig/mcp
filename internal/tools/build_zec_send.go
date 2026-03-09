package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildZECSendTool() mcp.Tool {
	return mcp.NewTool("build_zec_send",
		mcp.WithDescription(
			"Return Zcash transaction arguments for a send or swap. "+
				"Validates addresses and returns parameters for the client to build the transaction. "+
				"Fee is computed automatically using ZIP-317 on the client side. "+
				"Requires set_vault_info to be called first.",
		),
		mcp.WithString("to_address",
			mcp.Description("Recipient Zcash transparent address (t1... or t3...)"),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount to send in zatoshis (1 ZEC = 100,000,000 zatoshis, decimal string)"),
			mcp.Required(),
		),
		mcp.WithString("memo",
			mcp.Description("Optional OP_RETURN memo (e.g. MayaChain swap instruction, max 80 bytes)"),
		),
		mcp.WithString("address",
			mcp.Description("Sender Zcash address. Falls back to vault-derived address if omitted."),
		),
	)
}

func handleBuildZECSend(store *vault.Store, _ *blockchair.Client) server.ToolHandlerFunc {
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

		senderAddr, _, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.Zcash)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive Zcash address: %v", err)), nil
		}
		if explicitAddr != "" && explicitAddr != senderAddr {
			return mcp.NewToolResultError(fmt.Sprintf(
				"address %q does not match vault-derived address %q", explicitAddr, senderAddr)), nil
		}

		zcashChain := utxoChains["Zcash"]
		_, err = zcashChain.addressToPkScript(toAddress)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid to_address: %v", err)), nil
		}

		action := "transfer"
		if memo != "" {
			action = "swap"
		}

		result := map[string]any{
			"chain":       "Zcash",
			"action":      action,
			"from":        senderAddr,
			"to":          toAddress,
			"amount":      amount,
			"memo":        memo,
			"fee_note":    "fee computed automatically via ZIP-317 by the client",
			"tx_encoding": "zcash_v4",
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
