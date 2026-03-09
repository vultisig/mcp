package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
)

func newBuildEVMTxTool() mcp.Tool {
	return mcp.NewTool("build_evm_tx",
		mcp.WithDescription(
			"Return EVM transaction arguments for an EIP-1559 (type 2) transaction. "+
				"Use evm_tx_info to obtain nonce, gas prices, and chain ID first. "+
				"The client is responsible for assembling and signing the transaction.",
		),
		mcp.WithString("chain",
			mcp.Description("EVM chain name. One of: "+chainEnumDesc()+". Determines chain_id when not explicitly set."),
			mcp.DefaultString("Ethereum"),
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
			mcp.Description("Chain ID override (decimal string). Defaults to the chain's known ID."),
		),
	)
}

func handleBuildEVMTx() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainName := req.GetString("chain", "Ethereum")

		chainID, ok := evmclient.ChainIDByName(chainName)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unsupported chain: %s", chainName)), nil
		}
		if cidStr := req.GetString("chain_id", ""); cidStr != "" {
			cid, cidOK := new(big.Int).SetString(cidStr, 10)
			if !cidOK || cid.Sign() <= 0 {
				return mcp.NewToolResultError(fmt.Sprintf("invalid chain_id: %s", cidStr)), nil
			}
			chainID = cid
		}

		toStr, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError("missing to parameter"), nil
		}
		if !common.IsHexAddress(toStr) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid to address: %s", toStr)), nil
		}

		valueStr, err := req.RequireString("value")
		if err != nil {
			return mcp.NewToolResultError("missing value parameter"), nil
		}
		valueInt, valueOK := new(big.Int).SetString(valueStr, 10)
		if !valueOK || valueInt.Sign() < 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid value: %s", valueStr)), nil
		}

		dataHex := req.GetString("data", "0x")
		if dataHex != "" && dataHex != "0x" {
			_, hexErr := hexToBytes(dataHex)
			if hexErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid data hex: %v", hexErr)), nil
			}
		}

		nonceStr, err := req.RequireString("nonce")
		if err != nil {
			return mcp.NewToolResultError("missing nonce parameter"), nil
		}
		nonceInt, nonceOK := new(big.Int).SetString(nonceStr, 10)
		if !nonceOK || nonceInt.Sign() < 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid nonce: %s", nonceStr)), nil
		}

		gasLimitStr, err := req.RequireString("gas_limit")
		if err != nil {
			return mcp.NewToolResultError("missing gas_limit parameter"), nil
		}
		gasInt, gasOK := new(big.Int).SetString(gasLimitStr, 10)
		if !gasOK || gasInt.Sign() <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid gas_limit: %s", gasLimitStr)), nil
		}

		maxFeeStr, err := req.RequireString("max_fee_per_gas")
		if err != nil {
			return mcp.NewToolResultError("missing max_fee_per_gas parameter"), nil
		}
		maxFeeInt, maxFeeOK := new(big.Int).SetString(maxFeeStr, 10)
		if !maxFeeOK || maxFeeInt.Sign() <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid max_fee_per_gas: %s", maxFeeStr)), nil
		}

		maxPriorityFeeStr, err := req.RequireString("max_priority_fee_per_gas")
		if err != nil {
			return mcp.NewToolResultError("missing max_priority_fee_per_gas parameter"), nil
		}
		maxPriorityInt, maxPriorityOK := new(big.Int).SetString(maxPriorityFeeStr, 10)
		if !maxPriorityOK || maxPriorityInt.Sign() < 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid max_priority_fee_per_gas: %s", maxPriorityFeeStr)), nil
		}

		result := map[string]any{
			"chain":                    chainName,
			"chain_id":                 chainID.String(),
			"to":                       common.HexToAddress(toStr).Hex(),
			"value":                    valueStr,
			"data":                     dataHex,
			"nonce":                    nonceStr,
			"gas_limit":                gasLimitStr,
			"max_fee_per_gas":          maxFeeStr,
			"max_priority_fee_per_gas": maxPriorityFeeStr,
			"tx_type":                  2,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
