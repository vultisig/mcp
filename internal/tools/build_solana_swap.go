package tools

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/jupiter"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildSolanaSwapTool() mcp.Tool {
	return mcp.NewTool("build_solana_swap",
		mcp.WithDescription(
			"Build an unsigned Solana swap transaction via Jupiter aggregator. "+
				"Fetches a quote, gets swap instructions, and assembles a serialized "+
				"Solana transaction ready for EdDSA signing. "+
				"For native SOL as input, leave input_mint empty. "+
				"Returns a TransactionResult with signing_mode=eddsa_ed25519.",
		),
		mcp.WithString("from",
			mcp.Description("Sender's Solana address (base58). Optional if vault info is set."),
		),
		mcp.WithString("output_mint",
			mcp.Description("Destination token mint address (base58). Leave empty for native SOL."),
			mcp.Required(),
		),
		mcp.WithString("amount",
			mcp.Description("Amount to swap in base units (lamports for SOL, smallest unit for tokens)."),
			mcp.Required(),
		),
		mcp.WithString("input_mint",
			mcp.Description("Source token mint address (base58). Empty or omitted for native SOL."),
		),
		mcp.WithNumber("slippage_bps",
			mcp.Description("Slippage tolerance in basis points (default: 100 = 1%)."),
		),
		WithCategory("swap", "solana"),
	)
}

func handleBuildSolanaSwap(store *vault.Store, jupClient *jupiter.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("from", "")
		fromAddr, err := resolve.ChainAddress(explicit, resolve.SessionIDFromCtx(ctx), store, "Solana")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		outputMint, err := req.RequireString("output_mint")
		if err != nil {
			return mcp.NewToolResultError("missing output_mint parameter"), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount parameter"), nil
		}
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok || amount.Sign() <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %s", amountStr)), nil
		}

		inputMint := req.GetString("input_mint", "")
		slippageBps := int(req.GetFloat("slippage_bps", float64(jupiter.DefaultSlippageBps)))

		result, err := jupClient.BuildSwapTransaction(ctx, fromAddr, inputMint, outputMint, amount, slippageBps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build solana swap failed: %v", err)), nil
		}

		txDetails := map[string]string{
			"from":        fromAddr,
			"input_mint":  result.InputMint,
			"output_mint": result.OutputMint,
			"amount":      amountStr,
		}
		if result.OutAmount != nil {
			txDetails["expected_output"] = result.OutAmount.String()
		}
		if result.MinimumOutput != nil {
			txDetails["minimum_output"] = result.MinimumOutput.String()
		}
		if result.PriceImpact != "" {
			txDetails["price_impact"] = result.PriceImpact
		}

		txResult := &types.TransactionResult{
			Transactions: []types.Transaction{
				{
					Sequence:      1,
					Chain:         "Solana",
					Action:        "swap",
					SigningMode:   types.SigningModeEdDSA,
					UnsignedTxHex: hex.EncodeToString(result.TxBytes),
					TxDetails:     txDetails,
				},
			},
		}

		return txResult.ToToolResult()
	}
}
