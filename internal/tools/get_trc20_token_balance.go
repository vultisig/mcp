package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/tron"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetTRC20TokenBalanceTool() mcp.Tool {
	return mcp.NewTool("get_trc20_token_balance",
		mcp.WithDescription(
			"Query a TRC-20 token balance for a TRON address. "+
				"Returns the token balance, symbol, and decimals. "+
				"If no address is provided, derives it from the vault's ECDSA public key (requires set_vault_info first).",
		),
		mcp.WithString("contract_address",
			mcp.Description("TRC-20 token contract address (base58, e.g. USDT: TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t)."),
			mcp.Required(),
		),
		mcp.WithString("address",
			mcp.Description("Owner's TRON address (base58). Optional if vault info is set."),
		),
	)
}

func handleGetTRC20TokenBalance(store *vault.Store, tronClient *tron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		contractAddr, err := req.RequireString("contract_address")
		if err != nil {
			return mcp.NewToolResultError("missing contract_address parameter"), nil
		}

		err = tron.ValidateAddress(contractAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid contract address: %v", err)), nil
		}

		explicit := req.GetString("address", "")
		if explicit != "" {
			err = tron.ValidateAddress(explicit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid TRON address: %v", err)), nil
			}
		}

		addr, err := resolve.ChainAddress(explicit, resolve.ResolveVault(req, ctx, store), "Tron")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		addrHex, err := tron.AddressToHex(addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("convert address to hex: %v", err)), nil
		}
		if len(addrHex) > tron.ABIWordHexLen {
			return mcp.NewToolResultError(fmt.Sprintf("address hex too long: %d chars", len(addrHex))), nil
		}
		balanceParam := strings.Repeat("0", tron.ABIWordHexLen-len(addrHex)) + addrHex

		balanceResult, err := tronClient.TriggerConstantContract(ctx, addr, contractAddr, "balanceOf(address)", balanceParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get TRC-20 balance: %v", err)), nil
		}

		if len(balanceResult.ConstantResult) == 0 {
			return mcp.NewToolResultError("no result from balanceOf call"), nil
		}

		balance, err := tron.DecodeTRC20Balance(balanceResult.ConstantResult[0])
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode balance: %v", err)), nil
		}

		decimalsResult, err := tronClient.TriggerConstantContract(ctx, addr, contractAddr, "decimals()", "")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get TRC-20 decimals: %v", err)), nil
		}

		if len(decimalsResult.ConstantResult) == 0 {
			return mcp.NewToolResultError("no result from decimals() call"), nil
		}
		decimals, err := tron.DecodeTRC20Decimals(decimalsResult.ConstantResult[0])
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode decimals: %v", err)), nil
		}

		symbolResult, err := tronClient.TriggerConstantContract(ctx, addr, contractAddr, "symbol()", "")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get TRC-20 symbol: %v", err)), nil
		}

		var symbol string
		if len(symbolResult.ConstantResult) > 0 {
			symbol, err = tron.DecodeTRC20Symbol(symbolResult.ConstantResult[0])
			if err != nil {
				symbol = "UNKNOWN"
			}
		}

		result := map[string]any{
			"address":          addr,
			"contract_address": contractAddr,
			"symbol":           symbol,
			"balance":          tron.FormatTokenBalance(balance, decimals),
			"decimals":         decimals,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal trc20 balance result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
