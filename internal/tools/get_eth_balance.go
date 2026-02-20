package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetETHBalanceTool() mcp.Tool {
	return mcp.NewTool("get_eth_balance",
		mcp.WithDescription("Query the native ETH balance of an address. If no address is provided, derives it from the vault's ECDSA public key (requires set_vault_info first)."),
		mcp.WithString("address",
			mcp.Description("Ethereum address (0x-prefixed). Optional if vault info is set."),
		),
	)
}

func handleGetETHBalance(store *vault.Store, ethClient *ethereum.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		balance, err := ethClient.GetETHBalance(ctx, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get ETH balance: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Address: %s\nBalance: %s ETH", addr, balance)), nil
	}
}
