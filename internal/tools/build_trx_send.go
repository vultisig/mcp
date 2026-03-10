package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/tron"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildTRXSendTool() mcp.Tool {
	return mcp.NewTool("build_trx_send",
		mcp.WithDescription(
			"Prepare a native TRX transfer. "+
				"Returns the transaction parameters (owner, recipient, amount in SUN) "+
				"needed by the app to build and sign the transaction. "+
				"If no from address is provided, derives it from the vault's ECDSA public key (requires set_vault_info first).",
		),
		mcp.WithString("to",
			mcp.Description("Recipient TRON address (base58, starts with T)."),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount in SUN (1 TRX = 1,000,000 SUN, decimal string)."),
			mcp.Required(),
		),
		mcp.WithString("from",
			mcp.Description("Sender's TRON address (base58). Optional if vault info is set."),
		),
	)
}

func handleBuildTRXSend(store *vault.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toAddr, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError("missing to parameter"), nil
		}
		err = tron.ValidateAddress(toAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid recipient address: %v", err)), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount parameter"), nil
		}
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok || amount.Sign() <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %s", amountStr)), nil
		}

		explicit := req.GetString("from", "")
		if explicit != "" {
			err = tron.ValidateAddress(explicit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid sender address: %v", err)), nil
			}
		}

		fromAddr, err := resolve.ChainAddress(explicit, resolve.SessionIDFromCtx(ctx), store, "Tron")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result := map[string]any{
			"chain":         "Tron",
			"action":        "transfer",
			"signing_mode":  "ecdsa_secp256k1",
			"owner_address": fromAddr,
			"to_address":    toAddr,
			"amount_sun":    amountStr,
			"amount_trx":    tron.FormatSUN(amount),
			"ticker":        "TRX",
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
