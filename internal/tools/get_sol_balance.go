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

func newGetSOLBalanceTool() mcp.Tool {
	return mcp.NewTool("get_sol_balance",
		mcp.WithDescription(
			"Query the native SOL balance of a Solana address. "+
				"If no address is provided, derives it from the vault's EdDSA public key Accepts inline vault keys (ecdsa_public_key, eddsa_public_key, chain_code) or falls back to set_vault_info session state.",
		),
		mcp.WithString("address",
			mcp.Description("Solana address (base58). Optional if vault info is set."),
		),
	)
}

func handleGetSOLBalance(store *vault.Store, solClient *solanaclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")

		addr, err := resolve.ChainAddress(explicit, resolve.ResolveVault(ctx, req, store), "Solana")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pubkey, err := solanaclient.ParsePublicKey(addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid solana address: %v", err)), nil
		}

		lamports, err := solClient.GetNativeBalance(ctx, pubkey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get SOL balance: %v", err)), nil
		}

		result := map[string]any{
			"address":          addr,
			"balance_lamports": lamports,
			"balance_sol":      solanaclient.FormatLamports(lamports),
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal sol balance result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
