package tools

import (
	"context"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
)

func newConvertAmountTool() mcp.Tool {
	return mcp.NewTool("convert_amount",
		mcp.WithDescription("Convert between human-readable and base unit amounts. Use direction \"to_base\" to convert \"1.5\" → \"1500000\" (for 6 decimals), or \"to_human\" to convert \"1500000\" → \"1.5\"."),
		mcp.WithString("amount", mcp.Description("The amount to convert"), mcp.Required()),
		mcp.WithNumber("decimals", mcp.Description("Number of decimal places for the token (e.g. 18 for ETH, 6 for USDC)"), mcp.Required()),
		mcp.WithString("direction", mcp.Description("\"to_base\" (human→base) or \"to_human\" (base→human)"), mcp.Required()),
	)
}

func handleConvertAmount() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		decimals := int(req.GetInt("decimals", 18))
		direction, err := req.RequireString("direction")
		if err != nil {
			return mcp.NewToolResultError("missing direction"), nil
		}

		switch direction {
		case "to_base":
			result, parseErr := parseToBase(amountStr, decimals)
			if parseErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", parseErr)), nil
			}
			return mcp.NewToolResultText(result.String()), nil

		case "to_human":
			val, ok := new(big.Int).SetString(amountStr, 10)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("invalid base amount: %q", amountStr)), nil
			}
			return mcp.NewToolResultText(evmclient.FormatUnits(val, decimals)), nil

		default:
			return mcp.NewToolResultError(fmt.Sprintf("invalid direction: %q (use \"to_base\" or \"to_human\")", direction)), nil
		}
	}
}

func parseToBase(s string, decimals int) (*big.Int, error) {
	parts := splitDecimal(s)
	wholePart := parts[0]
	if wholePart == "" {
		wholePart = "0"
	}

	fracPart := ""
	if len(parts) > 1 {
		fracPart = parts[1]
	}

	if len(fracPart) > decimals {
		fracPart = fracPart[:decimals]
	}
	for len(fracPart) < decimals {
		fracPart += "0"
	}

	combined := wholePart + fracPart
	for len(combined) > 1 && combined[0] == '0' {
		combined = combined[1:]
	}

	result, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %q", s)
	}
	return result, nil
}

func splitDecimal(s string) []string {
	for i, c := range s {
		if c == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
