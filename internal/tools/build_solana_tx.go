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

func newBuildSolanaTxTool() mcp.Tool {
	return mcp.NewTool("build_solana_tx",
		mcp.WithDescription(
			"Build an unsigned native SOL transfer transaction. "+
				"Auto-fetches the recent blockhash. Checks destination account existence "+
				"and validates rent exemption for new accounts. "+
				"Returns a TransactionResult with signing_mode=eddsa_ed25519.",
		),
		mcp.WithString("from",
			mcp.Description("Sender's Solana address (base58). Optional if vault info is set."),
		),
		mcp.WithString("to",
			mcp.Description("Recipient's Solana address (base58)."),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount in lamports (decimal string)."),
			mcp.Required(),
		),
	)
}

func handleBuildSolanaTx(store *vault.Store, solClient *solanaclient.Client) server.ToolHandlerFunc {
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

		txBytes, err := solClient.BuildNativeTransfer(ctx, fromPubkey, toPubkey, amount.Uint64())
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build solana tx failed: %v", err)), nil
		}

		result := &types.TransactionResult{
			Transactions: []types.Transaction{
				{
					Sequence:      1,
					Chain:         "Solana",
					Action:        "transfer",
					SigningMode:   types.SigningModeEdDSA,
					UnsignedTxHex: hex.EncodeToString(txBytes),
					TxDetails: map[string]string{
						"from":   fromAddr,
						"to":     toStr,
						"amount": amountStr,
						"ticker": "SOL",
					},
				},
			},
		}

		return result.ToToolResult()
	}
}
