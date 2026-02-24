package tools

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	reth "github.com/vultisig/recipes/chain/evm/ethereum"

	"github.com/vultisig/mcp/internal/types"
)

func newBuildEVMTxTool() mcp.Tool {
	return mcp.NewTool("build_evm_tx",
		mcp.WithDescription(
			"Build an unsigned EIP-1559 (type 2) EVM transaction. "+
				"All parameters are explicit — the agent provides nonce, gas, and fee values "+
				"(typically obtained from evm_tx_info). "+
				"Returns the RLP-encoded unsigned transaction ready for signing.",
		),
		mcp.WithString("to",
			mcp.Description("Destination address (0x-prefixed)."),
			mcp.Required(),
		),
		mcp.WithString("value",
			mcp.Description("Wei value to transfer (decimal string)."),
			mcp.Required(),
		),
		mcp.WithString("data",
			mcp.Description("Hex-encoded calldata (0x-prefixed). Default \"0x\" (empty)."),
		),
		mcp.WithString("nonce",
			mcp.Description("Sender nonce (decimal string)."),
			mcp.Required(),
		),
		mcp.WithString("gas_limit",
			mcp.Description("Gas limit (decimal string)."),
			mcp.Required(),
		),
		mcp.WithString("max_fee_per_gas",
			mcp.Description("Max fee per gas in wei (decimal string)."),
			mcp.Required(),
		),
		mcp.WithString("max_priority_fee_per_gas",
			mcp.Description("Max priority fee (tip) per gas in wei (decimal string)."),
			mcp.Required(),
		),
		mcp.WithString("chain_id",
			mcp.Description("Chain ID (decimal string). Defaults to the server's connected chain."),
		),
	)
}

func handleBuildEVMTx(serverChainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toStr, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError("missing to parameter"), nil
		}
		if !common.IsHexAddress(toStr) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid to address: %s", toStr)), nil
		}
		to := common.HexToAddress(toStr)

		valueStr, err := req.RequireString("value")
		if err != nil {
			return mcp.NewToolResultError("missing value parameter"), nil
		}
		value, ok := new(big.Int).SetString(valueStr, 10)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("invalid value: %s", valueStr)), nil
		}

		var txData []byte
		if dataHex := req.GetString("data", ""); dataHex != "" && dataHex != "0x" {
			txData, err = hexToBytes(dataHex)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid data hex: %v", err)), nil
			}
		}

		nonceStr, err := req.RequireString("nonce")
		if err != nil {
			return mcp.NewToolResultError("missing nonce parameter"), nil
		}
		nonce, ok := new(big.Int).SetString(nonceStr, 10)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("invalid nonce: %s", nonceStr)), nil
		}

		gasLimitStr, err := req.RequireString("gas_limit")
		if err != nil {
			return mcp.NewToolResultError("missing gas_limit parameter"), nil
		}
		gasLimit, ok := new(big.Int).SetString(gasLimitStr, 10)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("invalid gas_limit: %s", gasLimitStr)), nil
		}

		maxFeeStr, err := req.RequireString("max_fee_per_gas")
		if err != nil {
			return mcp.NewToolResultError("missing max_fee_per_gas parameter"), nil
		}
		maxFee, ok := new(big.Int).SetString(maxFeeStr, 10)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("invalid max_fee_per_gas: %s", maxFeeStr)), nil
		}

		maxPriorityFeeStr, err := req.RequireString("max_priority_fee_per_gas")
		if err != nil {
			return mcp.NewToolResultError("missing max_priority_fee_per_gas parameter"), nil
		}
		maxPriorityFee, ok := new(big.Int).SetString(maxPriorityFeeStr, 10)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("invalid max_priority_fee_per_gas: %s", maxPriorityFeeStr)), nil
		}

		chainID := new(big.Int).Set(serverChainID)
		if cidStr := req.GetString("chain_id", ""); cidStr != "" {
			cid, ok := new(big.Int).SetString(cidStr, 10)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("invalid chain_id: %s", cidStr)), nil
			}
			chainID = cid
		}

		// Encode using the same DynamicFeeTxWithoutSignature struct the
		// Vultisig recipes SDK uses, ensuring the signer can decode it
		// via DecodeUnsignedPayload.
		payload, err := rlp.EncodeToBytes(reth.DynamicFeeTxWithoutSignature{
			ChainID:    chainID,
			Nonce:      nonce.Uint64(),
			GasTipCap:  maxPriorityFee,
			GasFeeCap:  maxFee,
			Gas:        gasLimit.Uint64(),
			To:         &to,
			Value:      value,
			Data:       txData,
			AccessList: ethtypes.AccessList{},
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode tx failed: %v", err)), nil
		}
		rawBytes := append([]byte{ethtypes.DynamicFeeTxType}, payload...)

		result := &types.TransactionResult{
			Transactions: []types.Transaction{
				{
					Sequence:      1,
					Chain:         types.EVMChainName(chainID),
					ChainID:       chainID.String(),
					Action:        "transfer",
					SigningMode:   types.SigningModeECDSA,
					UnsignedTxHex: hex.EncodeToString(rawBytes),
					TxDetails: map[string]string{
						"to":                       to.Hex(),
						"value":                    value.String(),
						"nonce":                    nonceStr,
						"gas_limit":                gasLimitStr,
						"max_fee_per_gas":          maxFee.String(),
						"max_priority_fee_per_gas": maxPriorityFee.String(),
						"data":                     "0x" + hex.EncodeToString(txData),
						"tx_encoding":              types.TxEncodingEIP1559RLP,
					},
				},
			},
		}

		return result.ToToolResult()
	}
}
