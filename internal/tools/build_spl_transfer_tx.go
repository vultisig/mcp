package tools

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildSPLTransferTxTool() mcp.Tool {
	return mcp.NewTool("build_spl_transfer_tx",
		mcp.WithDescription(
			"Build an unsigned SPL token transfer transaction. "+
				"Auto-detects token program (SPL vs Token-2022) and derives associated token accounts. "+
				"If the destination ATA does not exist, includes a create-ATA instruction in the transaction. "+
				"Returns a TransactionResult with signing_mode=eddsa_ed25519.",
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
		mcp.WithNumber("decimals",
			mcp.Description("Token decimals (required for transfer_checked instruction)."),
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

		decimalsVal := req.GetFloat("decimals", -1)
		if decimalsVal < 0 || decimalsVal > 255 || decimalsVal != float64(uint8(decimalsVal)) {
			return mcp.NewToolResultError("missing or invalid decimals parameter"), nil
		}
		decimals := uint8(decimalsVal)

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

		tokenProgram, _, err := solClient.GetTokenProgram(ctx, mintPubkey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to detect token program: %v", err)), nil
		}

		txBytes, err := solClient.BuildTokenTransfer(ctx, mintPubkey, fromPubkey, toPubkey, amount.Uint64(), decimals, tokenProgram)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build spl transfer tx failed: %v", err)), nil
		}

		result := &types.TransactionResult{
			Transactions: []types.Transaction{
				{
					Sequence:      1,
					Chain:         "Solana",
					Action:        "spl_transfer",
					SigningMode:   types.SigningModeEdDSA,
					UnsignedTxHex: hex.EncodeToString(txBytes),
					TxDetails: map[string]string{
						"from":          fromAddr,
						"to":            toStr,
						"mint":          mintStr,
						"amount":        amountStr,
						"decimals":      fmt.Sprintf("%d", decimals),
						"token_program": tokenProgram.String(),
					},
				},
			},
		}

		return result.ToToolResult()
	}
}
