package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetTokenBalanceTool() mcp.Tool {
	return mcp.NewTool("get_token_balance",
		mcp.WithDescription("Query the ERC-20 token balance of an address. If no holder address is provided, derives it from the vault's ECDSA public key (requires set_vault_info first)."),
		mcp.WithString("contract_address",
			mcp.Description("ERC-20 token contract address (0x-prefixed)"),
			mcp.Required(),
		),
		mcp.WithString("address",
			mcp.Description("Holder's Ethereum address (0x-prefixed). Optional if vault info is set."),
		),
	)
}

func handleGetTokenBalance(store *vault.Store, ethClient *ethereum.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		contractAddr, err := req.RequireString("contract_address")
		if err != nil {
			return mcp.NewToolResultError("missing contract_address"), nil
		}

		explicit := req.GetString("address", "")
		addr, err := resolveAddr(explicit, sessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		tb, err := ethClient.GetTokenBalance(ctx, contractAddr, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token balance: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Address: %s\nToken: %s (%s)\nBalance: %s\nDecimals: %d",
			addr, tb.Symbol, contractAddr, tb.Balance, tb.Decimals,
		)), nil
	}
}
