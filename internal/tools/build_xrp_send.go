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
	addresscodec "github.com/xyield/xrpl-go/address-codec"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
	xrpclient "github.com/vultisig/mcp/internal/xrp"
)

func newBuildXRPSendTool() mcp.Tool {
	return mcp.NewTool("build_xrp_send",
		mcp.WithDescription(
			"Return XRP Ledger Payment transaction arguments for the client to build and sign. "+
				"Fetches sequence number, current ledger, and base fee for the client's reference. "+
				"For THORChain cross-chain swaps, provide the memo parameter. "+
				"Requires set_vault_info to be called first.",
		),
		mcp.WithString("to",
			mcp.Description("Recipient XRP address (or THORChain vault address for swaps)"),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount in drops (1 XRP = 1,000,000 drops, decimal string)"),
			mcp.Required(),
		),
		mcp.WithString("memo",
			mcp.Description("Optional THORChain swap memo (ASCII). Gets hex-encoded into the XRPL Memos field."),
		),
		mcp.WithString("from",
			mcp.Description("Sender XRP address. Falls back to vault-derived address if omitted."),
		),
	)
}

func handleBuildXRPSend(store *vault.Store, xrpClient *xrpclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toAddr, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError("missing to parameter"), nil
		}
		if !addresscodec.IsValidClassicAddress(toAddr) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid XRP address: %q", toAddr)), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount parameter"), nil
		}
		amountDrops, parseErr := strconv.ParseUint(amountStr, 10, 64)
		if parseErr != nil || amountDrops == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %q", amountStr)), nil
		}

		memo := req.GetString("memo", "")
		explicit := req.GetString("from", "")

		sessionID := resolve.SessionIDFromCtx(ctx)
		v, ok := store.Get(sessionID)
		if !ok {
			return mcp.NewToolResultError("no vault info set — call set_vault_info first"), nil
		}

		senderAddr, derivedPubKey, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.XRP)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive XRP address: %v", err)), nil
		}
		if explicit != "" {
			if !addresscodec.IsValidClassicAddress(explicit) {
				return mcp.NewToolResultError(fmt.Sprintf("invalid sender XRP address: %q", explicit)), nil
			}
			if explicit != senderAddr {
				return mcp.NewToolResultError(fmt.Sprintf(
					"explicit from address %q does not match vault-derived address %q", explicit, senderAddr)), nil
			}
		}

		info, err := xrpClient.GetAccountInfo(ctx, senderAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get account info: %v", err)), nil
		}

		currentLedger, err := xrpClient.GetCurrentLedger(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get current ledger: %v", err)), nil
		}

		baseFee, err := xrpClient.GetBaseFee(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get base fee: %v", err)), nil
		}

		feeDrops := baseFee
		if memo != "" {
			feeDrops = baseFee + 3
		}

		action := "transfer"
		if memo != "" {
			action = "swap"
		}

		result := map[string]any{
			"chain":                "Ripple",
			"action":               action,
			"transaction_type":     "Payment",
			"account":              senderAddr,
			"signing_pub_key":      derivedPubKey,
			"destination":          toAddr,
			"amount":               amountStr,
			"fee":                  strconv.FormatUint(feeDrops, 10),
			"sequence":             info.Sequence,
			"last_ledger_sequence": currentLedger + 100,
			"memo":                 memo,
			"signing_mode":         "ecdsa_secp256k1",
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
