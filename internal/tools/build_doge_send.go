package tools

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	btcsdk "github.com/vultisig/recipes/sdk/btc"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildDOGESendTool() mcp.Tool {
	return mcp.NewTool("build_doge_send",
		mcp.WithDescription(
			"Build an unsigned Dogecoin PSBT for a send or swap. "+
				"Automatically selects UTXOs, calculates fees, and handles change. "+
				"For THORChain swaps, provide the memo parameter to include an OP_RETURN output. "+
				"Requires set_vault_info to be called first.",
		),
		mcp.WithString("to_address",
			mcp.Description("Recipient Dogecoin address"),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount to send in koinus (1 DOGE = 100,000,000 koinus, decimal string)"),
			mcp.Required(),
		),
		mcp.WithNumber("fee_rate",
			mcp.Description("Fee rate in sat/vB (use doge_fee_rate tool to get recommended rate)"),
			mcp.Required(),
		),
		mcp.WithString("memo",
			mcp.Description("Optional OP_RETURN memo (e.g. THORChain swap instruction, max 80 bytes)"),
		),
		mcp.WithString("address",
			mcp.Description("Sender Dogecoin address. Falls back to vault-derived address if omitted."),
		),
		WithCategory("send"),
	)
}

func handleBuildDOGESend(store *vault.Store, bcClient *blockchair.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toAddress, err := req.RequireString("to_address")
		if err != nil {
			return mcp.NewToolResultError("missing to_address"), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		amount, err := strconv.ParseInt(amountStr, 10, 64)
		if err != nil || amount <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %q", amountStr)), nil
		}

		feeRateFloat := req.GetFloat("fee_rate", 0)
		if math.IsNaN(feeRateFloat) || math.IsInf(feeRateFloat, 0) || feeRateFloat < 0 || feeRateFloat >= float64(math.MaxUint64) {
			return mcp.NewToolResultError("fee_rate must be a valid positive number"), nil
		}
		feeRate := uint64(math.Round(feeRateFloat))
		if feeRate == 0 {
			return mcp.NewToolResultError("fee_rate must be greater than 0"), nil
		}

		memo := req.GetString("memo", "")
		explicitAddr := req.GetString("address", "")

		sessionID := resolve.SessionIDFromCtx(ctx)
		v, ok := store.Get(sessionID)
		if !ok {
			return mcp.NewToolResultError("no vault info set — call set_vault_info first"), nil
		}

		senderAddr, derivedPubKey, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.Dogecoin)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive Dogecoin address: %v", err)), nil
		}
		if explicitAddr != "" && explicitAddr != senderAddr {
			return mcp.NewToolResultError(fmt.Sprintf(
				"address %q does not match vault-derived address %q", explicitAddr, senderAddr)), nil
		}

		pubKeyBytes, err := hex.DecodeString(derivedPubKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode derived pubkey: %v", err)), nil
		}

		dashboard, err := bcClient.GetAddressDashboard(ctx, "Dogecoin", senderAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetch UTXOs: %v", err)), nil
		}

		utxos := make([]btcsdk.UTXO, 0, len(dashboard.UTXOs))
		for _, u := range dashboard.UTXOs {
			if u.Value <= 0 || u.Index < 0 {
				continue
			}
			utxos = append(utxos, btcsdk.UTXO{
				TxHash: u.TransactionHash,
				Index:  uint32(u.Index),
				Value:  uint64(u.Value),
			})
		}

		dogeChain := utxoChains["Dogecoin"]

		recipientScript, err := dogeChain.addressToPkScript(toAddress)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid to_address: %v", err)), nil
		}

		changeScript, err := dogeChain.addressToPkScript(senderAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid sender address: %v", err)), nil
		}

		outputs := []*wire.TxOut{
			{Value: amount, PkScript: recipientScript},
			{Value: 0, PkScript: changeScript},
		}

		if memo != "" {
			if len(memo) > 80 {
				return mcp.NewToolResultError(fmt.Sprintf("memo too long: %d bytes (max 80)", len(memo))), nil
			}
			memoScript, err := txscript.NullDataScript([]byte(memo))
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("create OP_RETURN script: %v", err)), nil
			}
			outputs = append(outputs, &wire.TxOut{Value: 0, PkScript: memoScript})
		}

		changeIdx := 1
		utxoBuilder := btcsdk.NewBuilder(100_000_000)
		result, err := utxoBuilder.Build(utxos, outputs, changeIdx, feeRate, pubKeyBytes)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build transaction: %v", err)), nil
		}

		err = btcsdk.PopulatePSBTMetadata(result, bcClient.ChainFetcherWithCtx(ctx, "Dogecoin"))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("populate PSBT metadata: %v", err)), nil
		}

		var buf bytes.Buffer
		err = result.Packet.Serialize(&buf)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialize PSBT: %v", err)), nil
		}

		action := "transfer"
		if memo != "" {
			action = "swap"
		}

		txResult := &types.TransactionResult{
			Transactions: []types.Transaction{
				{
					Sequence:      1,
					Chain:         "Dogecoin",
					Action:        action,
					SigningMode:   types.SigningModeECDSA,
					UnsignedTxHex: hex.EncodeToString(buf.Bytes()),
					TxDetails: map[string]string{
						"ticker":      "DOGE",
						"from":        senderAddr,
						"to":          toAddress,
						"amount":      amountStr,
						"fee":         strconv.FormatUint(result.Fee, 10),
						"change":      strconv.FormatInt(result.ChangeAmount, 10),
						"input_count": strconv.Itoa(len(result.SelectedUTXOs)),
						"tx_encoding": types.TxEncodingPSBT,
					},
				},
			},
		}

		return txResult.ToToolResult()
	}
}
