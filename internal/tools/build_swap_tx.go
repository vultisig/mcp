package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/recipes/sdk/swap"
)

func newBuildSwapTxTool() mcp.Tool {
	return mcp.NewTool("build_swap_tx",
		mcp.WithDescription("Build unsigned transaction(s) for a token swap. Supports THORChain, Mayachain, 1inch, LiFi, Jupiter, and Uniswap providers. Returns the swap transaction and an optional ERC20 approval transaction."),
		mcp.WithString("from_chain", mcp.Description("Source chain (e.g. \"Ethereum\", \"Bitcoin\", \"Solana\")"), mcp.Required()),
		mcp.WithString("from_symbol", mcp.Description("Source token symbol (e.g. \"ETH\", \"USDC\")"), mcp.Required()),
		mcp.WithString("from_address", mcp.Description("Source token contract address (empty for native coins)")),
		mcp.WithNumber("from_decimals", mcp.Description("Source token decimals (e.g. 18 for ETH, 6 for USDC)"), mcp.Required()),
		mcp.WithString("to_chain", mcp.Description("Destination chain"), mcp.Required()),
		mcp.WithString("to_symbol", mcp.Description("Destination token symbol"), mcp.Required()),
		mcp.WithString("to_address", mcp.Description("Destination token contract address (empty for native coins)")),
		mcp.WithNumber("to_decimals", mcp.Description("Destination token decimals"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount in base units (e.g. \"1000000\" for 1 USDC)"), mcp.Required()),
		mcp.WithString("sender", mcp.Description("Sender wallet address"), mcp.Required()),
		mcp.WithString("destination", mcp.Description("Destination wallet address"), mcp.Required()),
		WithCategory("swap"),
	)
}

type swapResult struct {
	Provider       string          `json:"provider"`
	ExpectedOutput string          `json:"expected_output"`
	MinimumOutput  string          `json:"minimum_output"`
	NeedsApproval  bool            `json:"needs_approval"`
	ApprovalTx     json.RawMessage `json:"approval_tx,omitempty"`
	SwapTx         json.RawMessage `json:"swap_tx"`
	Memo           string          `json:"memo,omitempty"`
}

type swapTxJSON struct {
	To       string `json:"to"`
	Value    string `json:"value"`
	Data     string `json:"data,omitempty"`
	Memo     string `json:"memo,omitempty"`
	GasLimit uint64 `json:"gas_limit,omitempty"`
}

func handleBuildSwapTx(svc *swap.Service) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fromChain, err := req.RequireString("from_chain")
		if err != nil {
			return mcp.NewToolResultError("missing from_chain"), nil
		}
		fromSymbol, err := req.RequireString("from_symbol")
		if err != nil {
			return mcp.NewToolResultError("missing from_symbol"), nil
		}
		fromAddress := req.GetString("from_address", "")
		fromDecimals := int(req.GetInt("from_decimals", 18))

		toChain, err := req.RequireString("to_chain")
		if err != nil {
			return mcp.NewToolResultError("missing to_chain"), nil
		}
		toSymbol, err := req.RequireString("to_symbol")
		if err != nil {
			return mcp.NewToolResultError("missing to_symbol"), nil
		}
		toAddress := req.GetString("to_address", "")
		toDecimals := int(req.GetInt("to_decimals", 18))

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %q", amountStr)), nil
		}

		sender, err := req.RequireString("sender")
		if err != nil {
			return mcp.NewToolResultError("missing sender"), nil
		}
		destination, err := req.RequireString("destination")
		if err != nil {
			return mcp.NewToolResultError("missing destination"), nil
		}

		params := swap.SwapParams{
			FromChain:    fromChain,
			FromSymbol:   fromSymbol,
			FromAddress:  fromAddress,
			FromDecimals: fromDecimals,
			ToChain:      toChain,
			ToSymbol:     toSymbol,
			ToAddress:    toAddress,
			ToDecimals:   toDecimals,
			Amount:       amount,
			Sender:       sender,
			Destination:  destination,
		}

		bundle, err := svc.GetSwapTxBundle(ctx, params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("swap failed: %v", err)), nil
		}

		result := swapResult{
			Provider:       bundle.Provider,
			ExpectedOutput: bundle.ExpectedOutput.String(),
			MinimumOutput:  bundle.MinimumOutput.String(),
			NeedsApproval:  bundle.NeedsApproval,
			Memo:           bundle.Memo,
		}

		swapTx := txDataToJSON(bundle.SwapTx)
		result.SwapTx, _ = json.Marshal(swapTx)

		if bundle.NeedsApproval && bundle.ApprovalTx != nil {
			approvalTx := txDataToJSON(bundle.ApprovalTx)
			result.ApprovalTx, _ = json.Marshal(approvalTx)
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal swap result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func txDataToJSON(tx *swap.TxData) swapTxJSON {
	result := swapTxJSON{
		To:       tx.To,
		GasLimit: tx.GasLimit,
		Memo:     tx.Memo,
	}
	if tx.Value != nil {
		result.Value = tx.Value.String()
	} else {
		result.Value = "0"
	}
	if len(tx.Data) > 0 {
		result.Data = fmt.Sprintf("0x%x", tx.Data)
	}
	return result
}
