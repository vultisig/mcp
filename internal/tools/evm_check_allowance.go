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

func newEVMCheckAllowanceTool() mcp.Tool {
	return mcp.NewTool("evm_check_allowance",
		mcp.WithDescription(
			"Check how much of an ERC-20 token a spender is allowed to transfer on behalf of the owner. "+
				"Used to determine if an approve transaction is needed before a swap or protocol interaction.",
		),
		mcp.WithString("chain",
			mcp.Description("EVM chain name. One of: "+chainEnumDesc()),
			mcp.DefaultString("Ethereum"),
		),
		mcp.WithString("contract_address",
			mcp.Description("ERC-20 token contract address (0x-prefixed)."),
			mcp.Required(),
		),
		mcp.WithString("owner",
			mcp.Description("Token holder address (0x-prefixed). Optional if vault info is set."),
		),
		mcp.WithString("spender",
			mcp.Description("Address that is allowed to spend tokens (e.g. DEX router contract)."),
			mcp.Required(),
		),
		WithCategory("contract", "evm"),
	)
}

func handleEVMCheckAllowance(store *vault.Store, pool *evmclient.Pool) server.ToolHandlerFunc {
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
		spender, err := req.RequireString("spender")
		if err != nil {
			return mcp.NewToolResultError("missing spender"), nil
		}
		if !common.IsHexAddress(spender) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid spender: %s", spender)), nil
		}
		explicit := req.GetString("owner", "")
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid owner: %s", explicit)), nil
		}
		owner, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		allowance, decimals, symbol, err := client.GetAllowance(ctx, contractAddr, owner, spender)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get allowance: %v", err)), nil
		}

		result := map[string]any{
			"chain":               chainName,
			"contract_address":    contractAddr,
			"symbol":              symbol,
			"decimals":            decimals,
			"owner":               owner,
			"spender":             spender,
			"allowance":           allowance.String(),
			"allowance_formatted": evmclient.FormatUnits(allowance, int(decimals)),
		}
		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
