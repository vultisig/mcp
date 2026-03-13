package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newEVMTxInfoTool() mcp.Tool {
	return mcp.NewTool("evm_tx_info",
		mcp.WithDescription(
			"Get nonce, gas prices, and chain ID for building an EVM transaction on any EVM chain. "+
				"If to/data/value are provided, also estimates gas. "+
				"Address falls back to vault-derived if not provided.",
		),
		mcp.WithString("chain",
			mcp.Description("EVM chain name. One of: "+chainEnumDesc()),
			mcp.DefaultString("Ethereum"),
		),
		mcp.WithString("address",
			mcp.Description("Sender address (0x-prefixed). Optional if vault info is set."),
		),
		mcp.WithString("to",
			mcp.Description("Destination address for gas estimation (optional)."),
		),
		mcp.WithString("data",
			mcp.Description("Hex calldata for gas estimation (optional)."),
		),
		mcp.WithString("value",
			mcp.Description("Wei value for gas estimation (decimal string, optional)."),
		),
	)
}

func handleEVMTxInfo(store *vault.Store, pool *evmclient.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainName := req.GetString("chain", "Ethereum")

		client, chainID, err := pool.Get(ctx, chainName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("chain %s unavailable: %v", chainName, err)), nil
		}

		explicit := req.GetString("address", "")
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}
		addr, err := resolve.EVMAddress(explicit, resolve.ResolveVault(req, ctx, store))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		address := common.HexToAddress(addr)

		nonce, err := client.PendingNonce(ctx, address)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
		}

		tipCap, err := client.SuggestGasTipCap(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get gas tip cap: %v", err)), nil
		}

		baseFee, err := client.LatestBaseFee(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get base fee: %v", err)), nil
		}

		suggestedMaxFee := new(big.Int).Mul(baseFee, big.NewInt(2))
		suggestedMaxFee.Add(suggestedMaxFee, tipCap)

		resp := map[string]any{
			"chain":                     chainName,
			"address":                   addr,
			"chain_id":                  chainID.String(),
			"nonce":                     nonce,
			"base_fee_per_gas":          baseFee.String(),
			"max_priority_fee_per_gas":  tipCap.String(),
			"suggested_max_fee_per_gas": suggestedMaxFee.String(),
		}

		if toStr := req.GetString("to", ""); toStr != "" {
			if !common.IsHexAddress(toStr) {
				return mcp.NewToolResultError(fmt.Sprintf("invalid to address: %s", toStr)), nil
			}
			to := common.HexToAddress(toStr)

			msg := ethereum.CallMsg{
				From: address,
				To:   &to,
			}

			if dataHex := req.GetString("data", ""); dataHex != "" {
				calldata, err := hexToBytes(dataHex)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid data hex: %v", err)), nil
				}
				msg.Data = calldata
			}

			if valueStr := req.GetString("value", ""); valueStr != "" {
				val, ok := new(big.Int).SetString(valueStr, 10)
				if !ok {
					return mcp.NewToolResultError(fmt.Sprintf("invalid value: %s", valueStr)), nil
				}
				msg.Value = val
			}

			gasEstimate, err := client.EstimateGas(ctx, msg)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("gas estimation failed: %v", err)), nil
			}
			resp["estimated_gas"] = gasEstimate
		}

		data, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal evm_tx_info result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
