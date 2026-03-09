package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/tron"
	"github.com/vultisig/mcp/internal/vault"
)

const defaultTRC20FeeLimit = 100_000_000

func newBuildTRC20TransferTool() mcp.Tool {
	return mcp.NewTool("build_trc20_transfer",
		mcp.WithDescription(
			"Prepare a TRC-20 token transfer. "+
				"Returns the transaction parameters (owner, contract, recipient, amount, fee limit) "+
				"needed by the app to build and sign the transaction. "+
				"Automatically fetches the token's symbol and decimals for display. "+
				"If no from address is provided, derives it from the vault's ECDSA public key (requires set_vault_info first).",
		),
		mcp.WithString("contract_address",
			mcp.Description("TRC-20 token contract address (base58, e.g. USDT: TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t)."),
			mcp.Required(),
		),
		mcp.WithString("to",
			mcp.Description("Recipient TRON address (base58, starts with T)."),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount in smallest token unit (decimal string)."),
			mcp.Required(),
		),
		mcp.WithString("from",
			mcp.Description("Sender's TRON address (base58). Optional if vault info is set."),
		),
		mcp.WithNumber("fee_limit",
			mcp.Description("Maximum energy cost in SUN (default: 100,000,000 = 100 TRX)."),
		),
	)
}

func handleBuildTRC20Transfer(store *vault.Store, tronClient *tron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		contractAddr, err := req.RequireString("contract_address")
		if err != nil {
			return mcp.NewToolResultError("missing contract_address parameter"), nil
		}
		err = tron.ValidateAddress(contractAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid contract address: %v", err)), nil
		}

		toAddr, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError("missing to parameter"), nil
		}
		err = tron.ValidateAddress(toAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid recipient address: %v", err)), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount parameter"), nil
		}
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok || amount.Sign() <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %s", amountStr)), nil
		}

		explicit := req.GetString("from", "")
		if explicit != "" {
			err = tron.ValidateAddress(explicit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid sender address: %v", err)), nil
			}
		}

		fromAddr, err := resolve.ChainAddress(explicit, resolve.SessionIDFromCtx(ctx), store, "Tron")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		feeLimit := int64(defaultTRC20FeeLimit)
		if fl := req.GetFloat("fee_limit", 0); fl > 0 {
			feeLimit = int64(fl)
		}

		toHex, err := tron.AddressToHex(toAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("convert to address to hex: %v", err)), nil
		}
		parameter := fmt.Sprintf("%064s%064x", toHex, amount)

		symbol := "UNKNOWN"
		var decimals uint8

		result, err := tronClient.TriggerConstantContract(ctx, fromAddr, contractAddr, "symbol()", "")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token symbol: %v", err)), nil
		}
		if len(result.ConstantResult) > 0 {
			decoded, err := tron.DecodeTRC20Symbol(result.ConstantResult[0])
			if err != nil {
				log.Printf("[build_trc20_transfer] decode symbol for %s: %v", contractAddr, err)
			} else {
				symbol = decoded
			}
		}

		result, err = tronClient.TriggerConstantContract(ctx, fromAddr, contractAddr, "decimals()", "")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token decimals: %v", err)), nil
		}
		if len(result.ConstantResult) > 0 {
			d, err := tron.DecodeTRC20Decimals(result.ConstantResult[0])
			if err != nil {
				log.Printf("[build_trc20_transfer] decode decimals for %s: %v", contractAddr, err)
			} else {
				decimals = d
			}
		}

		out := map[string]any{
			"chain":             "Tron",
			"action":            "transfer",
			"signing_mode":      "ecdsa_secp256k1",
			"owner_address":     fromAddr,
			"contract_address":  contractAddr,
			"to_address":        toAddr,
			"amount":            amountStr,
			"amount_display":    tron.FormatTokenBalance(amount, decimals),
			"symbol":            symbol,
			"decimals":          decimals,
			"fee_limit_sun":     feeLimit,
			"function_selector": "transfer(address,uint256)",
			"parameter":         parameter,
		}

		data, err := json.Marshal(out)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
