package tools

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	ethclient "github.com/vultisig/mcp/internal/ethereum"
)

func newEVMCallTool() mcp.Tool {
	return mcp.NewTool("evm_call",
		mcp.WithDescription(
			"Execute an eth_call (read-only) against a contract. "+
				"Returns raw hex output, and optionally decodes it if output_types is provided.",
		),
		mcp.WithString("to",
			mcp.Description("Contract address (0x-prefixed)."),
			mcp.Required(),
		),
		mcp.WithString("data",
			mcp.Description("Hex-encoded calldata (0x-prefixed)."),
			mcp.Required(),
		),
		mcp.WithString("from",
			mcp.Description("Sender address for call context (optional)."),
		),
		mcp.WithString("value",
			mcp.Description("Wei value to send with the call (decimal string, default \"0\")."),
		),
		mcp.WithString("block",
			mcp.Description("Block number (decimal) or \"latest\" (default \"latest\")."),
		),
		mcp.WithString("output_types",
			mcp.Description("Comma-separated ABI types to decode the output (e.g. \"uint256,address\"). If omitted, only raw hex is returned."),
		),
	)
}

func handleEVMCall(ethClient *ethclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toStr, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError("missing to parameter"), nil
		}
		if !common.IsHexAddress(toStr) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid to address: %s", toStr)), nil
		}
		to := common.HexToAddress(toStr)

		dataHex, err := req.RequireString("data")
		if err != nil {
			return mcp.NewToolResultError("missing data parameter"), nil
		}
		calldata, err := hexToBytes(dataHex)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid data hex: %v", err)), nil
		}

		msg := ethereum.CallMsg{
			To:   &to,
			Data: calldata,
		}

		if fromStr := req.GetString("from", ""); fromStr != "" {
			if !common.IsHexAddress(fromStr) {
				return mcp.NewToolResultError(fmt.Sprintf("invalid from address: %s", fromStr)), nil
			}
			msg.From = common.HexToAddress(fromStr)
		}

		if valueStr := req.GetString("value", ""); valueStr != "" {
			val, ok := new(big.Int).SetString(valueStr, 10)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("invalid value: %s", valueStr)), nil
			}
			msg.Value = val
		}

		var blockNum *big.Int
		if blockStr := req.GetString("block", ""); blockStr != "" && blockStr != "latest" {
			bn, ok := new(big.Int).SetString(blockStr, 10)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("invalid block number: %s", blockStr)), nil
			}
			blockNum = bn
		}

		output, err := ethClient.CallContract(ctx, msg, blockNum)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("eth_call failed: %v", err)), nil
		}

		resp := map[string]any{
			"result": "0x" + hex.EncodeToString(output),
		}

		if outputTypes := req.GetString("output_types", ""); outputTypes != "" {
			args, err := parseABITypes(outputTypes)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid output_types: %v", err)), nil
			}
			values, err := args.UnpackValues(output)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("decode output failed: %v", err)), nil
			}
			formatted := make([]any, len(values))
			for i, v := range values {
				formatted[i] = formatABIValue(v)
			}
			resp["decoded"] = formatted
		}

		data, err := json.Marshal(resp)
		if err != nil {
			return nil, fmt.Errorf("marshal evm_call result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
