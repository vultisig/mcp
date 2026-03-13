package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/gaia"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildGaiaSendTool() mcp.Tool {
	return mcp.NewTool("build_gaia_send",
		mcp.WithDescription(
			"Prepare parameters for a Cosmos Hub (Gaia) ATOM transfer transaction. "+
				"Returns all required fields (account number, sequence, chain ID) for building the transaction externally. "+
				"For THORChain cross-chain swaps, provide the memo parameter. "+
				"Requires set_vault_info to be called first.",
		),
		mcp.WithString("to",
			mcp.Description("Recipient Cosmos address (bech32, cosmos1...)"),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount in uatom (1 ATOM = 1,000,000 uatom, decimal string)"),
			mcp.Required(),
		),
		mcp.WithString("memo",
			mcp.Description("Optional memo. Use for THORChain swap memos."),
		),
		mcp.WithString("from",
			mcp.Description("Sender Cosmos address. Falls back to vault-derived address if omitted."),
		),
	)
}

func handleBuildGaiaSend(store *vault.Store, gaiaClient *gaia.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toAddr, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError("missing to parameter"), nil
		}
		err = gaia.ValidateAddress(toAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid recipient address: %v", err)), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount parameter"), nil
		}
		amountUatom, err := strconv.ParseUint(amountStr, 10, 64)
		if err != nil || amountUatom == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %q (must be a positive integer in uatom)", amountStr)), nil
		}

		memo := req.GetString("memo", "")
		explicit := req.GetString("from", "")

		v := resolve.ResolveVault(req, ctx, store)
		if v == nil {
			return mcp.NewToolResultError("no vault info available — pass vault keys inline or call set_vault_info"), nil
		}

		senderAddr, derivedPubKey, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.GaiaChain)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive Cosmos address: %v", err)), nil
		}

		if explicit != "" {
			err = gaia.ValidateAddress(explicit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid sender address: %v", err)), nil
			}
			if explicit != senderAddr {
				return mcp.NewToolResultError(fmt.Sprintf(
					"explicit from address %q does not match vault-derived address %q", explicit, senderAddr)), nil
			}
		}

		account, err := gaiaClient.GetAccount(ctx, senderAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get account info: %v", err)), nil
		}

		action := "transfer"
		if memo != "" {
			action = "swap"
		}

		uatomBig := new(big.Int).SetUint64(amountUatom)

		result := map[string]any{
			"chain":           "Cosmos",
			"action":          action,
			"signing_mode":    "ecdsa_secp256k1",
			"signing_pub_key": derivedPubKey,
			"from_address":    senderAddr,
			"to_address":      toAddr,
			"amount_uatom":    amountStr,
			"amount_atom":     gaia.FormatUATOM(uatomBig),
			"denom":           "uatom",
			"chain_id":        "cosmoshub-4",
			"account_number":  account.AccountNumber,
			"sequence":        account.Sequence,
			"ticker":          "ATOM",
		}

		if memo != "" {
			result["memo"] = memo
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
