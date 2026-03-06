package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildSPLTransferTxTool() mcp.Tool {
	return mcp.NewTool("build_spl_transfer_tx",
		mcp.WithDescription(
			"Return SPL token transfer arguments for the client to build and sign the transaction. "+
				"Detects token program (SPL vs Token-2022) and returns token account addresses. "+
				"For native SOL transfers (including wSOL mint So1111...1112), use build_solana_tx instead.",
		),
		mcp.WithString("from",
			mcp.Description("Sender's Solana address (base58). Optional if vault info is set."),
		),
		mcp.WithString("to",
			mcp.Description("Recipient's Solana address (base58, the owner — not the ATA)."),
			mcp.Required(),
		),
		mcp.WithString("mint",
			mcp.Description("Token mint address (base58)."),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount in base units (decimal string)."),
			mcp.Required(),
		),
	)
}

func handleBuildSPLTransferTx(store *vault.Store, solClient *solanaclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("from", "")
		fromAddr, err := resolve.ChainAddress(explicit, resolve.SessionIDFromCtx(ctx), store, "Solana")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		toStr, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError("missing to parameter"), nil
		}

		mintStr, err := req.RequireString("mint")
		if err != nil {
			return mcp.NewToolResultError("missing mint parameter"), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount parameter"), nil
		}
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok || amount.Sign() <= 0 || !amount.IsUint64() {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %s", amountStr)), nil
		}

		fromPubkey, err := solanaclient.ParsePublicKey(fromAddr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid from address: %v", err)), nil
		}

		toPubkey, err := solanaclient.ParsePublicKey(toStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid to address: %v", err)), nil
		}

		mintPubkey, err := solanaclient.ParsePublicKey(mintStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid mint address: %v", err)), nil
		}

		tokenProgram, decimals, err := solClient.GetTokenProgram(ctx, mintPubkey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to detect token program: %v", err)), nil
		}
		if tokenProgram == (solana.PublicKey{}) {
			return mcp.NewToolResultError("native SOL transfers (including wSOL) should use build_solana_tx instead"), nil
		}

		fromATA, _, err := solanaclient.FindAssociatedTokenAddress(fromPubkey, mintPubkey, tokenProgram)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive sender ATA: %v", err)), nil
		}

		toATA, _, err := solanaclient.FindAssociatedTokenAddress(toPubkey, mintPubkey, tokenProgram)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive recipient ATA: %v", err)), nil
		}

		result := map[string]any{
			"chain":         "Solana",
			"action":        "spl_transfer",
			"from":          fromAddr,
			"to":            toStr,
			"mint":          mintStr,
			"amount":        amountStr,
			"decimals":      decimals,
			"token_program": tokenProgram.String(),
			"from_ata":      fromATA.String(),
			"to_ata":        toATA.String(),
			"signing_mode":  "eddsa_ed25519",
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
