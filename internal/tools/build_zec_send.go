package tools

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"

	"github.com/btcsuite/btcd/txscript"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	zcashsdk "github.com/vultisig/recipes/sdk/zcash"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildZECSendTool() mcp.Tool {
	return mcp.NewTool("build_zec_send",
		mcp.WithDescription(
			"Build an unsigned Zcash v4 transaction for a send or swap. "+
				"Automatically selects UTXOs and calculates the fee using ZIP-317 (no fee_rate param needed). "+
				"For MayaChain swaps, provide the memo parameter to include an OP_RETURN output. "+
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

func handleBuildZECSend(store *vault.Store, bcClient *blockchair.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toAddress, err := req.RequireString("to_address")
		if err != nil {
			return mcp.NewToolResultError("missing to_address"), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		amount, err := strconv.ParseUint(amountStr, 10, 64)
		if err != nil || amount == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %q", amountStr)), nil
		}

		memo := req.GetString("memo", "")
		if len(memo) > 80 {
			return mcp.NewToolResultError(fmt.Sprintf("memo too long: %d bytes (max 80)", len(memo))), nil
		}

		explicitAddr := req.GetString("address", "")

		sessionID := resolve.SessionIDFromCtx(ctx)
		v, ok := store.Get(sessionID)
		if !ok {
			return mcp.NewToolResultError("no vault info set — call set_vault_info first"), nil
		}

		senderAddr, derivedPubKey, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.Zcash)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive Zcash address: %v", err)), nil
		}
		if explicitAddr != "" {
			senderAddr = explicitAddr
		}

		pubKeyBytes, err := hex.DecodeString(derivedPubKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode derived pubkey: %v", err)), nil
		}

		dashboard, err := bcClient.GetAddressDashboard(ctx, "Zcash", senderAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetch UTXOs: %v", err)), nil
		}

		fromScript, err := zcashAddrToPkScript(senderAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid sender address: %v", err)), nil
		}

		toScript, err := zcashAddrToPkScript(toAddress)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid to_address: %v", err)), nil
		}

		changeScript, err := zcashAddrToPkScript(senderAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("create change script: %v", err)), nil
		}

		outputs := []*zcashsdk.TxOutput{
			{Value: int64(amount), Script: toScript},
			{Value: 0, Script: changeScript},
		}
		changeIdx := 1

		if memo != "" {
			memoScript, err := txscript.NullDataScript([]byte(memo))
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("create OP_RETURN script: %v", err)), nil
			}
			outputs = append(outputs, &zcashsdk.TxOutput{Value: 0, Script: memoScript})
		}

		availableUTXOs := make([]blockchair.UTXO, 0, len(dashboard.UTXOs))
		for _, u := range dashboard.UTXOs {
			if u.Value > 0 && u.Index >= 0 {
				availableUTXOs = append(availableUTXOs, u)
			}
		}
		sort.Slice(availableUTXOs, func(i, j int) bool {
			return availableUTXOs[i].Value > availableUTXOs[j].Value
		})

		var inputs []zcashsdk.TxInput
		var totalInput uint64

		for _, u := range availableUTXOs {
			inputs = append(inputs, zcashsdk.TxInput{
				TxHash:   u.TransactionHash,
				Index:    uint32(u.Index),
				Value:    uint64(u.Value),
				Script:   fromScript,
				Sequence: 0xffffffff,
			})
			totalInput += uint64(u.Value)

			fee := zecEstimateFee(len(inputs), outputs)
			if totalInput > amount+fee {
				outputs[changeIdx].Value = int64(totalInput - amount - fee)
				break
			}
		}

		if totalInput <= amount+zecEstimateFee(len(inputs), outputs) {
			return mcp.NewToolResultError(
				fmt.Sprintf("insufficient funds: have %d zatoshis, need more than %d", totalInput, amount),
			), nil
		}

		fee := zecEstimateFee(len(inputs), outputs)

		zecSDK := zcashsdk.NewSDK(nil)
		rawBytes, err := zecSDK.SerializeUnsignedTx(inputs, outputs)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialize Zcash tx: %v", err)), nil
		}

		sigHashes := make([][]byte, len(inputs))
		for i := range inputs {
			sigHash, err := zecSDK.CalculateSigHash(inputs, outputs, i)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("calculate sig hash for input %d: %v", i, err)), nil
			}
			sigHashes[i] = sigHash
		}

		finalBytes := zcashsdk.SerializeWithMetadata(rawBytes, sigHashes, pubKeyBytes)

		action := "transfer"
		if memo != "" {
			action = "swap"
		}

		txResult := &types.TransactionResult{
			Transactions: []types.Transaction{
				{
					Sequence:      1,
					Chain:         "Zcash",
					Action:        action,
					SigningMode:   types.SigningModeECDSA,
					UnsignedTxHex: hex.EncodeToString(finalBytes),
					TxDetails: map[string]string{
						"ticker":      "ZEC",
						"from":        senderAddr,
						"to":          toAddress,
						"amount":      amountStr,
						"fee":         strconv.FormatUint(fee, 10),
						"change":      strconv.FormatInt(outputs[changeIdx].Value, 10),
						"input_count": strconv.Itoa(len(inputs)),
						"tx_encoding": types.TxEncodingZcashV4,
					},
				},
			},
		}

		return txResult.ToToolResult()
	}
}

// zecEstimateFee estimates the Zcash transaction fee using ZIP-317 logical actions.
// ZIP-317: conventional_fee = marginal_fee × max(grace_actions, logical_actions)
// where marginal_fee = 5000 zatoshis, grace_actions = 2.
// See: https://zips.z.cash/zip-0317
func zecEstimateFee(numInputs int, outputs []*zcashsdk.TxOutput) uint64 {
	const (
		marginalFee     = 5000
		graceActions    = 2
		p2pkhInputSize  = 150
		p2pkhOutputSize = 34
	)

	totalOutputSize := 0
	for _, out := range outputs {
		totalOutputSize += 8 + 1 + len(out.Script)
	}

	totalInputSize := numInputs * p2pkhInputSize
	inputActions := (totalInputSize + p2pkhInputSize - 1) / p2pkhInputSize
	outputActions := (totalOutputSize + p2pkhOutputSize - 1) / p2pkhOutputSize

	logicalActions := inputActions
	if outputActions > logicalActions {
		logicalActions = outputActions
	}

	actions := graceActions
	if logicalActions > actions {
		actions = logicalActions
	}

	return uint64(marginalFee * actions)
}
