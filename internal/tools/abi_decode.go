package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func newABIDecodeTool() mcp.Tool {
	return mcp.NewTool("abi_decode",
		mcp.WithDescription(
			"Decode ABI-encoded data given a list of Solidity types. "+
				"Provide the hex data and comma-separated types (e.g. \"uint256,address,bool\").",
		),
		mcp.WithString("data",
			mcp.Description("0x-prefixed hex-encoded ABI data to decode."),
			mcp.Required(),
		),
		mcp.WithString("types",
			mcp.Description("Comma-separated Solidity types (e.g. \"uint256,address,bool\")."),
			mcp.Required(),
		),
	)
}

func handleABIDecode() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dataHex, err := req.RequireString("data")
		if err != nil {
			return mcp.NewToolResultError("missing data parameter"), nil
		}

		typesStr, err := req.RequireString("types")
		if err != nil {
			return mcp.NewToolResultError("missing types parameter"), nil
		}

		raw, err := hexToBytes(dataHex)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid hex data: %v", err)), nil
		}

		args, err := parseABITypes(typesStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid types: %v", err)), nil
		}

		values, err := args.UnpackValues(raw)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("ABI unpack failed: %v", err)), nil
		}

		formatted := make([]any, len(values))
		for i, v := range values {
			formatted[i] = formatABIValue(v)
		}

		resp := map[string]any{"values": formatted}
		data, err := json.Marshal(resp)
		if err != nil {
			return nil, fmt.Errorf("marshal abi_decode result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
