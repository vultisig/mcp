package tools

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newEVMGetBalanceTool() mcp.Tool {
	return mcp.NewTool("evm_get_balance",
		mcp.WithDescription(
			"Query the native coin balance of an address on any EVM chain. "+
				"If no address is provided, derives it from the vault's ECDSA public key Accepts inline vault keys (ecdsa_public_key, eddsa_public_key, chain_code) or falls back to set_vault_info session state.",
		),
		mcp.WithString("chain",
			mcp.Description("EVM chain name. One of: "+chainEnumDesc()),
			mcp.DefaultString("Ethereum"),
		),
		mcp.WithString("address",
			mcp.Description("Wallet address (0x-prefixed). Optional if vault info is set."),
		),
	)
}

func handleEVMGetBalance(store *vault.Store, pool *evmclient.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainName := req.GetString("chain", "Ethereum")

		client, _, err := pool.Get(ctx, chainName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("chain %s unavailable: %v", chainName, err)), nil
		}

		explicit := req.GetString("address", "")
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}
		addr, err := resolve.EVMAddress(explicit, resolve.ResolveVault(ctx, req, store))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		balance, err := client.GetNativeBalance(ctx, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get balance: %v", err)), nil
		}

		ticker := evmclient.NativeTicker(chainName)
		return mcp.NewToolResultText(fmt.Sprintf("Chain: %s\nAddress: %s\nBalance: %s %s", chainName, addr, balance, ticker)), nil
	}
}
