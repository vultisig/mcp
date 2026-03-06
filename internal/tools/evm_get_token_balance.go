package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newEVMGetTokenBalanceTool() mcp.Tool {
	return mcp.NewTool("evm_get_token_balance",
		mcp.WithDescription(
			"Query the ERC-20 token balance of an address on any EVM chain. "+
				"Returns balance, symbol, and decimals. "+
				"If no address is provided, derives it from the vault's ECDSA public key.",
		),
		mcp.WithString("chain",
			mcp.Description("EVM chain name. One of: "+chainEnumDesc()),
			mcp.DefaultString("Ethereum"),
		),
		mcp.WithString("contract_address",
			mcp.Description("ERC-20 token contract address (0x-prefixed)."),
			mcp.Required(),
		),
		mcp.WithString("address",
			mcp.Description("Holder address (0x-prefixed). Optional if vault info is set."),
		),
		WithCategory("balance", "evm"),
	)
}

func handleEVMGetTokenBalance(store *vault.Store, pool *evmclient.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainName := req.GetString("chain", "Ethereum")

		client, _, err := pool.Get(ctx, chainName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("chain %s unavailable: %v", chainName, err)), nil
		}

		contractAddr, err := req.RequireString("contract_address")
		if err != nil {
			return mcp.NewToolResultError("missing contract_address"), nil
		}
		if !common.IsHexAddress(contractAddr) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid contract_address: %s", contractAddr)), nil
		}

		explicit := req.GetString("address", "")
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}
		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		tb, err := client.GetTokenBalance(ctx, contractAddr, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token balance: %v", err)), nil
		}

		result := map[string]any{
			"chain":            chainName,
			"address":          addr,
			"contract_address": contractAddr,
			"symbol":           tb.Symbol,
			"balance":          tb.Balance,
			"decimals":         tb.Decimals,
		}
		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
