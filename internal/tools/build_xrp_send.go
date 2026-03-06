package tools

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"
	addresscodec "github.com/xyield/xrpl-go/address-codec"
	xrpgo "github.com/xyield/xrpl-go/binary-codec"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/vault"
	xrpclient "github.com/vultisig/mcp/internal/xrp"
)

func newBuildXRPSendTool() mcp.Tool {
	return mcp.NewTool("build_xrp_send",
		mcp.WithDescription(
			"Build an unsigned XRP Ledger Payment transaction. "+
				"Automatically fetches sequence number, current ledger, and base fee. "+
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
		WithCategory("send", "xrp"),
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
		amountDrops, err := strconv.ParseUint(amountStr, 10, 64)
		if err != nil || amountDrops == 0 {
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
					"explicit from address %q does not match vault-derived address %q — "+
						"SigningPubKey can only be derived from the current vault", explicit, senderAddr)), nil
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

		txBytes, err := buildXRPLPayment(
			senderAddr,
			toAddr,
			amountDrops,
			info.Sequence,
			feeDrops,
			currentLedger+100,
			derivedPubKey,
			memo,
		)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build XRP transaction: %v", err)), nil
		}

		action := "transfer"
		if memo != "" {
			action = "swap"
		}

		txResult := &types.TransactionResult{
			Transactions: []types.Transaction{
				{
					Sequence:      1,
					Chain:         "Ripple",
					Action:        action,
					SigningMode:   types.SigningModeECDSA,
					UnsignedTxHex: hex.EncodeToString(txBytes),
					TxDetails: map[string]string{
						"ticker":               "XRP",
						"from":                 senderAddr,
						"to":                   toAddr,
						"amount":               amountStr,
						"fee":                  strconv.FormatUint(feeDrops, 10),
						"sequence":             strconv.FormatUint(uint64(info.Sequence), 10),
						"last_ledger_sequence": strconv.FormatUint(uint64(currentLedger+100), 10),
					},
				},
			},
		}

		return txResult.ToToolResult()
	}
}

func buildXRPLPayment(
	from, to string,
	amountDrops uint64,
	sequence uint32,
	feeDrops uint64,
	lastLedgerSequence uint32,
	signingPubKey string,
	memo string,
) ([]byte, error) {
	jsonMap := map[string]any{
		"Account":            from,
		"TransactionType":    "Payment",
		"Amount":             fmt.Sprintf("%d", amountDrops),
		"Destination":        to,
		"Fee":                fmt.Sprintf("%d", feeDrops),
		"Sequence":           int(sequence),
		"LastLedgerSequence": int(lastLedgerSequence),
		"SigningPubKey":      strings.ToUpper(strings.TrimSpace(signingPubKey)),
	}

	if memo != "" {
		jsonMap["Memos"] = []any{
			map[string]any{
				"Memo": map[string]any{
					"MemoData": hex.EncodeToString([]byte(memo)),
					"MemoType": hex.EncodeToString([]byte("thorchain-memo")),
				},
			},
		}
	}

	hexStr, err := xrpgo.Encode(jsonMap)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	decoded, err := xrpgo.Decode(strings.ToUpper(hexStr))
	if err != nil {
		return nil, fmt.Errorf("decode round-trip: %w", err)
	}

	canonicalHex, err := xrpgo.Encode(decoded)
	if err != nil {
		return nil, fmt.Errorf("re-encode: %w", err)
	}

	txBytes, err := hex.DecodeString(canonicalHex)
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}

	return txBytes, nil
}
