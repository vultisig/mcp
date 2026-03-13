package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gagliardetto/solana-go"
	"github.com/vultisig/mcp/internal/jupiter"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildSolanaSwapTool() mcp.Tool {
	return mcp.NewTool("build_solana_swap",
		mcp.WithDescription(
			"Return Solana swap arguments via Jupiter aggregator for the client to build and sign the transaction. "+
				"Fetches a quote and returns swap parameters including expected output and price impact. "+
				"For native SOL as input, leave input_mint empty.",
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
	)
}

func handleBuildSolanaSwap(store *vault.Store, jupClient *jupiter.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("from", "")
		fromAddr, err := resolve.ChainAddress(explicit, resolve.ResolveVault(ctx, req, store), "Solana")
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
		if inputMint == "" {
			inputMint = solana.SolMint.String()
		}

		slippageBps := int(req.GetFloat("slippage_bps", float64(jupiter.DefaultSlippageBps)))

		quote, err := jupClient.GetQuote(ctx, inputMint, outputMint, amount, slippageBps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get swap quote failed: %v", err)), nil
		}

		txArgs := map[string]any{
			"chain":         "Solana",
			"action":        "swap",
			"from":          fromAddr,
			"input_mint":    quote.InputMint,
			"output_mint":   quote.OutputMint,
			"amount":        amountStr,
			"slippage_bps":  slippageBps,
			"out_amount":    quote.OutAmount,
			"min_output":    quote.OtherAmountThreshold,
			"price_impact":  quote.PriceImpactPct,
			"signing_mode":  "eddsa_ed25519",
		}

		data, err := json.Marshal(txArgs)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
