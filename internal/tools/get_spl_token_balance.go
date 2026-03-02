package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/vault"
)

func newGetSPLTokenBalanceTool() mcp.Tool {
	return mcp.NewTool("get_spl_token_balance",
		mcp.WithDescription(
			"Query the SPL token balance for a Solana address. "+
				"Auto-detects token program (SPL vs Token-2022) and derives the associated token account. "+
				"If no address is provided, derives it from the vault's EdDSA public key (requires set_vault_info first).",
		),
		mcp.WithString("mint",
			mcp.Description("Token mint address (base58)."),
			mcp.Required(),
		),
		mcp.WithString("address",
			mcp.Description("Owner's Solana address (base58). Optional if vault info is set."),
		),
	)
}

func handleGetSPLTokenBalance(store *vault.Store, solClient *solanaclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mintStr, err := req.RequireString("mint")
		if err != nil {
			return mcp.NewToolResultError("missing mint parameter"), nil
		}

		explicit := req.GetString("address", "")
		addr, err := resolve.ChainAddress(explicit, resolve.SessionIDFromCtx(ctx), store, "Solana")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		ownerPubkey, err := solanaclient.ParsePublicKey(addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid owner address: %v", err)), nil
		}

		mintPubkey, err := solanaclient.ParsePublicKey(mintStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid mint address: %v", err)), nil
		}

		tokenProgram, decimals, err := solClient.GetTokenProgram(ctx, mintPubkey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to detect token program: %v", err)), nil
		}

		ata, _, err := solanaclient.FindAssociatedTokenAddress(ownerPubkey, mintPubkey, tokenProgram)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to derive ATA: %v", err)), nil
		}

		balance, err := solClient.GetTokenBalance(ctx, ata)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token balance: %v", err)), nil
		}

		result := map[string]any{
			"address":       addr,
			"mint":          mintStr,
			"ata":           ata.String(),
			"token_program": tokenProgram.String(),
			"balance":       balance,
			"decimals":      decimals,
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal spl balance result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
