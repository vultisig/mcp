package tools

import (
	"context"
	"fmt"
	"regexp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/fourbyte"
)

var selectorRE = regexp.MustCompile(`^(0x)?[0-9a-fA-F]{8}$`)

func newResolveSelectorTool() mcp.Tool {
	return mcp.NewTool("resolve_4byte_selector",
		mcp.WithDescription(
			"Resolve an Ethereum function selector (the first 4 bytes of the keccak256 hash) "+
				"to its human-readable function signature using 4byte.directory. "+
				"This helps identify unknown contract function calls without needing the full ABI. "+
				"The selector can be provided with or without the 0x prefix.",
		),
		mcp.WithString("selector",
			mcp.Description("The 4-byte function selector (e.g., '0xa9059cbb' or 'a9059cbb')."),
			mcp.Required(),
		),
	)
}

func handleResolveSelector(fbClient *fourbyte.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		selector, err := req.RequireString("selector")
		if err != nil {
			return mcp.NewToolResultError("selector parameter is required"), nil
		}

		selector = normalizeSelectorInput(selector)
		if !selectorRE.MatchString(selector) {
			return mcp.NewToolResultError("invalid selector: must be 8 hex characters (with optional 0x prefix)"), nil
		}

		sigs, err := fbClient.ResolveSelector(ctx, selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("4byte lookup failed: %v", err)), nil
		}

		if len(sigs) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("no function signatures found for selector %s", selector)), nil
		}

		resp := fmt.Sprintf("Selector: %s\nFound %d signature(s):\n\n", selector, len(sigs))
		for i, sig := range sigs {
			resp += fmt.Sprintf("%d. %s\n", i+1, sig.TextSignature)
		}

		return mcp.NewToolResultText(resp), nil
	}
}

func normalizeSelectorInput(selector string) string {
	if len(selector) >= 2 && (selector[:2] == "0x" || selector[:2] == "0X") {
		return selector[2:]
	}
	return selector
}
