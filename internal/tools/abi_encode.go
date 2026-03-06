package tools

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func newABIEncodeTool() mcp.Tool {
	return mcp.NewTool("abi_encode",
		mcp.WithDescription(
			"ABI-encode a Solidity function call or pack raw arguments. "+
				"Pass a function signature like \"transfer(address,uint256)\" to get selector+args, "+
				"or bare types like \"uint256,address\" to pack args without a selector.",
		),
		mcp.WithString("signature",
			mcp.Description("Function signature (e.g. \"transfer(address,uint256)\") or bare types (e.g. \"uint256,address\")."),
			mcp.Required(),
		),
		mcp.WithArray("args",
			mcp.Description("Argument values as strings: addresses as 0x-hex, integers as decimal, bools as \"true\"/\"false\"."),
			mcp.Required(),
			mcp.WithStringItems(),
		),
		WithCategory("contract"),
	)
}

func handleABIEncode() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sig, err := req.RequireString("signature")
		if err != nil {
			return mcp.NewToolResultError("missing signature parameter"), nil
		}

		strArgs, err := req.RequireStringSlice("args")
		if err != nil {
			return mcp.NewToolResultError("missing args parameter"), nil
		}

		isFuncCall, funcName, typeStr := parseSoliditySignature(sig)

		args, err := parseABITypes(typeStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid types: %v", err)), nil
		}

		if len(strArgs) != len(args) {
			return mcp.NewToolResultError(fmt.Sprintf("expected %d args for %q, got %d", len(args), sig, len(strArgs))), nil
		}

		values := make([]any, len(args))
		for i, a := range args {
			val, err := convertStringArg(strArgs[i], a.Type)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("arg %d: %v", i, err)), nil
			}
			values[i] = val
		}

		packed, err := args.PackValues(values)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("ABI pack failed: %v", err)), nil
		}

		var result []byte
		if isFuncCall {
			// Compute 4-byte keccak selector from canonical signature.
			canonical := funcName + "(" + typeStr + ")"
			selector := crypto.Keccak256([]byte(canonical))[:4]
			result = append(selector, packed...)
		} else {
			result = packed
		}

		resp := map[string]string{"encoded": "0x" + hex.EncodeToString(result)}
		data, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal abi_encode result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// parseSoliditySignature splits a signature into function name and type list.
// Returns (isFuncCall, funcName, typeStr).
// "transfer(address,uint256)" → (true, "transfer", "address,uint256")
// "uint256,address"           → (false, "", "uint256,address")
func parseSoliditySignature(sig string) (bool, string, string) {
	sig = strings.TrimSpace(sig)

	idx := strings.Index(sig, "(")
	if idx <= 0 {
		return false, "", sig
	}

	name := sig[:idx]
	// Solidity types start with these prefixes; if the name matches one it's bare types.
	for _, prefix := range []string{
		"uint", "int", "address", "bool", "bytes", "string", "tuple", "fixed", "ufixed",
	} {
		if strings.HasPrefix(name, prefix) {
			return false, "", sig
		}
	}

	// Extract types between parens, strip spaces.
	typeStr := strings.TrimRight(sig[idx+1:], ")")
	typeStr = strings.ReplaceAll(typeStr, " ", "")

	return true, name, typeStr
}
