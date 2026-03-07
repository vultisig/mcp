package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

func newSetVaultInfoTool() mcp.Tool {
	return mcp.NewTool("set_vault_info",
		mcp.WithDescription("INTERNAL — called by the app to store vault key material. AI agents must NEVER call this tool — pass addresses explicitly instead."),
		mcp.WithString("ecdsa_public_key",
			mcp.Description("Hex-encoded compressed ECDSA public key (33 bytes / 66 hex chars)"),
			mcp.Required(),
		),
		mcp.WithString("eddsa_public_key",
			mcp.Description("Hex-encoded EdDSA public key (32 bytes / 64 hex chars)"),
			mcp.Required(),
		),
		mcp.WithString("chain_code",
			mcp.Description("Hex-encoded 32-byte chain code for BIP-32 derivation"),
			mcp.Required(),
		),
		WithCategory("utility"),
	)
}

func handleSetVaultInfo(store *vault.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ecdsa, err := req.RequireString("ecdsa_public_key")
		if err != nil {
			return mcp.NewToolResultError("missing ecdsa_public_key"), nil
		}
		eddsa, err := req.RequireString("eddsa_public_key")
		if err != nil {
			return mcp.NewToolResultError("missing eddsa_public_key"), nil
		}
		chainCode, err := req.RequireString("chain_code")
		if err != nil {
			return mcp.NewToolResultError("missing chain_code"), nil
		}

		sessionID := resolve.SessionIDFromCtx(ctx)
		store.Set(sessionID, vault.Info{
			ECDSAPublicKey: ecdsa,
			EdDSAPublicKey: eddsa,
			ChainCode:      chainCode,
		})

		return mcp.NewToolResultText("vault info stored for session"), nil
	}
}
