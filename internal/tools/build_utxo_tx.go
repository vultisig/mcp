package tools

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/types"
)

type utxoInput struct {
	TxID  string `json:"txid"`
	Vout  uint32 `json:"vout"`
	Value int64  `json:"value"`
}

type utxoOutput struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
}

func newBuildUTXOTxTool() mcp.Tool {
	return mcp.NewTool("build_utxo_tx",
		mcp.WithDescription(
			"Build an unsigned UTXO transaction for Bitcoin, Litecoin, Dogecoin, Dash, Bitcoin-Cash, or Zcash. "+
				"Provide explicit inputs (from list_utxos) and outputs. Fee = sum(inputs) - sum(outputs).",
		),
		mcp.WithString("chain",
			mcp.Description("UTXO chain name."),
			mcp.Required(),
			mcp.Enum(blockchair.ChainNames...),
		),
		mcp.WithString("inputs",
			mcp.Description("JSON array of inputs: [{\"txid\":\"<hex>\",\"vout\":<n>,\"value\":<sats>}]"),
			mcp.Required(),
		),
		mcp.WithString("outputs",
			mcp.Description("JSON array of outputs: [{\"address\":\"<addr>\",\"amount\":<sats>}]"),
			mcp.Required(),
		),
	)
}

func handleBuildUTXOTx() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainName, err := req.RequireString("chain")
		if err != nil {
			return mcp.NewToolResultError("missing chain parameter"), nil
		}

		chainParams, ok := utxoChains[chainName]
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unsupported UTXO chain: %s", chainName)), nil
		}

		inputsJSON, err := req.RequireString("inputs")
		if err != nil {
			return mcp.NewToolResultError("missing inputs parameter"), nil
		}
		var inputs []utxoInput
		if err := json.Unmarshal([]byte(inputsJSON), &inputs); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid inputs JSON: %v", err)), nil
		}
		if len(inputs) == 0 {
			return mcp.NewToolResultError("inputs array must not be empty"), nil
		}

		outputsJSON, err := req.RequireString("outputs")
		if err != nil {
			return mcp.NewToolResultError("missing outputs parameter"), nil
		}
		var outputs []utxoOutput
		if err := json.Unmarshal([]byte(outputsJSON), &outputs); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid outputs JSON: %v", err)), nil
		}
		if len(outputs) == 0 {
			return mcp.NewToolResultError("outputs array must not be empty"), nil
		}

		msgTx := wire.NewMsgTx(chainParams.txVersion)

		var totalInput int64
		for i, in := range inputs {
			txHash, err := chainhash.NewHashFromStr(in.TxID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("input %d: invalid txid %q: %v", i, in.TxID, err)), nil
			}
			outPoint := wire.NewOutPoint(txHash, in.Vout)
			txIn := wire.NewTxIn(outPoint, nil, nil)
			txIn.Sequence = 0xfffffffd // enable RBF
			msgTx.AddTxIn(txIn)
			totalInput += in.Value
		}

		var totalOutput int64
		for i, out := range outputs {
			if out.Amount <= 0 {
				return mcp.NewToolResultError(fmt.Sprintf("output %d: amount must be positive", i)), nil
			}
			pkScript, err := chainParams.addressToPkScript(out.Address)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("output %d: %v", i, err)), nil
			}
			msgTx.AddTxOut(wire.NewTxOut(out.Amount, pkScript))
			totalOutput += out.Amount
		}

		fee := totalInput - totalOutput
		if fee < 0 {
			return mcp.NewToolResultError(fmt.Sprintf("insufficient inputs: total_input=%d < total_output=%d", totalInput, totalOutput)), nil
		}

		var buf bytes.Buffer
		if err := msgTx.Serialize(&buf); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("serialize tx failed: %v", err)), nil
		}

		chainInfo := blockchair.SupportedChains[chainName]

		result := &types.TransactionResult{
			Transactions: []types.Transaction{
				{
					Sequence:      1,
					Chain:         chainName,
					Action:        "transfer",
					SigningMode:   types.SigningModeECDSA,
					UnsignedTxHex: hex.EncodeToString(buf.Bytes()),
					TxDetails: map[string]string{
						"input_count":  fmt.Sprintf("%d", len(inputs)),
						"output_count": fmt.Sprintf("%d", len(outputs)),
						"total_input":  fmt.Sprintf("%d", totalInput),
						"total_output": fmt.Sprintf("%d", totalOutput),
						"fee":          fmt.Sprintf("%d", fee),
						"ticker":       chainInfo.Ticker,
					},
				},
			},
		}

		return result.ToToolResult()
	}
}
